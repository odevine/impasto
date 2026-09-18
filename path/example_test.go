package path_test

import (
	"fmt"
	"image/color"

	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

// Path builders chain, and curves are stored exactly rather than flattened on
// construction, so one path renders cleanly at any scale
func ExamplePath() {
	p := path.New().
		MoveTo(10, 10).
		LineTo(90, 10).
		CubicTo(90, 50, 50, 90, 10, 90).
		Close()

	minX, minY, maxX, maxY := p.Bounds()
	fmt.Printf("bounds: (%.0f,%.0f)-(%.0f,%.0f)\n", minX, minY, maxX, maxY)
	fmt.Println("empty:", p.Empty())
	// Output:
	// bounds: (10,10)-(90,90)
	// empty: false
}

// Fill rasterizes a path and composites a paint through its coverage. Coverage
// is anti-aliased at float32 precision, so edges carry no 8-bit banding
func ExampleFill() {
	dst := raster.MustNewBuffer(20, 20)
	circle := path.New().Ellipse(10, 10, 6, 6)

	path.Fill(dst, circle, path.NonZero, path.NewFlatColor(1, 0, 0, 1))

	_, _, _, inside := dst.At(10, 10)
	_, _, _, outside := dst.At(0, 0)
	fmt.Printf("inside %.2f, outside %.2f\n", inside, outside)
	// Output: inside 1.00, outside 0.00
}

// The two fill rules differ only where a path overlaps itself. A five-pointed
// star drawn as one self-crossing subpath has a solid center under NonZero and
// a hollow one under EvenOdd
func ExampleFillRule() {
	star := path.New()
	// Connect every second vertex of a pentagon, so the edges cross
	pts := [5][2]float32{{50, 5}, {88, 95}, {5, 37}, {95, 37}, {12, 95}}
	star.MoveTo(pts[0][0], pts[0][1])
	for _, pt := range pts[1:] {
		star.LineTo(pt[0], pt[1])
	}
	star.Close()

	nonZero := star.Coverage(100, 100, path.NonZero, path.DefaultTolerance)
	evenOdd := star.Coverage(100, 100, path.EvenOdd, path.DefaultTolerance)

	center := 50*100 + 50
	fmt.Printf("center coverage: nonzero %.0f, evenodd %.0f\n", nonZero[center], evenOdd[center])
	// Output: center coverage: nonzero 1, evenodd 0
}

// Stroke turns a path into a fillable outline, which golang.org/x/image/vector
// does not cover. Fill the result with NonZero
func ExampleStroke() {
	line := path.New().MoveTo(10, 50).LineTo(90, 50)

	outline := path.Stroke(line, path.StrokeStyle{
		Width: 10,
		Cap:   path.CapRound,
		Join:  path.JoinRound,
	})

	minX, minY, maxX, maxY := outline.Bounds()
	// Round caps extend half the stroke width past each endpoint
	fmt.Printf("outline bounds: (%.0f,%.0f)-(%.0f,%.0f)\n", minX, minY, maxX, maxY)
	// Output: outline bounds: (5,45)-(95,55)
}

// StrokePath strokes and paints in one call, the common case
func ExampleStrokePath() {
	dst := raster.MustNewBuffer(100, 100)
	box := path.New().Rect(20, 20, 80, 80)

	path.StrokePath(dst, box, path.StrokeStyle{
		Width: 6,
		Join:  path.JoinMiter,
		Dash:  []float32{12, 6},
	}, path.FlatColorSRGB(color.NRGBA{R: 255, A: 255}))

	_, _, _, onEdge := dst.At(20, 22)
	_, _, _, inMiddle := dst.At(50, 50)
	fmt.Printf("on the edge %.2f, in the middle %.2f\n", onEdge, inMiddle)
	// Output: on the edge 1.00, in the middle 0.00
}

// FlatColorSRGB decodes a standard library color into the linear working space.
// Building a FlatColor by hand instead skips the decode, so its components are
// already linear
func ExampleFlatColorSRGB() {
	c := path.FlatColorSRGB(color.NRGBA{R: 128, G: 128, B: 128, A: 255})
	fmt.Printf("sRGB 128 -> linear %.4f\n", c[0])

	// Half-transparent white, premultiplied on construction
	h := path.NewFlatColor(1, 1, 1, 0.5)
	fmt.Printf("premultiplied: %.2f %.2f\n", h[0], h[3])
	// Output:
	// sRGB 128 -> linear 0.2159
	// premultiplied: 0.50 0.50
}
