// Command banner renders the project banner shown at the top of the README.
// Every pixel comes from impasto itself: the lettering is an SVG outline fed
// through the path package, and everything on top of it uses the layer, mask,
// gradient, and effect packages.
package main

import (
	"image/color"
	"math"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/canvas"
	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/examples/internal/demo"
	"github.com/odevine/impasto/gradient"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

const (
	W = 1280
	H = 400

	// Width the lettering occupies, which sets the scale for everything else
	markWidth = 850
)

func main() {
	scale := float32(markWidth) / viewWidth
	markHeight := viewHeight * scale
	originX := float32(W-markWidth) / 2
	originY := float32(H)/2 - markHeight/2 - 10

	word, err := demo.SVGPath(wordmark, scale, originX, originY-viewMinY*scale)
	if err != nil {
		panic(err)
	}

	doc := &canvas.Document{
		Width: W, Height: H,
		Root: canvas.Group{
			PassThrough: true, Opacity: 1,
			Layers: []canvas.Node{
				background(),
				glow(268, 138, 290, color.NRGBA{R: 255, G: 146, B: 51, A: 255}, 0.44),
				glow(1062, 300, 300, color.NRGBA{R: 236, G: 64, B: 122, A: 255}, 0.36),
				glow(624, 356, 280, color.NRGBA{R: 41, G: 182, B: 214, A: 255}, 0.20),
				vignette(),
				lettering(word),
				rule(originY + markHeight + 44),
			},
		},
	}

	demo.Save(canvas.MustRender(doc), "examples/out/banner.png")
}

// background lays down the diagonal base gradient the rest of the art sits on
func background() canvas.Node {
	buf := raster.MustNewBuffer(W, H)
	gradient.New(gradient.Linear, gradient.Pad,
		path.Point{X: 0, Y: 0}, path.Point{X: W, Y: H},
		[]gradient.Stop{
			{Pos: 0, Color: color.NRGBA{R: 8, G: 9, B: 28, A: 255}, Opacity: 1},
			{Pos: 0.55, Color: color.NRGBA{R: 27, G: 18, B: 54, A: 255}, Opacity: 1},
			{Pos: 1, Color: color.NRGBA{R: 58, G: 21, B: 64, A: 255}, Opacity: 1},
		}).Render(buf)
	return &canvas.Layer{Content: buf, Opacity: 1, Mode: blend.Normal}
}

// glow is a soft disc of colored light. The gradient fades to transparent at
// its rim, so it needs no mask, and Screen keeps it additive over the backdrop
func glow(cx, cy, radius float32, c color.Color, opacity float32) canvas.Node {
	buf := raster.MustNewBuffer(W, H)
	gradient.New(gradient.Radial, gradient.Pad,
		path.Point{X: cx, Y: cy}, path.Point{X: cx + radius, Y: cy},
		[]gradient.Stop{
			{Pos: 0, Color: c, Opacity: 1},
			{Pos: 0.4, Color: c, Opacity: 0.35},
			{Pos: 1, Color: c, Opacity: 0},
		}).Render(buf)
	return &canvas.Layer{Content: buf, Opacity: opacity, Mode: blend.Screen}
}

// vignette darkens the corners so the lettering holds the center. It is a
// radial gradient that starts fully transparent and ends near black
func vignette() canvas.Node {
	buf := raster.MustNewBuffer(W, H)
	gradient.New(gradient.Radial, gradient.Pad,
		path.Point{X: W / 2, Y: H / 2}, path.Point{X: W/2 + 760, Y: H / 2},
		[]gradient.Stop{
			{Pos: 0, Color: color.Black, Opacity: 0},
			{Pos: 0.45, Color: color.Black, Opacity: 0},
			{Pos: 1, Color: color.NRGBA{R: 5, G: 4, B: 16, A: 255}, Opacity: 0.88},
		}).Render(buf)
	return &canvas.Layer{Content: buf, Opacity: 1, Mode: blend.Normal}
}

// lettering is the wordmark: a solid fill shaped by a vector mask, wearing the
// full effect stack. Listed order does not matter, canvas sorts effects into
// Photoshop's fixed stacking order before compositing
func lettering(word *path.Path) canvas.Node {
	mid := float32(H) / 2
	warm := gradient.New(gradient.Linear, gradient.Pad,
		path.Point{X: 0, Y: mid - 92}, path.Point{X: 0, Y: mid + 92},
		[]gradient.Stop{
			{Pos: 0, Color: color.NRGBA{R: 255, G: 233, B: 188, A: 255}, Opacity: 1},
			{Pos: 0.45, Color: color.NRGBA{R: 255, G: 174, B: 69, A: 255}, Opacity: 1},
			{Pos: 1, Color: color.NRGBA{R: 238, G: 90, B: 60, A: 255}, Opacity: 1},
		})

	return &canvas.Layer{
		Content: demo.Solid(W, H, color.White),
		Opacity: 1,
		Mode:    blend.Normal,
		Mask:    mask.NewVectorMask(word, W, H, path.NonZero),
		Effects: []effects.Effect{
			&effects.DropShadow{
				Color: color.NRGBA{R: 6, G: 3, B: 16, A: 255}, Opacity: 0.8,
				Angle: math.Pi / 3, Distance: 13, BlurRadius: 26, Mode: blend.Multiply,
			},
			&effects.OuterGlow{
				Color: color.NRGBA{R: 255, G: 138, B: 66, A: 255}, Opacity: 0.55,
				BlurRadius: 30, Spread: 3,
			},
			&effects.GradientOverlay{Gradient: warm, Opacity: 1},
			&effects.BevelEmboss{
				Style: effects.BevelInner, Depth: 1.8, Size: 5, Soften: 1,
				Angle: 5 * math.Pi / 4, Altitude: math.Pi / 5, Direction: effects.DirUp,
				HighlightOpacity: 0.4, ShadowOpacity: 0.5,
			},
			&effects.Stroke{
				Width: 3, Color: color.NRGBA{R: 255, G: 250, B: 240, A: 255},
				Alignment: effects.StrokeOutside, Opacity: 0.85,
			},
		},
	}
}

// rule is the dashed underline, there to show the stroker doing caps and dashes
func rule(y float32) canvas.Node {
	buf := raster.MustNewBuffer(W, H)
	half := float32(markWidth) / 2
	line := path.New().MoveTo(float32(W)/2-half, y).LineTo(float32(W)/2+half, y)
	path.StrokePath(buf, line, path.StrokeStyle{
		Width: 5, Cap: path.CapRound, Dash: []float32{2, 18},
	}, path.FlatColorSRGB(color.NRGBA{R: 255, G: 206, B: 156, A: 255}))
	return &canvas.Layer{Content: buf, Opacity: 0.85, Mode: blend.Screen}
}
