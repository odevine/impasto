// Command showcase composes a single poster that combines gradients, masks,
// groups, blend modes, and the full set of layer effects into one scene.
package main

import (
	"image/color"

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
	W = 640
	H = 800
)

func main() {
	doc := &canvas.Document{
		Width: W, Height: H,
		Root: canvas.Group{
			PassThrough: true, Opacity: 1,
			Layers: []canvas.Node{
				background(),
				glowBlobs(),
				badge(),
				emblem(),
				panel(),
			},
		},
	}
	demo.Save(canvas.MustRender(doc), "examples/out/showcase.png")
}

// background is a deep diagonal gradient
func background() canvas.Node {
	buf := raster.MustNewBuffer(W, H)
	gradient.New(gradient.Linear, gradient.Pad,
		path.Point{X: 0, Y: 0}, path.Point{X: W, Y: H},
		[]gradient.Stop{
			{Pos: 0, Color: color.NRGBA{22, 24, 48, 255}, Opacity: 1},
			{Pos: 0.6, Color: color.NRGBA{48, 30, 72, 255}, Opacity: 1},
			{Pos: 1, Color: color.NRGBA{90, 40, 90, 255}, Opacity: 1},
		}).Render(buf)
	return &canvas.Layer{Content: buf, Opacity: 1}
}

// glowBlobs are soft colored circles blended with Screen for ambient light
func glowBlobs() canvas.Node {
	buf := raster.MustNewBuffer(W, H)
	blobs := []struct {
		x, y, r float32
		c       color.Color
	}{
		{120, 150, 140, color.NRGBA{80, 60, 200, 255}},
		{540, 220, 160, color.NRGBA{200, 60, 140, 255}},
		{500, 680, 150, color.NRGBA{60, 140, 200, 255}},
	}
	for _, b := range blobs {
		circle := path.New().Ellipse(b.x, b.y, b.r, b.r)
		path.Fill(buf, circle, path.NonZero, path.FlatColorSRGB(b.c))
	}
	return &canvas.Layer{Content: buf, Opacity: 0.5, Mode: blend.Screen}
}

// badge is the large disc with a gradient overlay, bevel, stroke, and shadow
func badge() canvas.Node {
	circle := path.New().Ellipse(320, 300, 150, 150)
	return &canvas.Layer{
		Content: demo.Solid(W, H, color.NRGBA{240, 240, 245, 255}),
		Opacity: 1,
		Mask:    mask.NewVectorMask(circle, W, H, path.NonZero),
		Effects: []effects.Effect{
			&effects.DropShadow{Color: color.Black, Opacity: 0.55, Angle: 1.8, Distance: 18, BlurRadius: 28},
			&effects.GradientOverlay{Gradient: gradient.New(gradient.Linear, gradient.Pad,
				path.Point{X: 180, Y: 160}, path.Point{X: 460, Y: 450},
				[]gradient.Stop{
					{Pos: 0, Color: color.NRGBA{255, 130, 90, 255}, Opacity: 1},
					{Pos: 1, Color: color.NRGBA{230, 60, 130, 255}, Opacity: 1},
				}), Opacity: 1},
			&effects.BevelEmboss{Depth: 2, Size: 10, Soften: 2, Angle: 2.36, Altitude: 0.6, HighlightOpacity: 0.6, ShadowOpacity: 0.4},
			&effects.Stroke{Width: 6, Color: color.NRGBA{255, 255, 255, 255}, Alignment: effects.StrokeOutside, Opacity: 0.9},
		},
	}
}

// emblem is a gold star sitting on the badge with an outer glow
func emblem() canvas.Node {
	star := demo.Star(320, 300, 95, 40, 5)
	return &canvas.Layer{
		Content: demo.Solid(W, H, color.NRGBA{255, 214, 90, 255}),
		Opacity: 1,
		Mask:    mask.NewVectorMask(star, W, H, path.NonZero),
		Effects: []effects.Effect{
			&effects.OuterGlow{Color: color.NRGBA{255, 240, 180, 255}, Opacity: 0.9, BlurRadius: 18, Spread: 2},
			&effects.DropShadow{Color: color.NRGBA{120, 60, 0, 255}, Opacity: 0.5, Angle: 1.8, Distance: 6, BlurRadius: 8},
			&effects.BevelEmboss{Depth: 3, Size: 4, Angle: 2.36, Altitude: 0.7, HighlightOpacity: 0.8, ShadowOpacity: 0.5},
		},
	}
}

// panel is a rounded card near the bottom with a gradient, inner shadow, stroke
func panel() canvas.Node {
	rr := demo.RoundRect(80, 560, W-80, 740, 28)
	return &canvas.Layer{
		Content: demo.Solid(W, H, color.NRGBA{40, 42, 60, 255}),
		Opacity: 1,
		Mask:    mask.NewVectorMask(rr, W, H, path.NonZero),
		Effects: []effects.Effect{
			&effects.DropShadow{Color: color.Black, Opacity: 0.5, Angle: 1.57, Distance: 10, BlurRadius: 20},
			&effects.GradientOverlay{Gradient: gradient.New(gradient.Linear, gradient.Pad,
				path.Point{X: 0, Y: 560}, path.Point{X: 0, Y: 740},
				[]gradient.Stop{
					{Pos: 0, Color: color.NRGBA{70, 80, 120, 255}, Opacity: 1},
					{Pos: 1, Color: color.NRGBA{40, 42, 70, 255}, Opacity: 1},
				}), Opacity: 1},
			&effects.InnerShadow{Color: color.Black, Opacity: 0.5, Angle: 1.57, Distance: 3, BlurRadius: 6},
			&effects.Stroke{Width: 2, Color: color.NRGBA{255, 255, 255, 255}, Alignment: effects.StrokeInside, Opacity: 0.4},
		},
	}
}
