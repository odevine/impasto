// Package effects implements Photoshop-style layer styles as declarative data.
// An effect is a parameter struct, not an imperative pixel call, so effects are
// composable, serializable, and testable in isolation. Each effect renders into
// a full-size premultiplied buffer that canvas composites in a fixed order using
// the effect's own blend mode. Effects build on blend, blur, gradient, mask, and
// path but know nothing about the layer stack above them.
package effects

import (
	"image/color"
	"sort"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/raster"
)

// Effect renders one layer style. Render receives the layer's premultiplied
// content (already masked) and returns one or more contributions. Most effects
// return a single one, bevel returns a highlight and a shadow with different
// blend modes
type Effect interface {
	Render(layer *raster.Buffer) []Rendered
}

// Rendered is an effect's output: the premultiplied pixels plus how canvas
// should place them. Behind means composite under the layer, otherwise over
type Rendered struct {
	Pixels  *raster.Buffer
	Behind  bool
	Mode    blend.Mode
	Opacity float32
}

// Sort returns effects in Photoshop's canonical stacking order so a given set
// always renders the same way regardless of the order the caller listed them
func Sort(list []Effect) []Effect {
	out := append([]Effect(nil), list...)
	sort.SliceStable(out, func(i, j int) bool {
		return stackOrder(out[i]) < stackOrder(out[j])
	})
	return out
}

// stackOrder maps each effect type to its position, back to front
func stackOrder(e Effect) int {
	switch e.(type) {
	case *DropShadow:
		return 0
	case *OuterGlow:
		return 1
	case *ColorOverlay:
		return 2
	case *GradientOverlay:
		return 3
	case *Stroke:
		return 4
	case *InnerGlow:
		return 5
	case *InnerShadow:
		return 6
	case *BevelEmboss:
		return 7
	default:
		return 100
	}
}

// linearColor decodes an sRGB color to straight linear RGB plus its alpha
func linearColor(c color.Color) (rgb [3]float32, a float32) {
	if c == nil {
		return [3]float32{}, 0
	}
	nc := color.NRGBAModel.Convert(c).(color.NRGBA)
	return [3]float32{
		raster.SRGBToLinear(float32(nc.R) / 255.0),
		raster.SRGBToLinear(float32(nc.G) / 255.0),
		raster.SRGBToLinear(float32(nc.B) / 255.0),
	}, float32(nc.A) / 255.0
}

// extractAlpha copies a buffer's alpha channel into a scalar coverage field
func extractAlpha(b *raster.Buffer) []float32 {
	out := make([]float32, b.Width*b.Height)
	for i := range out {
		out[i] = b.Pix[i*4+3]
	}
	return out
}

// colorize builds a premultiplied buffer from a scalar coverage field and a
// straight linear color, scaling coverage by opacity
func colorize(cov []float32, w, h int, rgb [3]float32, opacity float32) *raster.Buffer {
	b := raster.MustNewBuffer(w, h)
	for i, c := range cov {
		a := c * opacity
		b.Pix[i*4] = rgb[0] * a
		b.Pix[i*4+1] = rgb[1] * a
		b.Pix[i*4+2] = rgb[2] * a
		b.Pix[i*4+3] = a
	}
	return b
}

// offsetAlpha shifts a coverage field by (dx,dy) with bilinear sampling, filling
// vacated area with zero. Positive dx moves content right, dy moves it down
func offsetAlpha(cov []float32, w, h int, dx, dy float32) []float32 {
	out := make([]float32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out[y*w+x] = sampleBilinear(cov, w, h, float32(x)-dx, float32(y)-dy)
		}
	}
	return out
}

// sampleBilinear reads a coverage field at fractional coordinates, treating
// out-of-range samples as zero
func sampleBilinear(cov []float32, w, h int, fx, fy float32) float32 {
	x0 := int(floor(fx))
	y0 := int(floor(fy))
	tx := fx - float32(x0)
	ty := fy - float32(y0)
	c00 := at(cov, w, h, x0, y0)
	c10 := at(cov, w, h, x0+1, y0)
	c01 := at(cov, w, h, x0, y0+1)
	c11 := at(cov, w, h, x0+1, y0+1)
	top := c00 + (c10-c00)*tx
	bot := c01 + (c11-c01)*tx
	return top + (bot-top)*ty
}

func at(cov []float32, w, h, x, y int) float32 {
	if x < 0 || y < 0 || x >= w || y >= h {
		return 0
	}
	return cov[y*w+x]
}

// clipToLayer multiplies an effect buffer by the layer's alpha so an inside
// effect never spills past the layer's own shape
func clipToLayer(fx *raster.Buffer, layer *raster.Buffer) {
	for i := 0; i < len(fx.Pix); i += 4 {
		a := layer.Pix[i+3]
		fx.Pix[i] *= a
		fx.Pix[i+1] *= a
		fx.Pix[i+2] *= a
		fx.Pix[i+3] *= a
	}
}

func floor(v float32) float32 {
	i := int(v)
	if float32(i) > v {
		i--
	}
	return float32(i)
}
