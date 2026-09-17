package effects

import (
	"image/color"
	"math"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/blur"
	"github.com/odevine/impasto/raster"
)

// OuterGlow radiates color outward from the layer's edge. It is the drop shadow
// shape without the offset: spread the alpha, blur it, colorize. Mode defaults to
// Screen
type OuterGlow struct {
	Color      color.Color
	Opacity    float32
	BlurRadius float32
	Spread     float32
	Mode       blend.Mode
}

// Render produces the outer glow buffer, placed behind the layer
func (g *OuterGlow) Render(layer *raster.Buffer) []Rendered {
	w, h := layer.Width, layer.Height
	alpha := extractAlpha(layer)
	if g.Spread > 0 {
		alpha = dilate(alpha, w, h, int(math.Round(float64(g.Spread))))
	}
	rgb, _ := linearColor(g.Color)
	buf := colorize(alpha, w, h, rgb, 1)
	blur.Gaussian(buf, g.BlurRadius)

	return []Rendered{{
		Pixels:  buf,
		Behind:  true,
		Mode:    modeOr(g.Mode, blend.Screen),
		Opacity: opacityOr(g.Opacity),
	}}
}

// InnerGlow radiates color inward from the layer's edge, clipped to the layer.
// Mode defaults to Screen
type InnerGlow struct {
	Color      color.Color
	Opacity    float32
	BlurRadius float32
	Spread     float32
	Mode       blend.Mode
}

// Render produces the inner glow buffer, placed over the layer
func (g *InnerGlow) Render(layer *raster.Buffer) []Rendered {
	w, h := layer.Width, layer.Height
	alpha := extractAlpha(layer)

	// Glow grows from the layer's inner edge, so start from the inverse and clip
	inv := make([]float32, len(alpha))
	for i, a := range alpha {
		inv[i] = 1 - a
	}
	if g.Spread > 0 {
		inv = dilate(inv, w, h, int(math.Round(float64(g.Spread))))
	}
	rgb, _ := linearColor(g.Color)
	buf := colorize(inv, w, h, rgb, 1)
	blur.Gaussian(buf, g.BlurRadius)
	clipToLayer(buf, layer)

	return []Rendered{{
		Pixels:  buf,
		Behind:  false,
		Mode:    modeOr(g.Mode, blend.Screen),
		Opacity: opacityOr(g.Opacity),
	}}
}
