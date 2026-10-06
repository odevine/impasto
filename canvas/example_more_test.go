package canvas_test

import (
	"errors"
	"fmt"
	"image"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/canvas"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/raster"
)

// Place copies a smaller image into a new document-sized buffer at a position,
// for callers who want a buffer the size of the document. A layer does not need
// one, see the Layer example for Origin
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

// A layer's content can be smaller than the document. Origin says where its
// top-left pixel sits, and the part of the content that falls outside the
// document is clipped
func ExampleLayer_origin() {
	badge := raster.MustNewBuffer(4, 4)
	for i := 0; i < len(badge.Pix); i += 4 {
		badge.Pix[i], badge.Pix[i+1], badge.Pix[i+2], badge.Pix[i+3] = 1, 0, 0, 1
	}

	doc := &canvas.Document{Width: 32, Height: 32, Root: canvas.Group{
		PassThrough: true,
		Layers: []canvas.Node{
			&canvas.Layer{Content: badge, Origin: image.Pt(10, 10)},
			&canvas.Layer{Content: badge, Origin: image.Pt(-2, -2)},
		},
	}}
	out := canvas.MustRender(doc)

	alpha := func(x, y int) float32 { _, _, _, a := out.At(x, y); return a }
	fmt.Printf("badge at (10,10): %.0f, one pixel before it: %.0f, last pixel: %.0f\n", alpha(10, 10), alpha(9, 9), alpha(13, 13))
	fmt.Printf("badge hanging off the corner: %.0f at (1,1), %.0f at (2,2)\n", alpha(1, 1), alpha(2, 2))
	// Output:
	// badge at (10,10): 1, one pixel before it: 0, last pixel: 1
	// badge hanging off the corner: 1 at (1,1), 0 at (2,2)
}

// A layer can build its content when it is composited, so a document of many
// large layers does not hold them all at once. Load returns the buffer and where
// it sits, and canvas drops it when the layer is done
func ExampleLayer_load() {
	loaded := 0
	frame := &canvas.Layer{Load: func() (*raster.Buffer, image.Point, error) {
		loaded++
		b := raster.MustNewBuffer(4, 4)
		for i := 0; i < len(b.Pix); i += 4 {
			b.Pix[i], b.Pix[i+1], b.Pix[i+2], b.Pix[i+3] = 0, 0, 1, 1
		}
		return b, image.Pt(6, 6), nil
	}}
	doc := &canvas.Document{Width: 16, Height: 16, Root: canvas.Group{
		PassThrough: true,
		Layers:      []canvas.Node{frame},
	}}

	out := canvas.MustRender(doc)
	_, _, _, inside := out.At(7, 7)
	_, _, _, outside := out.At(2, 2)
	fmt.Printf("drawn at (7,7): %.0f, at (2,2): %.0f, Load calls: %d\n", inside, outside, loaded)
	// Output:
	// drawn at (7,7): 1, at (2,2): 0, Load calls: 1
}

// The first Load error stops the render. The error wraps the original and names
// the layer's position in the stack
func ExampleRender_loadError() {
	errMissing := errors.New("frame.png: no such file")
	doc := &canvas.Document{Width: 16, Height: 16, Root: canvas.Group{
		PassThrough: true,
		Layers: []canvas.Node{
			&canvas.Layer{Content: raster.MustNewBuffer(16, 16)},
			&canvas.Layer{Load: func() (*raster.Buffer, image.Point, error) {
				return nil, image.Point{}, errMissing
			}},
		},
	}}

	out, err := canvas.Render(doc)
	fmt.Println(out == nil, errors.Is(err, errMissing))
	fmt.Println(err)
	// Output:
	// true true
	// canvas: layer 1: load: frame.png: no such file
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
