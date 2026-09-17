package path

import "math"

// rasterizer accumulates signed coverage using the running-sum / signed-area
// method. Each edge deposits partial areas into acc, and a left-to-right prefix
// sum per row turns those into exact analytic pixel coverage. The buffer stride
// is width+1 so an edge landing on the right border writes into a guard column
// instead of bleeding into the next row.
type rasterizer struct {
	w, h   int
	stride int
	acc    []float32
}

func newRasterizer(w, h int) *rasterizer {
	// Two guard columns absorb deposits at and just past the right border
	// (single-column edges at x==w touch column w+1) without bleeding rows
	stride := w + 2
	return &rasterizer{
		w:      w,
		h:      h,
		stride: stride,
		acc:    make([]float32, stride*h),
	}
}

// addPolyline deposits every edge of a subpath, closing it first since fill
// coverage is only defined for closed regions
func (r *rasterizer) addPolyline(pl polyline) {
	n := len(pl.pts)
	if n < 2 {
		return
	}
	for i := 1; i < n; i++ {
		r.line(pl.pts[i-1], pl.pts[i])
	}
	// Always close for filling, an open subpath fills as if its ends were joined
	if pl.pts[n-1] != pl.pts[0] {
		r.line(pl.pts[n-1], pl.pts[0])
	}
}

// line deposits one edge's signed-area contribution. Coordinates are clipped to
// the raster: y by the scanline loop, x by clamping into [0,w], which is exactly
// correct since geometry left of the viewport contributes full coverage to
// column 0.
func (r *rasterizer) line(p0, p1 Point) {
	dir := float32(1)
	if p0.Y > p1.Y {
		dir = -1
		p0, p1 = p1, p0
	}
	if p0.Y == p1.Y {
		return
	}
	dxdy := (p1.X - p0.X) / (p1.Y - p0.Y)

	y0 := p0.Y
	y1 := p1.Y
	x := p0.X
	if y0 < 0 {
		x += (0 - y0) * dxdy
		y0 = 0
	}
	if y1 > float32(r.h) {
		y1 = float32(r.h)
	}
	if y0 >= y1 {
		return
	}

	yStart := int(math.Floor(float64(y0)))
	yEnd := int(math.Ceil(float64(y1)))
	xf := clampf(x, 0, float32(r.w))
	for y := yStart; y < yEnd; y++ {
		if y >= r.h {
			break
		}
		rowTop := fmax(float32(y), y0)
		rowBot := fmin(float32(y+1), y1)
		dy := rowBot - rowTop
		if dy <= 0 {
			continue
		}
		// x advances only across this band's own vertical span
		xNextRaw := x + dxdy*dy
		xNext := clampf(xNextRaw, 0, float32(r.w))
		d := dy * dir
		r.depositSpan(y, xf, xNext, d)
		x = xNextRaw
		xf = xNext
	}
}

// depositSpan spreads coverage d across the columns the edge crosses within one
// scanline, from xa to xb (already clamped into [0,w])
func (r *rasterizer) depositSpan(y int, xa, xb, d float32) {
	x0, x1 := xa, xb
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	base := y * r.stride
	x0floor := float32(math.Floor(float64(x0)))
	x0i := int(x0floor)
	x1ceil := float32(math.Ceil(float64(x1)))
	x1i := int(x1ceil)

	if x1i <= x0i+1 {
		// The edge stays within a single pixel column
		xmf := 0.5*(xa+xb) - x0floor
		r.acc[base+x0i] += d - d*xmf
		r.acc[base+x0i+1] += d * xmf
		return
	}

	s := 1.0 / (x1 - x0)
	x0f := x0 - x0floor
	oneMinusX0f := 1 - x0f
	a0 := 0.5 * s * oneMinusX0f * oneMinusX0f
	x1f := x1 - x1ceil + 1
	am := 0.5 * s * x1f * x1f

	r.acc[base+x0i] += d * a0
	if x1i == x0i+2 {
		r.acc[base+x0i+1] += d * (1 - a0 - am)
	} else {
		a1 := s * (1.5 - x0f)
		r.acc[base+x0i+1] += d * (a1 - a0)
		for xi := x0i + 2; xi < x1i-1; xi++ {
			r.acc[base+xi] += d * s
		}
		a2 := a1 + float32(x1i-x0i-3)*s
		r.acc[base+x1i-1] += d * (1 - a2 - am)
	}
	r.acc[base+x1i] += d * am
}

// coverage runs the per-row prefix sum and maps the signed area through the fill
// rule into a coverage buffer of length w*h
func (r *rasterizer) coverage(rule FillRule) []float32 {
	out := make([]float32, r.w*r.h)
	for y := 0; y < r.h; y++ {
		base := y * r.stride
		obase := y * r.w
		var sum float32
		for x := 0; x < r.w; x++ {
			sum += r.acc[base+x]
			out[obase+x] = applyRule(sum, rule)
		}
	}
	return out
}

// applyRule converts a signed winding-weighted area into [0,1] coverage
func applyRule(sum float32, rule FillRule) float32 {
	if rule == EvenOdd {
		v := sum - 2*float32(math.Floor(float64(sum)*0.5))
		if v > 1 {
			v = 2 - v
		}
		if v < 0 {
			v = 0
		}
		return v
	}
	if sum < 0 {
		sum = -sum
	}
	if sum > 1 {
		return 1
	}
	return sum
}

func clampf(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Coverage rasterizes the path into a standalone coverage buffer of the given
// size, using the fill rule and flattening tolerance. This is what vector masks
// and effects consume
func (p *Path) Coverage(w, h int, rule FillRule, tol float32) []float32 {
	r := newRasterizer(w, h)
	for _, pl := range p.flatten(tol) {
		r.addPolyline(pl)
	}
	return r.coverage(rule)
}
