// Command effects renders each of the eight layer styles on a rounded-rectangle
// badge, in a 4x2 grid. Glow cells use a dark backdrop so the glow reads.
package main

import (
	"image/color"

	"github.com/odevine/impasto/canvas"
	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/examples/internal/demo"
	"github.com/odevine/impasto/gradient"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

const (
	cell   = 190
	gap    = 8
	margin = 12
)

var (
	blue  = color.NRGBA{60, 120, 220, 255}
	light = color.NRGBA{225, 227, 230, 255}
	dark  = color.NRGBA{28, 30, 36, 255}
)

func main() {
	cells := []*raster.Buffer{
		effectCell(light, &effects.DropShadow{Color: color.Black, Opacity: 0.6, Angle: 2.36, Distance: 10, BlurRadius: 12}),
		effectCell(light, &effects.InnerShadow{Color: color.Black, Opacity: 0.7, Angle: 2.36, Distance: 8, BlurRadius: 8}),
		effectCell(dark, &effects.OuterGlow{Color: color.NRGBA{80, 200, 255, 255}, Opacity: 1, BlurRadius: 16, Spread: 2}),
		effectCell(dark, &effects.InnerGlow{Color: color.NRGBA{180, 240, 255, 255}, Opacity: 1, BlurRadius: 14, Spread: 1}),
		effectCell(light, &effects.ColorOverlay{Color: color.NRGBA{240, 140, 40, 255}, Opacity: 1}),
		effectCell(light, &effects.GradientOverlay{Gradient: overlayGrad(), Opacity: 1}),
		effectCell(light, &effects.Stroke{Width: 7, Color: color.NRGBA{255, 255, 255, 255}, Alignment: effects.StrokeOutside, Opacity: 1}),
		effectCell(light, &effects.BevelEmboss{Depth: 3, Size: 6, Soften: 1, Angle: 2.36, Altitude: 0.6, HighlightOpacity: 0.9, ShadowOpacity: 0.6}),
	}

	cols := 4
	rows := 2
	w := cols*cell + (cols-1)*gap + 2*margin
	h := rows*cell + (rows-1)*gap + 2*margin
	out := demo.Solid(w, h, color.NRGBA{18, 19, 22, 255})
	for i, c := range cells {
		cx := margin + (i%cols)*(cell+gap)
		cy := margin + (i/cols)*(cell+gap)
		canvas.PlaceInto(out, c, cx, cy)
	}
	demo.Save(out, "examples/out/effects.png")
}

// effectCell draws a blue badge carrying a single effect over the given backdrop
func effectCell(bg color.Color, eff effects.Effect) *raster.Buffer {
	shape := demo.RoundRect(45, 45, cell-45, cell-45, 26)
	layer := &canvas.Layer{
		Content: demo.Solid(cell, cell, blue),
		Opacity: 1,
		Mask:    mask.NewVectorMask(shape, cell, cell, path.NonZero),
		Effects: []effects.Effect{eff},
	}
	doc := &canvas.Document{
		Width: cell, Height: cell,
		Root: canvas.Group{PassThrough: true, Opacity: 1, Layers: []canvas.Node{
			&canvas.Layer{Content: demo.Solid(cell, cell, bg), Opacity: 1},
			layer,
		}},
	}
	return canvas.MustRender(doc)
}

func overlayGrad() *gradient.Gradient {
	return gradient.New(gradient.Linear, gradient.Pad,
		path.Point{X: 40, Y: 40}, path.Point{X: cell - 40, Y: cell - 40},
		[]gradient.Stop{
			{Pos: 0, Color: color.NRGBA{255, 90, 120, 255}, Opacity: 1},
			{Pos: 1, Color: color.NRGBA{255, 210, 90, 255}, Opacity: 1},
		})
}
