package blur_test

import (
	"fmt"

	"github.com/odevine/impasto/blur"
	"github.com/odevine/impasto/raster"
)

// Gaussian blurs in place, approximating a true Gaussian with three box passes.
// It works on premultiplied channels, so color and alpha spread together and a
// blurred edge stays free of fringing
func ExampleGaussian() {
	buf := raster.MustNewBuffer(64, 64)
	// A single opaque white pixel at the center
	buf.Set(32, 32, 1, 1, 1, 1)

	before := sumAlpha(buf)
	blur.Gaussian(buf, 4)
	after := sumAlpha(buf)

	_, _, _, center := buf.At(32, 32)
	_, _, _, near := buf.At(36, 32)
	fmt.Printf("center %.4f, 4px away %.4f\n", center, near)

	// A blur redistributes energy rather than creating or destroying it
	fmt.Printf("total alpha preserved: %v\n", after > before*0.99 && after < before*1.01)
	// Output:
	// center 0.0095, 4px away 0.0062
	// total alpha preserved: true
}

// A non-positive sigma is a no-op, so a caller can pass a configured radius
// through without guarding it
func ExampleGaussian_zeroSigma() {
	buf := raster.MustNewBuffer(8, 8)
	buf.Set(4, 4, 1, 1, 1, 1)

	blur.Gaussian(buf, 0)

	_, _, _, a := buf.At(4, 4)
	fmt.Printf("untouched: %.1f\n", a)
	// Output: untouched: 1.0
}

// BoxBlur exposes the cheaper single-pass primitive for callers who do not need
// Gaussian falloff. Radius is in pixels, not a standard deviation
func ExampleBoxBlur() {
	buf := raster.MustNewBuffer(16, 1)
	buf.Set(8, 0, 1, 1, 1, 1)

	blur.BoxBlur(buf, 2)

	// One pixel spread evenly over a 2*2+1 window
	for x := 6; x <= 10; x++ {
		_, _, _, a := buf.At(x, 0)
		fmt.Printf("%.2f ", a)
	}
	fmt.Println()
	// Output: 0.20 0.20 0.20 0.20 0.20
}

func sumAlpha(b *raster.Buffer) float32 {
	var total float32
	for i := 3; i < len(b.Pix); i += 4 {
		total += b.Pix[i]
	}
	return total
}
