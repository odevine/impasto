package blend_test

import (
	"fmt"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/raster"
)

// BlendPixel is the whole package in one call: a premultiplied linear source
// over a premultiplied linear backdrop. Multiply always darkens, so a mid-gray
// source over a mid-gray backdrop lands darker than either
func ExampleBlendPixel() {
	backdrop := [4]float32{0.5, 0.5, 0.5, 1}
	source := [4]float32{0.5, 0.5, 0.5, 1}

	out := blend.BlendPixel(backdrop, source, blend.Multiply)
	fmt.Printf("multiply: %.2f\n", out[0])

	out = blend.BlendPixel(backdrop, source, blend.Screen)
	fmt.Printf("screen:   %.2f\n", out[0])
	// Output:
	// multiply: 0.25
	// screen:   0.75
}

// A fully transparent source leaves the backdrop untouched whatever the mode,
// so there is no need to special-case empty regions before calling
func ExampleBlendPixel_transparentSource() {
	backdrop := [4]float32{0.4, 0.2, 0.1, 1}
	source := [4]float32{0, 0, 0, 0}

	out := blend.BlendPixel(backdrop, source, blend.Difference)
	fmt.Printf("%.1f %.1f %.1f %.1f\n", out[0], out[1], out[2], out[3])
	// Output: 0.4 0.2 0.1 1.0
}

// Composite applies a mode across two whole buffers in place, scaled by an
// opacity in [0,1]. Both buffers must be the same size
func ExampleComposite() {
	dst := raster.MustNewBuffer(2, 1)
	src := raster.MustNewBuffer(2, 1)
	for i := 0; i < len(dst.Pix); i += 4 {
		dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = 0, 0, 0, 1
		src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3] = 1, 1, 1, 1
	}

	blend.Composite(dst, src, blend.Normal, 0.25)
	r, _, _, a := dst.At(0, 0)
	fmt.Printf("white over black at 25%%: %.2f (alpha %.2f)\n", r, a)
	// Output: white over black at 25%: 0.25 (alpha 1.00)
}

// Mode values are a fixed iota ordering that is part of the API, so they are
// safe to persist. String and Valid cover display and input checking
func ExampleMode() {
	fmt.Println(blend.Normal, blend.SoftLight, blend.Luminosity)
	fmt.Println(blend.Mode(99).Valid(), blend.Mode(99))
	// Output:
	// Normal SoftLight Luminosity
	// false Mode(invalid)
}
