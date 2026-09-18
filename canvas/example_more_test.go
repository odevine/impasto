package canvas_test

import (
	"fmt"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/canvas"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/raster"
)

// Layer content is document-sized, so a smaller image is placed into a
// document-sized buffer first. This is what keeps the model free of per-layer
// offsets and lets an effect reach anywhere on the canvas
func ExamplePlace() {
	logo := raster.MustNewBuffer(4, 4)
	for i := 0; i < len(logo.Pix); i += 4 {
		logo.Pix[i], logo.Pix[i+1], logo.Pix[i+2], logo.Pix[i+3] = 1, 0, 0, 1
	}

	content := canvas.Place(32, 32, logo, 10, 10)

	_, _, _, atOrigin := content.At(0, 0)
	_, _, _, atLogo := content.At(11, 11)
	fmt.Printf("empty elsewhere %.1f, placed at (10,10) %.1f\n", atOrigin, atLogo)
	fmt.Printf("content is document-sized: %dx%d\n", content.Width, content.Height)
	// Output:
	// empty elsewhere 0.0, placed at (10,10) 1.0
	// content is document-sized: 32x32
}

// A pass-through group lets its children blend with whatever is beneath the
// group. An isolated group composites its children onto a transparent buffer
// first and then blends that result as a unit, so the children never see the
// backdrop.
//
// The difference shows whenever a child uses a non-Normal mode. A group is
// isolated whenever PassThrough is false, or it carries a mask, a non-Normal
// mode, or an opacity below one
func ExampleGroup() {
	render := func(passThrough bool) float32 {
		doc := &canvas.Document{
			Width: 8, Height: 8,
			Root: canvas.Group{
				PassThrough: true, Opacity: 1,
				Layers: []canvas.Node{
					// A mid-gray backdrop beneath the group
					&canvas.Layer{Content: fill(8, 8, 0.5, 0.5, 0.5, 1), Opacity: 1},
					&canvas.Group{
						PassThrough: passThrough,
						Opacity:     1,
						Layers: []canvas.Node{
							&canvas.Layer{
								Content: fill(8, 8, 0.5, 0.5, 0.5, 1),
								Opacity: 1,
								Mode:    blend.Multiply,
							},
						},
					},
				},
			},
		}
		r, _, _, _ := canvas.MustRender(doc).At(4, 4)
		return r
	}

	// Pass-through: the child multiplies against the gray backdrop, 0.5*0.5.
	// Isolated: the child multiplies against transparency, so it keeps its own
	// value and the group lands on the backdrop unchanged
	fmt.Printf("pass-through: %.2f\n", render(true))
	fmt.Printf("isolated:     %.2f\n", render(false))
	// Output:
	// pass-through: 0.25
	// isolated:     0.50
}

// ClipToBelow confines a layer to the alpha of the layer directly beneath it,
// the equivalent of a Photoshop clipping mask. The base is whichever
// non-clipped layer came last
func ExampleLayer_ClipToBelow() {
	base := canvas.Place(16, 16, fill(8, 8, 0, 0, 1, 1), 0, 0)
	texture := fill(16, 16, 1, 0, 0, 1)

	doc := &canvas.Document{
		Width: 16, Height: 16,
		Root: canvas.Group{
			PassThrough: true, Opacity: 1,
			Layers: []canvas.Node{
				&canvas.Layer{Content: base, Opacity: 1},
				&canvas.Layer{Content: texture, Opacity: 1, ClipToBelow: true},
			},
		},
	}

	out := canvas.MustRender(doc)
	r, _, _, _ := out.At(4, 4)   // over the base
	_, _, _, a := out.At(12, 12) // past the base
	fmt.Printf("over the base: red %.1f\n", r)
	fmt.Printf("past the base: alpha %.1f\n", a)
	// Output:
	// over the base: red 1.0
	// past the base: alpha 0.0
}

// A mask scales a layer's alpha before compositing. Multiple masks on one layer
// multiply together via mask.Multi
func ExampleLayer_mask() {
	doc := &canvas.Document{
		Width: 8, Height: 8,
		Root: canvas.Group{
			PassThrough: true, Opacity: 1,
			Layers: []canvas.Node{
				&canvas.Layer{
					Content: fill(8, 8, 1, 1, 1, 1),
					Opacity: 1,
					Mode:    blend.Normal,
					Mask: mask.FuncMask(func(x, y int) float32 {
						if x < 4 {
							return 1
						}
						return 0
					}),
				},
			},
		},
	}

	out := canvas.MustRender(doc)
	_, _, _, left := out.At(1, 4)
	_, _, _, right := out.At(6, 4)
	fmt.Printf("revealed %.1f, hidden %.1f\n", left, right)
	// Output: revealed 1.0, hidden 0.0
}

// Render reports an error only for invalid document dimensions. Everything else
// about a document is renderable by construction
func ExampleRender() {
	_, err := canvas.Render(&canvas.Document{Width: 0, Height: 10})
	fmt.Println(err)
	// Output: raster: invalid buffer dimensions: 0x10 must be positive
}

// fill returns a document-sized buffer of one premultiplied linear color
func fill(w, h int, r, g, b, a float32) *raster.Buffer {
	buf := raster.MustNewBuffer(w, h)
	for i := 0; i < len(buf.Pix); i += 4 {
		buf.Pix[i], buf.Pix[i+1], buf.Pix[i+2], buf.Pix[i+3] = r, g, b, a
	}
	return buf
}
