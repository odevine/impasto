package blend

import (
	"image"

	"github.com/odevine/impasto/internal/parallel"
	"github.com/odevine/impasto/raster"
)

// BlendPixel composites a single premultiplied linear source pixel over a
// premultiplied linear backdrop using mode m, and returns the premultiplied
// result. The source is taken as-is, apply opacity by scaling all four of its
// components before calling this.
//
// The math follows the general compositing formula shared by the W3C and ISO
// specs. The blend function B only changes the color term, the alpha term is
// plain source-over:
//
//	Co = as(1-ab)Cs + as*ab*B(Cb,Cs) + (1-as)ab*Cb
//	ao = as + (1-as)ab
//
// B needs straight color, so the premultiplied inputs are divided out first and
// the result comes back premultiplied.
// See https://www.w3.org/TR/compositing-1/#generalformula
func BlendPixel(cb, cs [4]float32, m Mode) [4]float32 {
	ab := cb[3]
	as := cs[3]
	if as == 0 {
		return cb
	}

	// Recover straight color for the blend function
	sb := straight(cb, ab)
	ss := straight(cs, as)

	var b rgb
	if m.isNonSeparable() {
		b = nonSeparable(m, rgb{sb[0], sb[1], sb[2]}, rgb{ss[0], ss[1], ss[2]})
	} else {
		b[0] = separable(m, sb[0], ss[0])
		b[1] = separable(m, sb[1], ss[1])
		b[2] = separable(m, sb[2], ss[2])
	}

	ao := as + (1-as)*ab
	kSrc := as * (1 - ab)
	kBlend := as * ab
	kBack := (1 - as) * ab

	var out [4]float32
	out[0] = kSrc*ss[0] + kBlend*b[0] + kBack*sb[0]
	out[1] = kSrc*ss[1] + kBlend*b[1] + kBack*sb[1]
	out[2] = kSrc*ss[2] + kBlend*b[2] + kBack*sb[2]
	out[3] = ao
	return out
}

// straight recovers non-premultiplied color from a premultiplied pixel
func straight(p [4]float32, a float32) [4]float32 {
	if a <= 0 {
		return [4]float32{0, 0, 0, 0}
	}
	inv := 1.0 / a
	return [4]float32{p[0] * inv, p[1] * inv, p[2] * inv, a}
}

// Composite blends src over dst in place, across the whole buffer, using mode m
// at the given opacity in [0,1]. Both buffers must be the same size. Work is
// split into fixed row bands so the output is identical regardless of how the
// goroutines are scheduled.
func Composite(dst, src *raster.Buffer, m Mode, opacity float32) {
	if !dst.SameSize(src) {
		panic("blend: Composite requires equal-size buffers")
	}
	CompositeRect(dst, src, image.Point{}, m, opacity)
}

// CompositeRect blends src over the part of dst that src covers when src's
// top-left sits at origin, clipped to dst, and touches nothing else. The origin
// may be negative or leave src partly or fully outside dst. Mode, opacity and
// row banding follow Composite, so a zero origin with equal-size buffers gives
// the same result
func CompositeRect(dst, src *raster.Buffer, origin image.Point, m Mode, opacity float32) {
	if opacity <= 0 {
		return
	}
	if opacity > 1 {
		opacity = 1
	}
	r := image.Rect(origin.X, origin.Y, origin.X+src.Width, origin.Y+src.Height).
		Intersect(image.Rect(0, 0, dst.Width, dst.Height))
	if r.Empty() {
		return
	}
	if m == Normal {
		compositeNormal(dst, src, origin, r, opacity)
		return
	}
	parallel.Rows(r.Dy(), func(lo, hi int) {
		for y := r.Min.Y + lo; y < r.Min.Y+hi; y++ {
			di := (y*dst.Width + r.Min.X) * 4
			si := ((y-origin.Y)*src.Width + r.Min.X - origin.X) * 4
			for x := r.Min.X; x < r.Max.X; x++ {
				cs := [4]float32{
					src.Pix[si] * opacity,
					src.Pix[si+1] * opacity,
					src.Pix[si+2] * opacity,
					src.Pix[si+3] * opacity,
				}
				cb := [4]float32{dst.Pix[di], dst.Pix[di+1], dst.Pix[di+2], dst.Pix[di+3]}
				out := BlendPixel(cb, cs, m)
				dst.Pix[di] = out[0]
				dst.Pix[di+1] = out[1]
				dst.Pix[di+2] = out[2]
				dst.Pix[di+3] = out[3]
				di += 4
				si += 4
			}
		}
	})
}

// compositeNormal is the source-over fast path over the rect r of dst. Normal
// blending needs no un-premultiply, so it skips straight-color recovery
// entirely, which is the common case since most layers use Normal
func compositeNormal(dst, src *raster.Buffer, origin image.Point, r image.Rectangle, opacity float32) {
	parallel.Rows(r.Dy(), func(lo, hi int) {
		for y := r.Min.Y + lo; y < r.Min.Y+hi; y++ {
			di := (y*dst.Width + r.Min.X) * 4
			si := ((y-origin.Y)*src.Width + r.Min.X - origin.X) * 4
			for x := r.Min.X; x < r.Max.X; x++ {
				sr := src.Pix[si] * opacity
				sg := src.Pix[si+1] * opacity
				sb := src.Pix[si+2] * opacity
				sa := src.Pix[si+3] * opacity
				inv := 1 - sa
				dst.Pix[di] = sr + dst.Pix[di]*inv
				dst.Pix[di+1] = sg + dst.Pix[di+1]*inv
				dst.Pix[di+2] = sb + dst.Pix[di+2]*inv
				dst.Pix[di+3] = sa + dst.Pix[di+3]*inv
				di += 4
				si += 4
			}
		}
	})
}
