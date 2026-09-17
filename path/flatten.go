package path

// DefaultTolerance is the maximum distance in pixels between a flattened line
// segment and the true curve. Smaller values produce smoother curves at the
// cost of more segments
const DefaultTolerance = 0.1

// polyline is a flattened subpath: a run of points and whether it is closed
type polyline struct {
	pts    []Point
	closed bool
}

// flatten converts the path's curves into polylines within the given tolerance.
// Each move starts a new polyline, close marks the current one closed
func (p *Path) flatten(tol float32) []polyline {
	if tol <= 0 {
		tol = DefaultTolerance
	}
	var out []polyline
	var cur []Point
	var start Point
	pi := 0

	flush := func(closed bool) {
		if len(cur) > 0 {
			out = append(out, polyline{pts: cur, closed: closed})
			cur = nil
		}
	}

	for _, v := range p.verbs {
		switch v {
		case verbMove:
			flush(false)
			start = p.pts[pi]
			cur = []Point{start}
			pi++
		case verbLine:
			cur = append(cur, p.pts[pi])
			pi++
		case verbQuad:
			c := p.pts[pi]
			end := p.pts[pi+1]
			from := cur[len(cur)-1]
			cur = flattenQuad(cur, from, c, end, tol)
			pi += 2
		case verbCubic:
			c1 := p.pts[pi]
			c2 := p.pts[pi+1]
			end := p.pts[pi+2]
			from := cur[len(cur)-1]
			cur = flattenCubic(cur, from, c1, c2, end, tol)
			pi += 3
		case verbClose:
			flush(true)
			cur = []Point{start}
		}
	}
	flush(false)
	// Drop degenerate single-point trailing polylines left by a close
	filtered := out[:0]
	for _, pl := range out {
		if len(pl.pts) >= 2 {
			filtered = append(filtered, pl)
		}
	}
	return filtered
}

// flattenQuad recursively subdivides a quadratic bezier until it is flat enough,
// appending on-curve points (not including the start) to dst
func flattenQuad(dst []Point, p0, p1, p2 Point, tol float32) []Point {
	// Distance of the control point from the p0-p2 chord estimates flatness
	if quadFlat(p0, p1, p2) <= tol {
		return append(dst, p2)
	}
	p01 := mid(p0, p1)
	p12 := mid(p1, p2)
	p012 := mid(p01, p12)
	dst = flattenQuad(dst, p0, p01, p012, tol)
	return flattenQuad(dst, p012, p12, p2, tol)
}

// flattenCubic recursively subdivides a cubic bezier until flat enough
func flattenCubic(dst []Point, p0, p1, p2, p3 Point, tol float32) []Point {
	if cubicFlat(p0, p1, p2, p3) <= tol {
		return append(dst, p3)
	}
	p01 := mid(p0, p1)
	p12 := mid(p1, p2)
	p23 := mid(p2, p3)
	p012 := mid(p01, p12)
	p123 := mid(p12, p23)
	p0123 := mid(p012, p123)
	dst = flattenCubic(dst, p0, p01, p012, p0123, tol)
	return flattenCubic(dst, p0123, p123, p23, p3, tol)
}

func mid(a, b Point) Point {
	return Point{(a.X + b.X) * 0.5, (a.Y + b.Y) * 0.5}
}

// quadFlat returns the perpendicular distance of the control point from the
// chord, the standard quadratic flatness measure
func quadFlat(p0, p1, p2 Point) float32 {
	return distToSegment(p1, p0, p2)
}

// cubicFlat returns the larger of the two control points' distances from the
// chord
func cubicFlat(p0, p1, p2, p3 Point) float32 {
	return fmax(distToSegment(p1, p0, p3), distToSegment(p2, p0, p3))
}

// distToSegment returns the distance from point c to the line through a and b
func distToSegment(c, a, b Point) float32 {
	dx := b.X - a.X
	dy := b.Y - a.Y
	l := hypot(dx, dy)
	if l == 0 {
		return hypot(c.X-a.X, c.Y-a.Y)
	}
	// Perpendicular distance via the cross product magnitude
	return fabs((c.X-a.X)*dy-(c.Y-a.Y)*dx) / l
}
