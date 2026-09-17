package path

import (
	"github.com/odevine/impasto/internal/parallel"
	"github.com/odevine/impasto/raster"
)

// Fill rasterizes the path and composites the paint over dst using source-over,
// scaled by per-pixel coverage. The path is in dst's coordinate space with the
// top-left at (0,0). Fill uses the default flattening tolerance.
func Fill(dst *raster.Buffer, p *Path, rule FillRule, paint Paint) {
	FillTol(dst, p, rule, paint, DefaultTolerance)
}

// FillTol is Fill with an explicit flattening tolerance for callers that need
// finer curves at large scale
func FillTol(dst *raster.Buffer, p *Path, rule FillRule, paint Paint, tol float32) {
	cov := p.Coverage(dst.Width, dst.Height, rule, tol)
	w := dst.Width
	parallel.Rows(dst.Height, func(lo, hi int) {
		for y := lo; y < hi; y++ {
			ci := y * w
			pi := y * w * 4
			for x := 0; x < w; x++ {
				c := cov[ci]
				if c > 0 {
					src := paint.ColorAt(x, y)
					sr := src[0] * c
					sg := src[1] * c
					sb := src[2] * c
					sa := src[3] * c
					inv := 1 - sa
					dst.Pix[pi] = sr + dst.Pix[pi]*inv
					dst.Pix[pi+1] = sg + dst.Pix[pi+1]*inv
					dst.Pix[pi+2] = sb + dst.Pix[pi+2]*inv
					dst.Pix[pi+3] = sa + dst.Pix[pi+3]*inv
				}
				ci++
				pi += 4
			}
		}
	})
}

// FillColor is a convenience wrapper filling with a single sRGB color
func FillColor(dst *raster.Buffer, p *Path, rule FillRule, paint FlatColor) {
	Fill(dst, p, rule, paint)
}

// StrokePath strokes p with the given style and paints the result onto dst
func StrokePath(dst *raster.Buffer, p *Path, style StrokeStyle, paint Paint) {
	Fill(dst, Stroke(p, style), NonZero, paint)
}
