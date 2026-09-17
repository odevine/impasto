package path

import (
	"image/color"

	"github.com/odevine/impasto/raster"
)

// Paint supplies the premultiplied linear color to fill a path with at each
// pixel. A flat color returns the same value everywhere, a gradient returns a
// spatially varying field. Keeping this an interface lets the gradient package
// act as a paint source without path depending on it.
type Paint interface {
	ColorAt(x, y int) [4]float32
}

// FlatColor is a single premultiplied linear RGBA value used everywhere
type FlatColor [4]float32

// ColorAt returns the constant color
func (c FlatColor) ColorAt(x, y int) [4]float32 { return c }

// NewFlatColor builds a FlatColor from straight linear-light components in [0,1]
func NewFlatColor(r, g, b, a float32) FlatColor {
	return FlatColor{r * a, g * a, b * a, a}
}

// FlatColorSRGB builds a FlatColor from a standard library color, decoding sRGB
// to linear light so it matches the working color space
func FlatColorSRGB(c color.Color) FlatColor {
	nc := color.NRGBAModel.Convert(c).(color.NRGBA)
	r := raster.SRGBToLinear(float32(nc.R) / 255.0)
	g := raster.SRGBToLinear(float32(nc.G) / 255.0)
	b := raster.SRGBToLinear(float32(nc.B) / 255.0)
	a := float32(nc.A) / 255.0
	return FlatColor{r * a, g * a, b * a, a}
}
