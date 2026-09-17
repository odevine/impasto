package effects

import (
	"image/color"
	"math"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/blur"
	"github.com/odevine/impasto/raster"
)

// DropShadow casts a blurred, offset copy of the layer's alpha behind the layer.
// Angle is in radians measured clockwise from the positive x axis, Distance and
// BlurRadius are in pixels, Choke erodes the alpha before blurring to tighten the
// edge. Mode defaults to Multiply
type DropShadow struct {
	Color      color.Color
	Opacity    float32
	Angle      float32
	Distance   float32
	BlurRadius float32
	Choke      float32
	Mode       blend.Mode
}

// Render produces the drop shadow buffer, placed behind the layer
func (d *DropShadow) Render(layer *raster.Buffer) []Rendered {
	w, h := layer.Width, layer.Height
	alpha := extractAlpha(layer)

	if d.Choke > 0 {
		alpha = erode(alpha, w, h, int(math.Round(float64(d.Choke))))
	}
	dx := float32(math.Cos(float64(d.Angle))) * d.Distance
	dy := float32(math.Sin(float64(d.Angle))) * d.Distance
	alpha = offsetAlpha(alpha, w, h, dx, dy)

	rgb, _ := linearColor(d.Color)
	buf := colorize(alpha, w, h, rgb, 1)
	blur.Gaussian(buf, d.BlurRadius)

	return []Rendered{{
		Pixels:  buf,
		Behind:  true,
		Mode:    modeOr(d.Mode, blend.Multiply),
		Opacity: opacityOr(d.Opacity),
	}}
}

// InnerShadow darkens the inside edge of the layer, as if light were blocked at
// the rim. It offsets the inverse of the alpha, blurs it, and clips the result
// back inside the layer. Mode defaults to Multiply
type InnerShadow struct {
	Color      color.Color
	Opacity    float32
	Angle      float32
	Distance   float32
	BlurRadius float32
	Choke      float32
	Mode       blend.Mode
}

// Render produces the inner shadow buffer, placed over the layer
func (s *InnerShadow) Render(layer *raster.Buffer) []Rendered {
	w, h := layer.Width, layer.Height
	alpha := extractAlpha(layer)

	// The shadow originates from where the layer is absent, seen from inside
	inv := make([]float32, len(alpha))
	for i, a := range alpha {
		inv[i] = 1 - a
	}
	if s.Choke > 0 {
		inv = dilate(inv, w, h, int(math.Round(float64(s.Choke))))
	}
	dx := float32(math.Cos(float64(s.Angle))) * s.Distance
	dy := float32(math.Sin(float64(s.Angle))) * s.Distance
	inv = offsetAlpha(inv, w, h, dx, dy)

	rgb, _ := linearColor(s.Color)
	buf := colorize(inv, w, h, rgb, 1)
	blur.Gaussian(buf, s.BlurRadius)
	clipToLayer(buf, layer)

	return []Rendered{{
		Pixels:  buf,
		Behind:  false,
		Mode:    modeOr(s.Mode, blend.Multiply),
		Opacity: opacityOr(s.Opacity),
	}}
}

// modeOr returns m unless it is the zero value Normal, in which case the
// effect's documented default applies. Callers wanting Normal explicitly get the
// default of that effect, which for shadows and glows is what they want anyway
func modeOr(m, def blend.Mode) blend.Mode {
	if m == blend.Normal {
		return def
	}
	return m
}

// opacityOr defaults a zero opacity to fully opaque
func opacityOr(o float32) float32 {
	if o <= 0 {
		return 1
	}
	if o > 1 {
		return 1
	}
	return o
}
