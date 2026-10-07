package blend

import (
	"image"

	"github.com/odevine/impasto/internal/parallel"
	"github.com/odevine/impasto/raster"
)

// srgb8ToLinear maps an 8-bit sRGB code to linear light. It holds the values
// raster's own ingest table holds, since both come from raster.SRGBToLinear
var srgb8ToLinear [256]float32

func init() {
	for i := range srgb8ToLinear {
		srgb8ToLinear[i] = raster.SRGBToLinear(float32(i) / 255.0)
	}
}

// CompositeNRGBA blends an 8-bit straight-alpha sRGB image over the part of dst
// it covers when its top-left sits at origin, clipped to dst, with mode m at
// the given opacity in [0,1]. It touches nothing outside that rectangle.
//
// The result is exactly what converting src with raster.FromImage and passing
// the buffer to CompositeRect gives, without building that buffer. The image is
// never written to, so one decoded image can serve any number of concurrent
// calls. A pixel with no alpha leaves dst alone, and in Normal mode a fully
// opaque pixel at full opacity replaces it, so neither costs any arithmetic.
// Work is split into fixed row bands, as in Composite
func CompositeNRGBA(dst *raster.Buffer, src *image.NRGBA, origin image.Point, m Mode, opacity float32) {
	if opacity <= 0 {
		return
	}
	if opacity > 1 {
		opacity = 1
	}
	sb := src.Bounds()
	r := image.Rect(origin.X, origin.Y, origin.X+sb.Dx(), origin.Y+sb.Dy()).
		Intersect(image.Rect(0, 0, dst.Width, dst.Height))
	if r.Empty() {
		return
	}
	parallel.Rows(r.Dy(), func(lo, hi int) {
		for y := r.Min.Y + lo; y < r.Min.Y+hi; y++ {
			di := (y*dst.Width + r.Min.X) * 4
			si := src.PixOffset(sb.Min.X+r.Min.X-origin.X, sb.Min.Y+y-origin.Y)
			if m == Normal {
				normalRow(dst.Pix[di:di+r.Dx()*4], src.Pix[si:si+r.Dx()*4], opacity)
			} else {
				modeRow(dst.Pix[di:di+r.Dx()*4], src.Pix[si:si+r.Dx()*4], m, opacity)
			}
		}
	})
}

// normalRow composites one row of straight-alpha 8-bit pixels over dst with
// source-over. The arithmetic is compositeNormal's, applied to the premultiplied
// linear value ingest would have produced
func normalRow(dst []float32, src []uint8, opacity float32) {
	for i := 0; i+3 < len(src); i += 4 {
		a8 := src[i+3]
		if a8 == 0 {
			continue
		}
		if a8 == 255 && opacity == 1 {
			dst[i] = srgb8ToLinear[src[i]]
			dst[i+1] = srgb8ToLinear[src[i+1]]
			dst[i+2] = srgb8ToLinear[src[i+2]]
			dst[i+3] = 1
			continue
		}
		a := float32(a8) / 255.0
		sr := srgb8ToLinear[src[i]] * a * opacity
		sg := srgb8ToLinear[src[i+1]] * a * opacity
		sb := srgb8ToLinear[src[i+2]] * a * opacity
		sa := a * opacity
		inv := 1 - sa
		dst[i] = sr + dst[i]*inv
		dst[i+1] = sg + dst[i+1]*inv
		dst[i+2] = sb + dst[i+2]*inv
		dst[i+3] = sa + dst[i+3]*inv
	}
}

// modeRow composites one row of straight-alpha 8-bit pixels over dst with any
// mode, through the same BlendPixel the buffer path uses. A pixel with no alpha
// returns the backdrop unchanged, so it is skipped
func modeRow(dst []float32, src []uint8, m Mode, opacity float32) {
	for i := 0; i+3 < len(src); i += 4 {
		a8 := src[i+3]
		if a8 == 0 {
			continue
		}
		a := float32(a8) / 255.0
		cs := [4]float32{
			srgb8ToLinear[src[i]] * a * opacity,
			srgb8ToLinear[src[i+1]] * a * opacity,
			srgb8ToLinear[src[i+2]] * a * opacity,
			a * opacity,
		}
		out := BlendPixel([4]float32{dst[i], dst[i+1], dst[i+2], dst[i+3]}, cs, m)
		dst[i], dst[i+1], dst[i+2], dst[i+3] = out[0], out[1], out[2], out[3]
	}
}
