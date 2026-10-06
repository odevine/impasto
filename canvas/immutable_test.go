package canvas

import (
	"image/color"
	"slices"
	"testing"

	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/gradient"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/path"
)

// TestRenderLeavesContentUntouched pins that Render never writes to a caller's
// content buffer, which lets renderLayer skip the clone when nothing mutates it
func TestRenderLeavesContentUntouched(t *testing.T) {
	const w, h = 48, 40
	blob := func() *Layer {
		c := fill(w, h, 0, 0, 0, 0)
		for y := 10; y < 30; y++ {
			for x := 12; x < 36; x++ {
				i := (y*w + x) * 4
				c.Pix[i], c.Pix[i+1], c.Pix[i+2], c.Pix[i+3] = 0.4, 0.2, 0.1, 0.5
			}
		}
		return &Layer{Content: c, Opacity: 0.8}
	}

	allEffects := []effects.Effect{
		&effects.DropShadow{Color: color.Black, Opacity: 0.7, Angle: 2.36, Distance: 4, BlurRadius: 3},
		&effects.OuterGlow{Color: color.White, Opacity: 0.5, BlurRadius: 3, Spread: 1},
		&effects.ColorOverlay{Color: color.RGBA{R: 255, A: 255}, Opacity: 0.5},
		&effects.GradientOverlay{Gradient: gradient.New(gradient.Linear, gradient.Pad, path.Point{}, path.Point{X: w, Y: h}, []gradient.Stop{
			{Pos: 0, Color: color.Black, Opacity: 1},
			{Pos: 1, Color: color.White, Opacity: 1},
		})},
		&effects.Stroke{Width: 2, Color: color.White, Alignment: effects.StrokeOutside, Opacity: 1},
		&effects.InnerGlow{Color: color.White, Opacity: 0.5, BlurRadius: 3, Spread: 1},
		&effects.InnerShadow{Color: color.Black, Opacity: 0.5, Distance: 2, BlurRadius: 2},
		&effects.BevelEmboss{},
	}

	plain, masked, clipped := blob(), blob(), blob()
	plain.Effects = allEffects
	masked.Mask = mask.FuncMask(func(x, y int) float32 { return 0.5 })
	clipped.ClipToBelow = true
	layers := []*Layer{plain, masked, clipped}

	before := make([][]float32, len(layers))
	nodes := make([]Node, len(layers))
	for i, l := range layers {
		before[i] = slices.Clone(l.Content.Pix)
		nodes[i] = l
	}
	MustRender(&Document{Width: w, Height: h, Root: Group{PassThrough: true, Layers: nodes}})
	for i, l := range layers {
		if !slices.Equal(before[i], l.Content.Pix) {
			t.Errorf("layer %d content was modified by Render", i)
		}
	}
}
