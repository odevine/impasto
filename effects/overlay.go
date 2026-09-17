package effects

import (
	"image/color"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/gradient"
	"github.com/odevine/impasto/raster"
)

// ColorOverlay fills the layer's shape with a flat color. Mode defaults to Normal
type ColorOverlay struct {
	Color   color.Color
	Opacity float32
	Mode    blend.Mode
}

// Render produces the color overlay buffer, clipped to the layer and placed over
// it
func (o *ColorOverlay) Render(layer *raster.Buffer) []Rendered {
	rgb, _ := linearColor(o.Color)
	alpha := extractAlpha(layer)
	buf := colorize(alpha, layer.Width, layer.Height, rgb, 1)
	return []Rendered{{
		Pixels:  buf,
		Behind:  false,
		Mode:    o.Mode,
		Opacity: opacityOr(o.Opacity),
	}}
}

// GradientOverlay fills the layer's shape with a gradient field. Mode defaults to
// Normal
type GradientOverlay struct {
	Gradient *gradient.Gradient
	Opacity  float32
	Mode     blend.Mode
}

// Render produces the gradient overlay buffer, clipped to the layer and placed
// over it
func (o *GradientOverlay) Render(layer *raster.Buffer) []Rendered {
	buf := raster.MustNewBuffer(layer.Width, layer.Height)
	if o.Gradient != nil {
		o.Gradient.Render(buf)
	}
	clipToLayer(buf, layer)
	return []Rendered{{
		Pixels:  buf,
		Behind:  false,
		Mode:    o.Mode,
		Opacity: opacityOr(o.Opacity),
	}}
}
