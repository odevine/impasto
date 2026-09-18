package mask_test

import (
	"fmt"

	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

// Apply multiplies coverage into a premultiplied buffer in place. Scaling all
// four channels by the same factor keeps the pixel premultiplied, so the result
// is still valid input for compositing
func ExampleApply() {
	buf := raster.MustNewBuffer(4, 1)
	for i := 0; i < len(buf.Pix); i += 4 {
		buf.Pix[i], buf.Pix[i+1], buf.Pix[i+2], buf.Pix[i+3] = 1, 1, 1, 1
	}

	// A one-row ramp from fully hidden to fully revealed
	m := mask.NewRasterMask([]float32{0, 0.25, 0.5, 1}, 4, 1)
	mask.Apply(buf, m)

	for x := 0; x < 4; x++ {
		r, _, _, a := buf.At(x, 0)
		fmt.Printf("x=%d color=%.2f alpha=%.2f\n", x, r, a)
	}
	// Output:
	// x=0 color=0.00 alpha=0.00
	// x=1 color=0.25 alpha=0.25
	// x=2 color=0.50 alpha=0.50
	// x=3 color=1.00 alpha=1.00
}

// A VectorMask rasterizes a path to coverage once, up front, so reusing the
// layer never re-rasterizes the same geometry
func ExampleNewVectorMask() {
	circle := path.New().Ellipse(10, 10, 8, 8)
	m := mask.NewVectorMask(circle, 20, 20, path.NonZero)

	fmt.Printf("center: %.2f\n", m.Coverage(10, 10))
	fmt.Printf("corner: %.2f\n", m.Coverage(0, 0))
	// Output:
	// center: 1.00
	// corner: 0.00
}

// Multi combines masks by multiplying their coverage, which is what a layer
// carrying both a raster and a vector mask does
func ExampleMulti() {
	left := mask.FuncMask(func(x, y int) float32 {
		if x < 2 {
			return 1
		}
		return 0
	})
	top := mask.FuncMask(func(x, y int) float32 {
		if y < 2 {
			return 1
		}
		return 0
	})

	both := mask.Multi{left, top}
	fmt.Printf("top-left     %.0f\n", both.Coverage(0, 0))
	fmt.Printf("top-right    %.0f\n", both.Coverage(3, 0))
	fmt.Printf("bottom-left  %.0f\n", both.Coverage(0, 3))
	// Output:
	// top-left     1
	// top-right    0
	// bottom-left  0
}

// FuncMask adapts a plain function to the Mask interface, for coverage that is
// cheaper to compute than to store
func ExampleFuncMask() {
	// A horizontal fade across a 100px document
	fade := mask.FuncMask(func(x, y int) float32 {
		return float32(x) / 99
	})

	fmt.Printf("%.2f %.2f %.2f\n", fade.Coverage(0, 0), fade.Coverage(49, 0), fade.Coverage(99, 0))
	// Output: 0.00 0.49 1.00
}
