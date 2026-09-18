package effects_test

import (
	"fmt"
	"image/color"
	"math"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/raster"
)

// An effect is a parameter struct, not an imperative call. Render hands back
// the pixels plus instructions on how to place them, and the caller (normally
// canvas) does the compositing
func ExampleDropShadow() {
	layer := solidSquare(64, 64, 20, 20, 24, 24)

	shadow := &effects.DropShadow{
		Color:      color.Black,
		Opacity:    0.75,
		Angle:      math.Pi / 4, // down and to the right
		Distance:   6,
		BlurRadius: 4,
		Mode:       blend.Multiply,
	}

	out := shadow.Render(layer)
	fmt.Printf("contributions: %d\n", len(out))
	fmt.Printf("behind the layer: %v\n", out[0].Behind)
	fmt.Printf("mode: %v, opacity: %.2f\n", out[0].Mode, out[0].Opacity)
	// Output:
	// contributions: 1
	// behind the layer: true
	// mode: Multiply, opacity: 0.75
}

// Bevel is the one effect returning two contributions, a highlight and a
// shadow, because they need different blend modes
func ExampleBevelEmboss() {
	layer := solidSquare(64, 64, 16, 16, 32, 32)

	bevel := &effects.BevelEmboss{
		Style: effects.BevelInner,
		Depth: 2,
		Size:  4,
		Angle: math.Pi / 4,
	}

	out := bevel.Render(layer)
	for _, r := range out {
		fmt.Printf("mode %v, behind %v\n", r.Mode, r.Behind)
	}
	// Output:
	// mode Screen, behind false
	// mode Multiply, behind false
}

// Effects render in Photoshop's fixed stacking order no matter what order the
// caller lists them, so the same set always produces the same image. Sort
// returns the canonical order without mutating the input
func ExampleSort() {
	listed := []effects.Effect{
		&effects.BevelEmboss{},
		&effects.DropShadow{},
		&effects.Stroke{},
		&effects.OuterGlow{},
	}

	for _, e := range effects.Sort(listed) {
		fmt.Printf("%T\n", e)
	}
	// Output:
	// *effects.DropShadow
	// *effects.OuterGlow
	// *effects.Stroke
	// *effects.BevelEmboss
}

// Stroke derives its band from the layer's alpha edge by morphology, so it
// works on any layer content, not just shapes that came from a path
func ExampleStroke() {
	layer := solidSquare(64, 64, 20, 20, 24, 24)

	for _, a := range []struct {
		name  string
		align effects.StrokeAlignment
	}{
		{"outside", effects.StrokeOutside},
		{"inside", effects.StrokeInside},
		{"center", effects.StrokeCenter},
	} {
		s := &effects.Stroke{Width: 4, Color: color.White, Alignment: a.align}
		out := s.Render(layer)

		// Sample just outside the square's left edge
		_, _, _, outer := out[0].Pixels.At(18, 32)
		// And just inside it
		_, _, _, inner := out[0].Pixels.At(21, 32)
		fmt.Printf("%-7s outside-edge %.1f inside-edge %.1f\n", a.name, outer, inner)
	}
	// Output:
	// outside outside-edge 1.0 inside-edge 0.0
	// inside  outside-edge 0.0 inside-edge 1.0
	// center  outside-edge 1.0 inside-edge 1.0
}

// Shadows and glows default to the blend mode Photoshop uses, so the zero value
// of Mode is the sensible one rather than Normal
func ExampleOuterGlow() {
	layer := solidSquare(32, 32, 8, 8, 16, 16)

	glow := &effects.OuterGlow{Color: color.White, BlurRadius: 3, Spread: 1}
	out := glow.Render(layer)

	fmt.Printf("default mode: %v\n", out[0].Mode)
	fmt.Printf("renders behind: %v\n", out[0].Behind)
	// Output:
	// default mode: Screen
	// renders behind: true
}

// solidSquare returns a buffer with an opaque white rectangle in it
func solidSquare(w, h, x, y, sw, sh int) *raster.Buffer {
	b := raster.MustNewBuffer(w, h)
	for yy := y; yy < y+sh; yy++ {
		for xx := x; xx < x+sw; xx++ {
			b.Set(xx, yy, 1, 1, 1, 1)
		}
	}
	return b
}
