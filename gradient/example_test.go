package gradient_test

import (
	"fmt"
	"image/color"

	"github.com/odevine/impasto/gradient"
	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

// A Gradient is immutable once built. New sorts the stops and precomputes the
// geometry, so sampling is cheap enough to call per pixel.
//
// Sampling happens at pixel centers, so pixel 0 of a 100-wide gradient sits at
// x=0.5 and lands a half-pixel short of the first stop. The end colors are
// approached, not reached
func ExampleNew() {
	g := gradient.New(gradient.Linear, gradient.Pad,
		path.Point{X: 0, Y: 0}, path.Point{X: 100, Y: 0},
		[]gradient.Stop{
			{Pos: 0, Color: color.NRGBA{R: 255, A: 255}, Opacity: 1},
			{Pos: 1, Color: color.NRGBA{B: 255, A: 255}, Opacity: 1},
		})

	for _, x := range []int{0, 50, 99} {
		c := g.ColorAt(x, 0)
		fmt.Printf("x=%-2d r=%.2f b=%.2f\n", x, c[0], c[2])
	}
	// Output:
	// x=0  r=0.99 b=0.00
	// x=50 r=0.21 b=0.22
	// x=99 r=0.00 b=0.99
}

// Opacity is per stop and independent of the stop color's own alpha, which is
// ignored. This is how a fade-to-transparent is expressed
func ExampleStop() {
	g := gradient.New(gradient.Linear, gradient.Pad,
		path.Point{X: 0, Y: 0}, path.Point{X: 10, Y: 0},
		[]gradient.Stop{
			{Pos: 0, Color: color.White, Opacity: 1},
			{Pos: 1, Color: color.White, Opacity: 0},
		})

	fmt.Printf("first pixel %.2f, last pixel %.2f\n", g.ColorAt(0, 0)[3], g.ColorAt(9, 0)[3])
	// Output: first pixel 0.95, last pixel 0.05
}

// Spread decides what happens outside the [0,1] parameter range. Pad clamps to
// the end stops, Repeat tiles, Reflect mirrors on each repeat.
//
// Sampled here at 1.55 gradient-lengths along the axis: Pad holds white, Repeat
// wraps to 0.55, and Reflect mirrors to 0.45. The printed values are linear
// light, which is why they look lower than the sRGB fractions
func ExampleSpread() {
	stops := []gradient.Stop{
		{Pos: 0, Color: color.Black, Opacity: 1},
		{Pos: 1, Color: color.White, Opacity: 1},
	}
	p0, p1 := path.Point{X: 0, Y: 0}, path.Point{X: 10, Y: 0}

	for _, s := range []struct {
		name   string
		spread gradient.Spread
	}{
		{"Pad", gradient.Pad},
		{"Repeat", gradient.Repeat},
		{"Reflect", gradient.Reflect},
	} {
		g := gradient.New(gradient.Linear, s.spread, p0, p1, stops)
		fmt.Printf("%-7s %.2f\n", s.name, g.ColorAt(15, 0)[0])
	}
	// Output:
	// Pad     1.00
	// Repeat  0.26
	// Reflect 0.17
}

// Render overwrites a whole buffer with the gradient field, ignoring whatever
// was there. Compositing and masking are the caller's job
func ExampleGradient_Render() {
	buf := raster.MustNewBuffer(64, 64)
	g := gradient.New(gradient.Radial, gradient.Pad,
		path.Point{X: 32, Y: 32}, path.Point{X: 64, Y: 32},
		[]gradient.Stop{
			{Pos: 0, Color: color.White, Opacity: 1},
			{Pos: 1, Color: color.Black, Opacity: 1},
		})

	g.Render(buf)

	fmt.Printf("center %.2f, edge %.2f\n", buf.Pix[(32*64+32)*4], buf.Pix[(32*64+63)*4])
	// Output: center 0.95, edge 0.00
}

// A Gradient satisfies path.Paint, so it can fill a shape directly without an
// intermediate buffer
func ExampleGradient_ColorAt() {
	dst := raster.MustNewBuffer(100, 100)
	circle := path.New().Ellipse(50, 50, 40, 40)

	g := gradient.New(gradient.Angle, gradient.Pad,
		path.Point{X: 50, Y: 50}, path.Point{X: 100, Y: 50},
		[]gradient.Stop{
			{Pos: 0, Color: color.NRGBA{R: 255, A: 255}, Opacity: 1},
			{Pos: 1, Color: color.NRGBA{G: 255, A: 255}, Opacity: 1},
		})

	path.Fill(dst, circle, path.NonZero, g)

	_, _, _, inside := dst.At(50, 50)
	_, _, _, outside := dst.At(2, 2)
	fmt.Printf("inside the circle %.2f, outside %.2f\n", inside, outside)
	// Output: inside the circle 1.00, outside 0.00
}
