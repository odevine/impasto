// Command blendmodes renders a 4x4 chart of every blend mode, each cell blending
// a vertical color ramp over a horizontal rainbow so the modes' behavior is
// visible at a glance. Cell order is the blend.Mode iota order.
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
	cell   = 150
	gap    = 8
	cols   = 4
	rows   = 4
	margin = 12
)

func main() {
	modes := []blend.Mode{
		blend.Normal, blend.Multiply, blend.Screen, blend.Overlay,
		blend.SoftLight, blend.HardLight, blend.ColorDodge, blend.ColorBurn,
		blend.Darken, blend.Lighten, blend.Difference, blend.Exclusion,
		blend.Hue, blend.Saturation, blend.Color, blend.Luminosity,
	}

	w := cols*cell + (cols-1)*gap + 2*margin
	h := rows*cell + (rows-1)*gap + 2*margin
	out := demo.Solid(w, h, color.NRGBA{24, 26, 30, 255})

	for i, m := range modes {
		c := renderCell(m)
		cx := margin + (i%cols)*(cell+gap)
		cy := margin + (i/cols)*(cell+gap)
		canvas.PlaceInto(out, c, cx, cy)
	}

	demo.Save(out, "examples/out/blendmodes.png")

	// Also export the two source gradients so the chart's inputs are visible
	demo.Save(makeBase(), "examples/out/blendmodes-base.png")
	demo.Save(makeOverlay(), "examples/out/blendmodes-overlay.png")
}

// makeBase is the horizontal rainbow that every cell uses as its backdrop
func makeBase() *raster.Buffer {
	base := raster.MustNewBuffer(cell, cell)
	gradient.New(gradient.Linear, gradient.Pad,
		path.Point{X: 0, Y: 0}, path.Point{X: cell, Y: 0},
		[]gradient.Stop{
			{Pos: 0, Color: color.NRGBA{255, 40, 40, 255}, Opacity: 1},
			{Pos: 0.33, Color: color.NRGBA{40, 220, 60, 255}, Opacity: 1},
			{Pos: 0.66, Color: color.NRGBA{40, 90, 255, 255}, Opacity: 1},
			{Pos: 1, Color: color.NRGBA{255, 220, 40, 255}, Opacity: 1},
		}).Render(base)
	return base
}

// makeOverlay is the vertical ramp that every cell blends over the base
func makeOverlay() *raster.Buffer {
	top := raster.MustNewBuffer(cell, cell)
	gradient.New(gradient.Linear, gradient.Pad,
		path.Point{X: 0, Y: 0}, path.Point{X: 0, Y: cell},
		[]gradient.Stop{
			{Pos: 0, Color: color.NRGBA{255, 255, 255, 255}, Opacity: 1},
			{Pos: 0.5, Color: color.NRGBA{120, 60, 200, 255}, Opacity: 1},
			{Pos: 1, Color: color.NRGBA{0, 0, 0, 255}, Opacity: 1},
		}).Render(top)
	return top
}

// renderCell blends the overlay ramp over the base rainbow using one mode
func renderCell(m blend.Mode) *raster.Buffer {
	doc := &canvas.Document{
		Width: cell, Height: cell,
		Root: canvas.Group{
			PassThrough: true, Opacity: 1,
			Layers: []canvas.Node{
				&canvas.Layer{Content: makeBase(), Opacity: 1, Mode: blend.Normal},
				&canvas.Layer{Content: makeOverlay(), Opacity: 1, Mode: m},
			},
		},
	}
	return canvas.MustRender(doc)
}
