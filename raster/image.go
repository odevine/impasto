package raster

import (
	"fmt"
	"image"
	"image/color"
	"os"

	// Register the standard decoders so FromFile handles the common formats
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// FromImage decodes any image.Image into a Buffer: sRGB to linear light and
// straight to premultiplied alpha. Fast paths cover the concrete types the
// standard decoders produce most often, everything else goes through the
// generic straight-alpha model so the result is always correct.
func FromImage(img image.Image) (*Buffer, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	buf, err := NewBuffer(w, h)
	if err != nil {
		return nil, err
	}
	switch src := img.(type) {
	case *image.NRGBA:
		buf.fromNRGBA(src)
	case *image.NRGBA64:
		buf.fromNRGBA64(src)
	case *image.Gray:
		buf.fromGray(src)
	case *image.Gray16:
		buf.fromGray16(src)
	default:
		buf.fromGeneric(img)
	}
	return buf, nil
}

// FromFile decodes an image file into a Buffer. PNG, JPEG, and GIF are
// supported out of the box
func FromFile(path string) (*Buffer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("raster: decode %s: %w", path, err)
	}
	return FromImage(img)
}

// fromNRGBA ingests straight-alpha 8-bit sRGB, the usual PNG decode output
func (buf *Buffer) fromNRGBA(src *image.NRGBA) {
	b := src.Bounds()
	di := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		si := src.PixOffset(b.Min.X, y)
		for x := b.Min.X; x < b.Max.X; x++ {
			r := srgb8ToLinearLUT[src.Pix[si]]
			g := srgb8ToLinearLUT[src.Pix[si+1]]
			bl := srgb8ToLinearLUT[src.Pix[si+2]]
			a := float32(src.Pix[si+3]) / 255.0
			buf.Pix[di] = r * a
			buf.Pix[di+1] = g * a
			buf.Pix[di+2] = bl * a
			buf.Pix[di+3] = a
			si += 4
			di += 4
		}
	}
}

// fromNRGBA64 ingests straight-alpha 16-bit sRGB
func (buf *Buffer) fromNRGBA64(src *image.NRGBA64) {
	b := src.Bounds()
	di := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		si := src.PixOffset(b.Min.X, y)
		for x := b.Min.X; x < b.Max.X; x++ {
			r := srgb16ToLinearLUT[uint16(src.Pix[si])<<8|uint16(src.Pix[si+1])]
			g := srgb16ToLinearLUT[uint16(src.Pix[si+2])<<8|uint16(src.Pix[si+3])]
			bl := srgb16ToLinearLUT[uint16(src.Pix[si+4])<<8|uint16(src.Pix[si+5])]
			a := float32(uint16(src.Pix[si+6])<<8|uint16(src.Pix[si+7])) / 65535.0
			buf.Pix[di] = r * a
			buf.Pix[di+1] = g * a
			buf.Pix[di+2] = bl * a
			buf.Pix[di+3] = a
			si += 8
			di += 4
		}
	}
}

// fromGray ingests opaque 8-bit grayscale
func (buf *Buffer) fromGray(src *image.Gray) {
	b := src.Bounds()
	di := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		si := src.PixOffset(b.Min.X, y)
		for x := b.Min.X; x < b.Max.X; x++ {
			v := srgb8ToLinearLUT[src.Pix[si]]
			buf.Pix[di] = v
			buf.Pix[di+1] = v
			buf.Pix[di+2] = v
			buf.Pix[di+3] = 1
			si++
			di += 4
		}
	}
}

// fromGray16 ingests opaque 16-bit grayscale
func (buf *Buffer) fromGray16(src *image.Gray16) {
	b := src.Bounds()
	di := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		si := src.PixOffset(b.Min.X, y)
		for x := b.Min.X; x < b.Max.X; x++ {
			v := srgb16ToLinearLUT[uint16(src.Pix[si])<<8|uint16(src.Pix[si+1])]
			buf.Pix[di] = v
			buf.Pix[di+1] = v
			buf.Pix[di+2] = v
			buf.Pix[di+3] = 1
			si += 2
			di += 4
		}
	}
}

// fromGeneric ingests any image type by asking for straight 16-bit sRGB, the
// correct-but-slower fallback for YCbCr, paletted, premultiplied RGBA, etc
func (buf *Buffer) fromGeneric(img image.Image) {
	b := img.Bounds()
	di := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBA64Model.Convert(img.At(x, y)).(color.NRGBA64)
			r := srgb16ToLinearLUT[c.R]
			g := srgb16ToLinearLUT[c.G]
			bl := srgb16ToLinearLUT[c.B]
			a := float32(c.A) / 65535.0
			buf.Pix[di] = r * a
			buf.Pix[di+1] = g * a
			buf.Pix[di+2] = bl * a
			buf.Pix[di+3] = a
			di += 4
		}
	}
}

// ToImage encodes the Buffer back to an image.Image: premultiplied to straight
// alpha, linear light to sRGB. bitDepth 8 returns an *image.NRGBA with ordered
// dithering to hide banding, bitDepth 16 returns an *image.NRGBA64 without
// dither since 16-bit steps are already below the visible threshold. Any other
// bitDepth is treated as 8.
func (b *Buffer) ToImage(bitDepth int) image.Image {
	if bitDepth == 16 {
		return b.toNRGBA64()
	}
	return b.toNRGBA()
}

// toNRGBA produces dithered 8-bit straight-alpha sRGB output
func (b *Buffer) toNRGBA() *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, b.Width, b.Height))
	si := 0
	for y := 0; y < b.Height; y++ {
		di := dst.PixOffset(0, y)
		for x := 0; x < b.Width; x++ {
			r, g, bl, a := unpremultiply(b.Pix[si], b.Pix[si+1], b.Pix[si+2], b.Pix[si+3])
			d := ditherOffset(x, y)
			dst.Pix[di] = quantize8(LinearToSRGB(r), d)
			dst.Pix[di+1] = quantize8(LinearToSRGB(g), d)
			dst.Pix[di+2] = quantize8(LinearToSRGB(bl), d)
			dst.Pix[di+3] = quantize8(a, d)
			si += 4
			di += 4
		}
	}
	return dst
}

// toNRGBA64 produces undithered 16-bit straight-alpha sRGB output
func (b *Buffer) toNRGBA64() *image.NRGBA64 {
	dst := image.NewNRGBA64(image.Rect(0, 0, b.Width, b.Height))
	si := 0
	for y := 0; y < b.Height; y++ {
		di := dst.PixOffset(0, y)
		for x := 0; x < b.Width; x++ {
			r, g, bl, a := unpremultiply(b.Pix[si], b.Pix[si+1], b.Pix[si+2], b.Pix[si+3])
			put16(dst.Pix[di:], quantize16(LinearToSRGB(r)))
			put16(dst.Pix[di+2:], quantize16(LinearToSRGB(g)))
			put16(dst.Pix[di+4:], quantize16(LinearToSRGB(bl)))
			put16(dst.Pix[di+6:], quantize16(a))
			si += 4
			di += 8
		}
	}
	return dst
}

// unpremultiply recovers straight color from premultiplied, returning
// transparent black when alpha is zero
func unpremultiply(r, g, bl, a float32) (sr, sg, sb, sa float32) {
	if a <= 0 {
		return 0, 0, 0, 0
	}
	if a >= 1 {
		return clamp01(r), clamp01(g), clamp01(bl), 1
	}
	inv := 1.0 / a
	return clamp01(r * inv), clamp01(g * inv), clamp01(bl * inv), a
}

// put16 writes a big-endian 16-bit sample, the layout image.NRGBA64 expects
func put16(p []uint8, v uint16) {
	p[0] = uint8(v >> 8)
	p[1] = uint8(v)
}

// quantize8 rounds an sRGB-encoded value in [0,1] to an 8-bit code with an
// ordered dither offset added first
func quantize8(v, dither float32) uint8 {
	q := v*255.0 + dither
	if q < 0 {
		return 0
	}
	if q > 255 {
		return 255
	}
	return uint8(q + 0.5)
}

// quantize16 rounds an sRGB-encoded value in [0,1] to a 16-bit code
func quantize16(v float32) uint16 {
	q := v * 65535.0
	if q < 0 {
		return 0
	}
	if q > 65535 {
		return 65535
	}
	return uint16(q + 0.5)
}

// bayer8 is the classic 8x8 ordered-dither threshold matrix
var bayer8 = [64]float32{
	0, 32, 8, 40, 2, 34, 10, 42,
	48, 16, 56, 24, 50, 18, 58, 26,
	12, 44, 4, 36, 14, 46, 6, 38,
	60, 28, 52, 20, 62, 30, 54, 22,
	3, 35, 11, 43, 1, 33, 9, 41,
	51, 19, 59, 27, 49, 17, 57, 25,
	15, 47, 7, 39, 13, 45, 5, 37,
	63, 31, 55, 23, 61, 29, 53, 21,
}

// ditherOffset returns a per-pixel code-space offset in [-0.5,0.5) drawn from
// the Bayer matrix. Applying it only at quantization keeps the float32 working
// buffer undithered as the spec requires
func ditherOffset(x, y int) float32 {
	return (bayer8[(y&7)*8+(x&7)]+0.5)/64.0 - 0.5
}
