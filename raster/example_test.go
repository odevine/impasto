package raster_test

import (
	"fmt"
	"image"
	"image/color"

	"github.com/odevine/impasto/raster"
)

// A Buffer stores premultiplied linear float32 RGBA. Set and At speak that
// representation directly, so a half-transparent red arrives with its color
// channels already scaled by alpha
func ExampleBuffer() {
	buf := raster.MustNewBuffer(4, 2)
	buf.Set(1, 0, 0.5, 0, 0, 0.5)

	r, g, b, a := buf.At(1, 0)
	fmt.Printf("stored:  %.2f %.2f %.2f %.2f\n", r, g, b, a)

	// Reads outside the buffer return transparent black instead of panicking,
	// which keeps sampling loops that run past an edge well defined
	r, g, b, a = buf.At(99, 99)
	fmt.Printf("outside: %.2f %.2f %.2f %.2f\n", r, g, b, a)
	// Output:
	// stored:  0.50 0.00 0.00 0.50
	// outside: 0.00 0.00 0.00 0.00
}

// NewBuffer reports bad dimensions as an error rather than panicking, so a
// decoder can reject a hostile image header before anything is allocated
func ExampleNewBuffer() {
	if _, err := raster.NewBuffer(0, 100); err != nil {
		fmt.Println(err)
	}
	if _, err := raster.NewBuffer(raster.MaxDimension+1, 1); err != nil {
		fmt.Println(err)
	}
	// Output:
	// raster: invalid buffer dimensions: 0x100 must be positive
	// raster: invalid buffer dimensions: 65537x1 exceeds max edge 65536
}

// FromImage decodes a standard library image into the working representation:
// sRGB to linear light, straight alpha to premultiplied
func ExampleFromImage() {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.Set(0, 0, color.NRGBA{R: 128, G: 128, B: 128, A: 255})

	buf, err := raster.FromImage(src)
	if err != nil {
		panic(err)
	}
	r, _, _, _ := buf.At(0, 0)

	// Mid-gray is not half brightness: sRGB encoding is not linear
	fmt.Printf("sRGB 128 -> linear %.4f\n", r)
	// Output: sRGB 128 -> linear 0.2159
}

// ToImage encodes back to a standard library image. Pass 8 for a dithered
// *image.NRGBA or 16 for an undithered *image.NRGBA64
func ExampleBuffer_ToImage() {
	buf := raster.MustNewBuffer(2, 2)
	for i := 0; i < len(buf.Pix); i += 4 {
		// Opaque mid-gray in linear light
		buf.Pix[i], buf.Pix[i+1], buf.Pix[i+2], buf.Pix[i+3] = 0.2159, 0.2159, 0.2159, 1
	}

	img8 := buf.ToImage(8)
	img16 := buf.ToImage(16)
	fmt.Printf("%T %v\n", img8, img8.Bounds())
	fmt.Printf("%T %v\n", img16, img16.Bounds())
	// Output:
	// *image.NRGBA (0,0)-(2,2)
	// *image.NRGBA64 (0,0)-(2,2)
}

// The sRGB transfer functions are exported for callers that need to convert a
// single value, such as when building a color constant by hand
func ExampleSRGBToLinear() {
	for _, v := range []float32{0, 0.25, 0.5, 1} {
		fmt.Printf("sRGB %.2f -> linear %.4f -> sRGB %.2f\n",
			v, raster.SRGBToLinear(v), raster.LinearToSRGB(raster.SRGBToLinear(v)))
	}
	// Output:
	// sRGB 0.00 -> linear 0.0000 -> sRGB 0.00
	// sRGB 0.25 -> linear 0.0509 -> sRGB 0.25
	// sRGB 0.50 -> linear 0.2140 -> sRGB 0.50
	// sRGB 1.00 -> linear 1.0000 -> sRGB 1.00
}
