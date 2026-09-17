// Package mask defines coverage sources that scale a layer's alpha before it is
// composited. A mask answers one question per pixel: how much of the layer
// shows through, as a value in [0,1]. Raster masks read that from a grayscale
// buffer, vector masks rasterize a path to the same thing (see vector.go), and
// clip-to-below is handled by canvas because it is a compositing-order rule
// rather than a mask. Multiple masks on one layer multiply together.
package mask

import (
	"image"
	"image/color"

	"github.com/odevine/impasto/internal/parallel"
	"github.com/odevine/impasto/raster"
)

// Mask reports per-pixel coverage in [0,1]. Coordinates outside the mask's own
// extent return 0, so a mask only ever reveals, never invents, pixels
type Mask interface {
	Coverage(x, y int) float32
}

// RasterMask is a grayscale coverage buffer. Coverage is stored directly as an
// alpha-like quantity, not a color, so it is never gamma decoded
type RasterMask struct {
	Cov    []float32
	Width  int
	Height int
}

// NewRasterMask wraps an existing coverage slice. It panics if the slice length
// does not match the dimensions, which is always a caller bug
func NewRasterMask(cov []float32, w, h int) *RasterMask {
	if len(cov) != w*h {
		panic("mask: coverage length does not match dimensions")
	}
	return &RasterMask{Cov: cov, Width: w, Height: h}
}

// NewRasterMaskFromImage reads coverage from a grayscale image. The luminance of
// each pixel becomes coverage, scaled by the pixel's own alpha so transparent
// regions of the mask reveal nothing.
func NewRasterMaskFromImage(img image.Image) *RasterMask {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	cov := make([]float32, w*h)
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			luma := (299*float32(c.R) + 587*float32(c.G) + 114*float32(c.B)) / 1000.0
			cov[i] = luma / 255.0 * float32(c.A) / 255.0
			i++
		}
	}
	return &RasterMask{Cov: cov, Width: w, Height: h}
}

// Coverage returns the stored value, or 0 outside the buffer
func (m *RasterMask) Coverage(x, y int) float32 {
	if x < 0 || y < 0 || x >= m.Width || y >= m.Height {
		return 0
	}
	return m.Cov[y*m.Width+x]
}

// FuncMask adapts a plain function to the Mask interface for callers that
// generate coverage procedurally
type FuncMask func(x, y int) float32

// Coverage calls the underlying function
func (f FuncMask) Coverage(x, y int) float32 { return f(x, y) }

// Multi combines several masks by multiplying their coverage, the defined
// semantics when a layer carries a raster and a vector mask at once. An empty
// Multi is fully opaque
type Multi []Mask

// Coverage returns the product of every member's coverage at (x,y)
func (ms Multi) Coverage(x, y int) float32 {
	cov := float32(1)
	for _, m := range ms {
		cov *= m.Coverage(x, y)
		if cov == 0 {
			return 0
		}
	}
	return cov
}

// Apply multiplies a mask's coverage into a premultiplied buffer in place.
// Scaling all four premultiplied components by the same coverage keeps the pixel
// premultiplied and correctly attenuates both color and alpha. The buffer's
// top-left is treated as the mask's origin (0,0).
func Apply(dst *raster.Buffer, m Mask) {
	if m == nil {
		return
	}
	w := dst.Width
	parallel.Rows(dst.Height, func(lo, hi int) {
		for y := lo; y < hi; y++ {
			i := y * w * 4
			for x := 0; x < w; x++ {
				c := m.Coverage(x, y)
				dst.Pix[i] *= c
				dst.Pix[i+1] *= c
				dst.Pix[i+2] *= c
				dst.Pix[i+3] *= c
				i += 4
			}
		}
	})
}
