// Package path builds bezier paths and rasterizes them with anti-aliased
// scanline coverage. It supports both fill (nonzero and even-odd winding) and
// stroking, the piece missing from the standard library's vector rasterizer.
// Coverage is produced at float32 precision so gradients and effects downstream
// do not inherit 8-bit banding.
package path

import "math"

// Point is a 2D coordinate in pixel space
type Point struct {
	X, Y float32
}

// FillRule selects how overlapping subpaths and self-intersections are resolved
type FillRule int

const (
	// NonZero fills a region wherever the winding number is not zero
	NonZero FillRule = iota
	// EvenOdd fills a region wherever the crossing count is odd
	EvenOdd
)

// verb tags each command in a path's stream
type verb uint8

const (
	verbMove verb = iota
	verbLine
	verbQuad
	verbCubic
	verbClose
)

// Path is a sequence of subpaths built from move, line, quadratic, and cubic
// segments. Curves are stored exactly and only flattened to line segments when
// the path is rasterized, so the same path renders cleanly at any scale.
type Path struct {
	verbs  []verb
	pts    []Point
	cur    Point
	start  Point
	hasCur bool
}

// New returns an empty path
func New() *Path { return &Path{} }

// MoveTo starts a new subpath at (x,y)
func (p *Path) MoveTo(x, y float32) *Path {
	p.verbs = append(p.verbs, verbMove)
	pt := Point{x, y}
	p.pts = append(p.pts, pt)
	p.cur = pt
	p.start = pt
	p.hasCur = true
	return p
}

// LineTo adds a straight segment from the current point to (x,y)
func (p *Path) LineTo(x, y float32) *Path {
	if !p.hasCur {
		return p.MoveTo(x, y)
	}
	p.verbs = append(p.verbs, verbLine)
	pt := Point{x, y}
	p.pts = append(p.pts, pt)
	p.cur = pt
	return p
}

// QuadTo adds a quadratic bezier through control point (cx,cy) to (x,y)
func (p *Path) QuadTo(cx, cy, x, y float32) *Path {
	if !p.hasCur {
		p.MoveTo(cx, cy)
	}
	p.verbs = append(p.verbs, verbQuad)
	p.pts = append(p.pts, Point{cx, cy}, Point{x, y})
	p.cur = Point{x, y}
	return p
}

// CubicTo adds a cubic bezier through control points (c1x,c1y) and (c2x,c2y) to
// (x,y)
func (p *Path) CubicTo(c1x, c1y, c2x, c2y, x, y float32) *Path {
	if !p.hasCur {
		p.MoveTo(c1x, c1y)
	}
	p.verbs = append(p.verbs, verbCubic)
	p.pts = append(p.pts, Point{c1x, c1y}, Point{c2x, c2y}, Point{x, y})
	p.cur = Point{x, y}
	return p
}

// Close connects the current point back to the subpath's start
func (p *Path) Close() *Path {
	if !p.hasCur {
		return p
	}
	p.verbs = append(p.verbs, verbClose)
	p.cur = p.start
	return p
}

// Empty reports whether the path holds no segments
func (p *Path) Empty() bool { return len(p.verbs) == 0 }

// Rect appends an axis-aligned rectangle as a closed subpath
func (p *Path) Rect(x0, y0, x1, y1 float32) *Path {
	return p.MoveTo(x0, y0).LineTo(x1, y0).LineTo(x1, y1).LineTo(x0, y1).Close()
}

// Ellipse appends an ellipse centered at (cx,cy) as a closed subpath, using the
// standard four-cubic approximation
func (p *Path) Ellipse(cx, cy, rx, ry float32) *Path {
	const k = 0.5522847498307936 // 4/3 * (sqrt(2)-1), the circle-to-cubic constant
	ox, oy := rx*k, ry*k
	p.MoveTo(cx+rx, cy)
	p.CubicTo(cx+rx, cy+oy, cx+ox, cy+ry, cx, cy+ry)
	p.CubicTo(cx-ox, cy+ry, cx-rx, cy+oy, cx-rx, cy)
	p.CubicTo(cx-rx, cy-oy, cx-ox, cy-ry, cx, cy-ry)
	p.CubicTo(cx+ox, cy-ry, cx+rx, cy-oy, cx+rx, cy)
	return p.Close()
}

// Bounds returns the tight axis-aligned bounding box over the path's on-curve
// and control points. Control points can extend past the true curve, so this is
// a conservative outer bound
func (p *Path) Bounds() (minX, minY, maxX, maxY float32) {
	if len(p.pts) == 0 {
		return 0, 0, 0, 0
	}
	minX, minY = p.pts[0].X, p.pts[0].Y
	maxX, maxY = minX, minY
	for _, pt := range p.pts {
		minX = fmin(minX, pt.X)
		minY = fmin(minY, pt.Y)
		maxX = fmax(maxX, pt.X)
		maxY = fmax(maxY, pt.Y)
	}
	return
}

func fmin(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func fmax(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func fabs(a float32) float32 {
	if a < 0 {
		return -a
	}
	return a
}

func hypot(x, y float32) float32 {
	return float32(math.Hypot(float64(x), float64(y)))
}
