package blend

import (
	"image"
	"image/color"
	"slices"
	"testing"

	"github.com/odevine/impasto/internal/testimg"
	"github.com/odevine/impasto/raster"
)

func TestIndexListsTheVisibleRuns(t *testing.T) {
	img := testimg.Sparse(120, 12, minGap)
	ix := Index(img)
	row := func(y int) []int32 { return ix.runs[2*ix.rows[y] : 2*ix.rows[y+1]] }
	for y, want := range map[int][]int32{
		0:  nil,
		1:  {0, 120},
		2:  {3, 10 + int32(minGap) + 9},                         // a gap just under the limit is merged
		3:  {3, 10, 10 + int32(minGap), 10 + int32(minGap) + 9}, // a gap at the limit is not
		4:  {0, 5, 115, 120},
		5:  {40, 41, 119, 120},
		6:  nil,
		10: {0, 5, 115, 120},
	} {
		if got := row(y); !slices.Equal(got, want) {
			t.Errorf("row %d runs = %v, want %v", y, got, want)
		}
	}
}

func TestIndexReadsASubImageAtItsOwnBounds(t *testing.T) {
	full := testimg.Sparse(120, 12, minGap)
	sub := full.SubImage(image.Rect(10, 1, 110, 5)).(*image.NRGBA)
	ix := Index(sub)
	if len(ix.rows) != 5 {
		t.Fatalf("rows = %d entries, want 5 for 4 rows", len(ix.rows))
	}
	// Row 1 of the parent is full, so its run is the sub-image's whole width
	if got := ix.runs[2*ix.rows[0] : 2*ix.rows[1]]; !slices.Equal(got, []int32{0, 100}) {
		t.Errorf("first row runs = %v, want [0 100]", got)
	}
}

func TestCompositeIndexedMatchesTheBufferPath(t *testing.T) {
	const w, h = 150, 40
	src := testimg.Sparse(131, 29, minGap)
	ix := Index(src)
	buf, err := raster.FromImage(src)
	if err != nil {
		t.Fatal(err)
	}
	origins := []image.Point{{0, 0}, {7, 5}, {-20, -3}, {100, 20}, {-200, 0}, {300, 300}}
	for m := Normal; m < numModes; m++ {
		for _, opacity := range []float32{1, 0.6} {
			for _, at := range origins {
				want, got, plain := backdrop(w, h), backdrop(w, h), backdrop(w, h)
				CompositeRect(want, buf, at, m, opacity)
				CompositeIndexed(got, ix, at, m, opacity)
				CompositeNRGBA(plain, src, at, m, opacity)
				sameBits(t, got, want, m.String())
				sameBits(t, plain, want, m.String())
			}
		}
	}
}

func TestCompositeIndexedReadsASubImageWhereItSits(t *testing.T) {
	full := testimg.Sparse(120, 24, minGap)
	sub := full.SubImage(image.Rect(7, 3, 100, 20)).(*image.NRGBA)
	buf, _ := raster.FromImage(sub)
	ix := Index(sub)
	for _, m := range []Mode{Normal, Multiply} {
		want, got := backdrop(130, 30), backdrop(130, 30)
		CompositeRect(want, buf, image.Pt(11, 4), m, 0.8)
		CompositeIndexed(got, ix, image.Pt(11, 4), m, 0.8)
		sameBits(t, got, want, m.String())
	}
}

func TestCompositeIndexedSkipsAnImageWithNothingVisible(t *testing.T) {
	ix := Index(image.NewNRGBA(image.Rect(0, 0, 50, 50)))
	if len(ix.runs) != 0 {
		t.Fatalf("a transparent image has %d runs", len(ix.runs)/2)
	}
	want, got := backdrop(60, 60), backdrop(60, 60)
	CompositeIndexed(got, ix, image.Pt(3, 3), Normal, 1)
	sameBits(t, got, want, "transparent image")
}

func TestCompositeIndexedIgnoresNonPositiveOpacityAndClampsAboveOne(t *testing.T) {
	src := testimg.Sparse(60, 30, minGap)
	ix := Index(src)
	want, got := backdrop(70, 40), backdrop(70, 40)
	for _, o := range []float32{0, -1} {
		CompositeIndexed(got, ix, image.Point{}, Normal, o)
		sameBits(t, got, want, "non-positive opacity")
	}
	CompositeNRGBA(want, src, image.Point{}, Normal, 1)
	CompositeIndexed(got, ix, image.Point{}, Normal, 4)
	sameBits(t, got, want, "opacity above one")
}

// An index of a fully opaque image holds one run per row, and of a transparent one
// none
func TestIndexOfASolidImageIsOneRunPerRow(t *testing.T) {
	ix := Index(testimg.Solid(50, 20, 255))
	if len(ix.runs) != 2*20 {
		t.Errorf("runs = %d, want %d", len(ix.runs)/2, 20)
	}
}

// Both blend packages keep their own table of 8-bit sRGB codes, so every code
// must come out as raster.FromImage reads it
func TestSRGBTableMatchesIngest(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 256, 1))
	for c := 0; c < 256; c++ {
		img.SetNRGBA(c, 0, color.NRGBA{R: uint8(c), G: uint8(255 - c), B: uint8(c), A: 255})
	}
	buf, err := raster.FromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	for c := 0; c < 256; c++ {
		r, g, b, a := buf.At(c, 0)
		if r != srgb8ToLinear[c] || g != srgb8ToLinear[255-c] || b != srgb8ToLinear[c] || a != 1 {
			t.Fatalf("code %d reads as %v %v %v %v, table has %v %v", c, r, g, b, a, srgb8ToLinear[c], srgb8ToLinear[255-c])
		}
	}
}

func BenchmarkCompositeHollowFrame(b *testing.B) {
	frame := testimg.Hollow(2048, 2048, 60)
	ix := Index(frame)
	dst := raster.MustNewBuffer(2048, 2048)
	b.Run("nrgba", func(b *testing.B) {
		for b.Loop() {
			CompositeNRGBA(dst, frame, image.Point{}, Normal, 1)
		}
	})
	b.Run("indexed", func(b *testing.B) {
		for b.Loop() {
			CompositeIndexed(dst, ix, image.Point{}, Normal, 1)
		}
	})
}

func BenchmarkIndex(b *testing.B) {
	frame := testimg.Hollow(3264, 4440, 100)
	b.SetBytes(int64(len(frame.Pix)))
	for b.Loop() {
		Index(frame)
	}
}
