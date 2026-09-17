package canvas_test

import (
	"fmt"
	"image/color"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/canvas"
	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

// Example composes a small document with a masked layer, a drop shadow, and a
// stroke, then flattens it. This mirrors the shape of a real caller: build
// document-sized layers, attach masks and effects, and render to an image
func Example() {
	const w, h = 200, 120

	bg := raster.MustNewBuffer(w, h)
	for i := 0; i < len(bg.Pix); i += 4 {
		bg.Pix[i], bg.Pix[i+1], bg.Pix[i+2], bg.Pix[i+3] = 0.1, 0.12, 0.15, 1
	}

	logo := raster.MustNewBuffer(w, h)
	for i := 0; i < len(logo.Pix); i += 4 {
		logo.Pix[i], logo.Pix[i+1], logo.Pix[i+2], logo.Pix[i+3] = 0.9, 0.3, 0.2, 1
	}
	rounded := path.New().Ellipse(w/2, h/2, 60, 40)

	doc := &canvas.Document{
		Width: w, Height: h,
		Root: canvas.Group{
			PassThrough: true, Opacity: 1,
			Layers: []canvas.Node{
				&canvas.Layer{Content: bg, Opacity: 1, Mode: blend.Normal},
				&canvas.Layer{
					Content: logo,
					Opacity: 1,
					Mode:    blend.Normal,
					Mask:    mask.NewVectorMask(rounded, w, h, path.NonZero),
					Effects: []effects.Effect{
						&effects.DropShadow{
							Color: color.Black, Opacity: 0.75,
							Angle: 2.36, Distance: 12, BlurRadius: 18,
							Mode: blend.Multiply,
						},
						&effects.Stroke{
							Width: 4, Color: color.White,
							Alignment: effects.StrokeOutside,
						},
					},
				},
			},
		},
	}

	out := canvas.MustRender(doc)
	img := out.ToImage(8)
	fmt.Printf("rendered %dx%d\n", img.Bounds().Dx(), img.Bounds().Dy())
	// Output: rendered 200x120
}
