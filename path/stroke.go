package path

import "math"

// Cap selects how open subpath ends are terminated
type Cap int

const (
	CapButt Cap = iota
	CapRound
	CapSquare
)

// Join selects how consecutive segments are connected at a vertex
type Join int

const (
	JoinMiter Join = iota
	JoinRound
	JoinBevel
)

// StrokeStyle describes a stroke. The zero value is a hairline-width butt-capped
// miter-joined stroke, so callers only set what they care about
type StrokeStyle struct {
	Width      float32
	Cap        Cap
	Join       Join
	MiterLimit float32
	Dash       []float32
	DashOffset float32
}

// DefaultMiterLimit matches the SVG/PostScript default
const DefaultMiterLimit = 4

// Stroke converts a path into a fillable outline. Fill the result with NonZero
// to paint the stroke. The outline is built from many convex pieces (a quad per
// segment, a shape per join, a shape per cap) all wound the same way, so nonzero
// winding unions them cleanly rather than requiring fragile edge stitching.
func Stroke(p *Path, style StrokeStyle) *Path {
	hw := style.Width * 0.5
	if hw <= 0 {
		return New()
	}
	miter := style.MiterLimit
	if miter <= 0 {
		miter = DefaultMiterLimit
	}
	out := New()
	for _, pl := range p.flatten(DefaultTolerance) {
		runs := dashRuns(pl, style)
		for _, run := range runs {
			strokeRun(out, run.pts, run.closed, hw, style, miter)
		}
	}
	return out
}

// run is a contiguous stretch of the stroke to render, either a full closed loop
// or an open dash segment
type run struct {
	pts    []Point
	closed bool
}

// dashRuns splits a polyline into dashed runs, or returns it whole when no dash
// pattern is set. A closed loop becomes open runs once dashed
func dashRuns(pl polyline, style StrokeStyle) []run {
	if len(style.Dash) == 0 {
		return []run{{pts: pl.pts, closed: pl.closed}}
	}
	pts := pl.pts
	if pl.closed && (pts[0] != pts[len(pts)-1]) {
		pts = append(append([]Point{}, pts...), pts[0])
	}
	pattern := style.Dash
	total := float32(0)
	for _, d := range pattern {
		total += d
	}
	if total <= 0 {
		return []run{{pts: pl.pts, closed: pl.closed}}
	}

	// Locate the starting dash index and remaining length from the offset
	di := 0
	rem := pattern[0]
	off := style.DashOffset
	for off > 0 {
		if off < rem {
			rem -= off
			off = 0
		} else {
			off -= rem
			di = (di + 1) % len(pattern)
			rem = pattern[di]
		}
	}
	on := di%2 == 0

	var runs []run
	var cur []Point
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		segLen := hypot(b.X-a.X, b.Y-a.Y)
		if segLen == 0 {
			continue
		}
		dir := Point{(b.X - a.X) / segLen, (b.Y - a.Y) / segLen}
		pos := float32(0)
		if on && len(cur) == 0 {
			cur = []Point{a}
		}
		for pos < segLen {
			step := fmin(rem, segLen-pos)
			pos += step
			rem -= step
			p := Point{a.X + dir.X*pos, a.Y + dir.Y*pos}
			if on {
				cur = append(cur, p)
			}
			if rem <= 1e-6 {
				if on {
					runs = append(runs, run{pts: cur, closed: false})
					cur = nil
				}
				di = (di + 1) % len(pattern)
				rem = pattern[di]
				on = !on
				if on {
					cur = []Point{p}
				}
			}
		}
	}
	if len(cur) >= 2 {
		runs = append(runs, run{pts: cur, closed: false})
	}
	return runs
}

// strokeRun emits the outline pieces for one run of points
func strokeRun(out *Path, pts []Point, closed bool, hw float32, style StrokeStyle, miter float32) {
	// Drop consecutive duplicate points so directions are well defined
	pts = dedup(pts)
	n := len(pts)
	if n < 2 {
		if n == 1 && style.Cap == CapRound {
			emitDisc(out, pts[0], hw)
		}
		return
	}

	segCount := n - 1
	for i := 0; i < segCount; i++ {
		emitSegmentQuad(out, pts[i], pts[i+1], hw)
	}

	// Interior joins
	for i := 1; i < n-1; i++ {
		emitJoin(out, pts[i-1], pts[i], pts[i+1], hw, style.Join, miter)
	}
	if closed {
		// Join across the closing vertex where last meets first
		emitJoin(out, pts[n-2], pts[0], pts[1], hw, style.Join, miter)
		// The closing segment
		if pts[n-1] != pts[0] {
			emitSegmentQuad(out, pts[n-1], pts[0], hw)
			emitJoin(out, pts[n-2], pts[n-1], pts[0], hw, style.Join, miter)
			emitJoin(out, pts[n-1], pts[0], pts[1], hw, style.Join, miter)
		}
		return
	}

	// Open ends get caps
	emitCap(out, pts[1], pts[0], hw, style.Cap)
	emitCap(out, pts[n-2], pts[n-1], hw, style.Cap)
}

// emitSegmentQuad stamps the rectangle covering one segment offset by half-width
func emitSegmentQuad(out *Path, a, b Point, hw float32) {
	nx, ny, ok := segNormal(a, b)
	if !ok {
		return
	}
	nx, ny = nx*hw, ny*hw
	emitConvex(out, []Point{
		{a.X + nx, a.Y + ny},
		{b.X + nx, b.Y + ny},
		{b.X - nx, b.Y - ny},
		{a.X - nx, a.Y - ny},
	})
}

// emitJoin fills the wedge left open on the outer side of a corner
func emitJoin(out *Path, prev, v, next Point, hw float32, join Join, miter float32) {
	n0x, n0y, ok0 := segNormal(prev, v)
	n1x, n1y, ok1 := segNormal(v, next)
	if !ok0 || !ok1 {
		return
	}
	if join == JoinRound {
		emitDisc(out, v, hw)
		return
	}
	// Outer corners on both sides, one side is the real gap and the other is
	// already covered by the quads, both are harmless to fill under nonzero
	p0a := Point{v.X + n0x*hw, v.Y + n0y*hw}
	p1a := Point{v.X + n1x*hw, v.Y + n1y*hw}
	p0b := Point{v.X - n0x*hw, v.Y - n0y*hw}
	p1b := Point{v.X - n1x*hw, v.Y - n1y*hw}
	emitConvex(out, []Point{v, p0a, p1a})
	emitConvex(out, []Point{v, p0b, p1b})

	if join == JoinMiter {
		emitMiterTip(out, v, p0a, p1a, n0x, n0y, n1x, n1y, hw, miter)
		emitMiterTip(out, v, p0b, p1b, -n0x, -n0y, -n1x, -n1y, hw, miter)
	}
}

// emitMiterTip extends two offset edges to their intersection, when the sharp
// tip stays within the miter limit
func emitMiterTip(out *Path, v, pa, pb Point, n0x, n0y, n1x, n1y, hw, miter float32) {
	// Edge directions are perpendicular to the normals
	d0 := Point{n0y, -n0x}
	d1 := Point{n1y, -n1x}
	m, ok := intersect(pa, d0, pb, d1)
	if !ok {
		return
	}
	dist := hypot(m.X-v.X, m.Y-v.Y)
	if dist > miter*hw {
		return
	}
	emitConvex(out, []Point{pa, m, pb})
}

// emitCap terminates an open end. from is the neighbor point, end is the tip
func emitCap(out *Path, from, end Point, hw float32, cap Cap) {
	switch cap {
	case CapRound:
		emitDisc(out, end, hw)
	case CapSquare:
		dx := end.X - from.X
		dy := end.Y - from.Y
		l := hypot(dx, dy)
		if l == 0 {
			return
		}
		ex, ey := dx/l*hw, dy/l*hw
		nx, ny := -ey, ex
		emitConvex(out, []Point{
			{end.X + nx, end.Y + ny},
			{end.X + nx + ex, end.Y + ny + ey},
			{end.X - nx + ex, end.Y - ny + ey},
			{end.X - nx, end.Y - ny},
		})
	}
}

// emitDisc approximates a filled circle with a polygon dense enough that its
// facets stay under about a tenth of a pixel for typical radii
func emitDisc(out *Path, c Point, r float32) {
	if r <= 0 {
		return
	}
	// Chord error for a regular n-gon is roughly r*(pi/n)^2/2, solve for 0.1px
	steps := int(math.Ceil(math.Pi * math.Sqrt(5*float64(r))))
	if steps < 8 {
		steps = 8
	}
	if steps > 256 {
		steps = 256
	}
	pts := make([]Point, steps)
	for i := 0; i < steps; i++ {
		a := 2 * math.Pi * float64(i) / float64(steps)
		pts[i] = Point{c.X + r*float32(math.Cos(a)), c.Y + r*float32(math.Sin(a))}
	}
	emitConvex(out, pts)
}

// emitConvex appends a closed subpath, forcing counter-clockwise winding so
// every emitted piece adds rather than cancels under nonzero fill
func emitConvex(out *Path, pts []Point) {
	if len(pts) < 3 {
		return
	}
	if signedArea(pts) < 0 {
		for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
			pts[i], pts[j] = pts[j], pts[i]
		}
	}
	out.MoveTo(pts[0].X, pts[0].Y)
	for _, p := range pts[1:] {
		out.LineTo(p.X, p.Y)
	}
	out.Close()
}

// segNormal returns the unit left normal of segment a to b
func segNormal(a, b Point) (nx, ny float32, ok bool) {
	dx := b.X - a.X
	dy := b.Y - a.Y
	l := hypot(dx, dy)
	if l == 0 {
		return 0, 0, false
	}
	return -dy / l, dx / l, true
}

// intersect returns the crossing of line (p0,dir0) with line (p1,dir1)
func intersect(p0, dir0, p1, dir1 Point) (Point, bool) {
	denom := dir0.X*dir1.Y - dir0.Y*dir1.X
	if fabs(denom) < 1e-6 {
		return Point{}, false
	}
	t := ((p1.X-p0.X)*dir1.Y - (p1.Y-p0.Y)*dir1.X) / denom
	return Point{p0.X + dir0.X*t, p0.Y + dir0.Y*t}, true
}

// signedArea is twice the signed area of a polygon, positive for CCW
func signedArea(pts []Point) float32 {
	var a float32
	for i := 0; i < len(pts); i++ {
		j := (i + 1) % len(pts)
		a += pts[i].X*pts[j].Y - pts[j].X*pts[i].Y
	}
	return a
}

// dedup removes consecutive identical points
func dedup(pts []Point) []Point {
	if len(pts) == 0 {
		return pts
	}
	out := pts[:1]
	for _, p := range pts[1:] {
		if p != out[len(out)-1] {
			out = append(out, p)
		}
	}
	return out
}
