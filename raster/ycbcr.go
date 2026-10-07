package raster

import (
	"image"
	"image/color"
	"math"

	"github.com/odevine/impasto/internal/parallel"
)

// ToYCbCr encodes the buffer as 8-bit sRGB in the full-range BT.601 color space
// JPEG stores, with chroma subsampled 4:2:0 by averaging each 2x2 block of
// pixels. The planes are what image/jpeg's encoder takes without converting
// again, so a buffer headed for a JPEG skips the intermediate RGB image
// that ToImage and the encoder would otherwise build.
//
// Alpha is dropped by flattening the buffer over a background, which is the
// optional argument and black when it is missing. A background with alpha of its
// own is composited under the buffer and the result is then taken over black, so
// an opaque one is the usual choice. The conversion is not dithered, unlike
// ToImage, since the quantization a JPEG applies would hide it at the cost of a
// larger file. A pixel past the edge of an odd-sized buffer is read as its inside
// neighbor when averaging chroma.
//
// Rows are converted in parallel bands aligned to the 2x2 blocks, so the result
// does not depend on how many goroutines share the work
func (b *Buffer) ToYCbCr(background ...color.Color) *image.YCbCr {
	w, h := b.Width, b.Height
	dst := image.NewYCbCr(image.Rect(0, 0, w, h), image.YCbCrSubsampleRatio420)
	bg, flatten := premultipliedLinear(background)
	parallel.RowsAligned(h, 2, func(lo, hi int) {
		for y := lo; y < hi; y += 2 {
			y1 := min(y+1, h-1)
			for x := 0; x < w; x += 2 {
				x1 := min(x+1, w-1)
				var sr, sg, sb int32
				for _, at := range [4][2]int{{x, y}, {x1, y}, {x, y1}, {x1, y1}} {
					i := (at[1]*w + at[0]) * 4
					r, g, bl := b.Pix[i], b.Pix[i+1], b.Pix[i+2]
					if flatten {
						// The part of the background the pixel leaves showing
						show := max(1-b.Pix[i+3], 0)
						r, g, bl = r+show*bg[0], g+show*bg[1], bl+show*bg[2]
					}
					cr, cg, cb := code8(r), code8(g), code8(bl)
					dst.Y[at[1]*dst.YStride+at[0]] = uint8((19595*cr + 38470*cg + 7471*cb + 1<<15) >> 16)
					sr, sg, sb = sr+cr, sg+cg, sb+cb
				}
				c := y/2*dst.CStride + x/2
				dst.Cb[c] = chromaCode(-11059*sr - 21709*sg + 32768*sb)
				dst.Cr[c] = chromaCode(32768*sr - 27439*sg - 5329*sb)
			}
		}
	})
	return dst
}

// premultipliedLinear is the background as premultiplied linear light, and
// whether there is any to flatten over. No argument, and a transparent or black
// one, flatten over nothing
func premultipliedLinear(background []color.Color) (bg [3]float32, flatten bool) {
	if len(background) == 0 || background[0] == nil {
		return bg, false
	}
	r, g, b, a := background[0].RGBA()
	if a == 0 {
		return bg, false
	}
	// RGBA returns premultiplied 16-bit values, and the transfer function acts
	// on the straight color
	for i, v := range [3]uint32{r, g, b} {
		straight := float32(v) / float32(a)
		bg[i] = SRGBToLinear(min(straight, 1)) * float32(a) / 65535
		if bg[i] != 0 {
			flatten = true
		}
	}
	return bg, flatten
}

// srgbCodes maps a linear value quantized to 16 bits to its 8-bit sRGB code, so
// the conversion is a lookup and not a power function. The 64 KiB table stays
// resident in cache. A code can differ by one from the exact encoding where the
// exact value sits within a twentieth of a code of a rounding boundary, which the
// JPEG quantization that follows hides
var srgbCodes [65536]uint8

func init() {
	for i := range srgbCodes {
		srgbCodes[i] = uint8(math.Round(linearToSRGB64(float64(i)/65535) * 255))
	}
}

// linearToSRGB64 is LinearToSRGB in double precision, so a table entry is the
// correctly rounded code
func linearToSRGB64(c float64) float64 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*math.Pow(c, 1.0/2.4) - 0.055
}

// code8 is the 8-bit sRGB code of a linear value, clamped to [0,1]
func code8(v float32) int32 {
	if !(v > 0) {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return int32(srgbCodes[int(v*65535+0.5)])
}

// chromaCode turns a fixed-point chroma sum over four pixels, scaled by 2^16, into
// a code centered on 128
func chromaCode(sum int32) uint8 {
	return uint8(min((128<<18+sum+1<<17)>>18, 255))
}
