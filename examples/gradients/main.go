// Command gradients renders the five gradient types plus a per-stop opacity fade
// shown over a checkerboard, in a 3x2 grid.
package main

import (
	"image/color"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/canvas"
	"github.com/odevine/impasto/examples/internal/demo"
	"github.com/odevine/impasto/gradient"
	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

const (
	cell   = 180
	gap    = 8
	margin = 12
)

func main() {
	w := 3*cell + 2*gap + 2*margin
	h := 2*cell + gap + 2*margin
	out := demo.Solid(w, h, color.NRGBA{24, 26, 30, 255})

	rainbow := []gradient.Stop{
		{Pos: 0, Color: color.NRGBA{230, 40, 70, 255}, Opacity: 1},
		{Pos: 0.25, Color: color.NRGBA{240, 180, 40, 255}, Opacity: 1},
		{Pos: 0.5, Color: color.NRGBA{60, 200, 90, 255}, Opacity: 1},
		{Pos: 0.75, Color: color.NRGBA{50, 140, 240, 255}, Opacity: 1},
		{Pos: 1, Color: color.NRGBA{150, 70, 220, 255}, Opacity: 1},
	}
	mid := float32(cell) / 2

	cells := []*raster.Buffer{
		field(gradient.New(gradient.Linear, gradient.Pad,
			path.Point{X: 0, Y: 0}, path.Point{X: cell, Y: cell}, rainbow)),
		field(gradient.New(gradient.Radial, gradient.Pad,
			path.Point{X: mid, Y: mid}, path.Point{X: mid, Y: 0}, rainbow)),
		field(gradient.New(gradient.Angle, gradient.Pad,
			path.Point{X: mid, Y: mid}, path.Point{X: cell, Y: mid}, rainbow)),
		field(gradient.New(gradient.Reflected, gradient.Pad,
			path.Point{X: mid, Y: mid}, path.Point{X: cell, Y: mid}, rainbow)),
		field(gradient.New(gradient.Diamond, gradient.Pad,
			path.Point{X: mid, Y: mid}, path.Point{X: cell, Y: mid}, rainbow)),
		fadeCell(),
	}

	for i, c := range cells {
		cx := margin + (i%3)*(cell+gap)
		cy := margin + (i/3)*(cell+gap)
		canvas.PlaceInto(out, c, cx, cy)
	}
	demo.Save(out, "examples/out/gradients.png")
}

// field renders a gradient into a fresh cell-sized buffer
func field(g *gradient.Gradient) *raster.Buffer {
	buf := raster.MustNewBuffer(cell, cell)
	g.Render(buf)
	return buf
}

// fadeCell shows a color fading to transparent over a checkerboard, proving
// per-stop opacity
func fadeCell() *raster.Buffer {
	base := demo.Checker(cell, cell, 16, color.NRGBA{210, 210, 210, 255}, color.NRGBA{150, 150, 150, 255})
	top := raster.MustNewBuffer(cell, cell)
	gradient.New(gradient.Linear, gradient.Pad,
		path.Point{X: 0, Y: 0}, path.Point{X: cell, Y: 0},
		[]gradient.Stop{
			{Pos: 0, Color: color.NRGBA{220, 40, 120, 255}, Opacity: 1},
			{Pos: 1, Color: color.NRGBA{220, 40, 120, 255}, Opacity: 0},
		}).Render(top)
	doc := &canvas.Document{
		Width: cell, Height: cell,
		Root: canvas.Group{PassThrough: true, Opacity: 1, Layers: []canvas.Node{
			&canvas.Layer{Content: base, Opacity: 1},
			&canvas.Layer{Content: top, Opacity: 1, Mode: blend.Normal},
		}},
	}
	return canvas.MustRender(doc)
}
