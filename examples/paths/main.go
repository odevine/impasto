// Command paths renders stroke caps, stroke joins, dashes, the two fill rules,
// and a stroked bezier, all drawn directly onto one buffer.
package main

import (
	"image/color"

	"github.com/odevine/impasto/examples/internal/demo"
	"github.com/odevine/impasto/path"
)

func main() {
	const w, h = 720, 440
	out := demo.Solid(w, h, color.NRGBA{240, 241, 243, 255})

	ink := path.FlatColorSRGB(color.NRGBA{40, 44, 52, 255})
	orange := path.FlatColorSRGB(color.NRGBA{240, 140, 40, 255})
	blue := path.FlatColorSRGB(color.NRGBA{50, 120, 220, 255})
	teal := path.FlatColorSRGB(color.NRGBA{30, 170, 160, 255})

	// Row 1: caps (butt, round, square)
	caps := []path.Cap{path.CapButt, path.CapRound, path.CapSquare}
	for i, c := range caps {
		x := float32(50 + i*230)
		seg := path.New().MoveTo(x, 55).LineTo(x+150, 55)
		path.StrokePath(out, seg, path.StrokeStyle{Width: 26, Cap: c}, ink)
	}

	// Row 2: joins (miter, round, bevel)
	joins := []path.Join{path.JoinMiter, path.JoinRound, path.JoinBevel}
	for i, j := range joins {
		x := float32(50 + i*230)
		v := path.New().MoveTo(x, 200).LineTo(x+75, 130).LineTo(x+150, 200)
		path.StrokePath(out, v, path.StrokeStyle{Width: 22, Join: j, Cap: path.CapButt, MiterLimit: 8}, blue)
	}

	// Row 3: a dashed line
	dash := path.New().MoveTo(50, 260).LineTo(670, 260)
	path.StrokePath(out, dash, path.StrokeStyle{Width: 10, Cap: path.CapRound, Dash: []float32{28, 16}}, teal)

	// Row 4: a self-intersecting pentagram, nonzero (solid) vs even-odd (hollow
	// center pentagon), the two fill rules side by side
	nz := demo.Pentagram(150, 370, 62)
	path.Fill(out, nz, path.NonZero, orange)
	path.StrokePath(out, nz, path.StrokeStyle{Width: 3, Join: path.JoinRound}, ink)

	eo := demo.Pentagram(360, 370, 62)
	path.Fill(out, eo, path.EvenOdd, orange)
	path.StrokePath(out, eo, path.StrokeStyle{Width: 3, Join: path.JoinRound}, ink)

	// Row 4 right: a stroked cubic bezier wave
	wave := path.New().MoveTo(470, 400).
		CubicTo(520, 300, 580, 460, 630, 350).
		CubicTo(660, 300, 690, 380, 700, 340)
	path.StrokePath(out, wave, path.StrokeStyle{Width: 8, Cap: path.CapRound, Join: path.JoinRound}, blue)

	demo.Save(out, "examples/out/paths.png")
}
