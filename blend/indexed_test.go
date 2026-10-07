package blend

import (
	"image"
	"image/color"
	"slices"
	"testing"

	"github.com/odevine/impasto/raster"
)

// sparse is an image whose rows hold stretches of visible pixels separated by
// gaps that are shorter than, equal to and longer than the merge distance, plus
// rows that are empty and rows that are full
func sparse(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	set := func(x, y int, a uint8) {
		img.SetNRGBA(x, y, color.NRGBA{R: uint8(x*13 + y), G: uint8(y * 7), B: uint8(x + 40), A: a})
	}
	for y := 0; y < h; y++ {
		switch y % 6 {
		case 0: // empty
		case 1: // full and opaque
			for x := 0; x < w; x++ {
				set(x, y, 255)
			}
		case 2: // two strips, a gap of minGap-1 between them
			for x := 3; x < 10; x++ {
				set(x, y, 200)
			}
			for x := 10 + minGap - 1; x < 10+minGap+9; x++ {
				set(x, y, 90)
			}
		case 3: // two strips, a gap of exactly minGap between them
			for x := 3; x < 10; x++ {
				set(x, y, 255)
			}
			for x := 10 + minGap; x < 10+minGap+9; x++ {
				set(x, y, 255)
			}
		case 4: // a ring: a strip at each end and nothing between
			for x := 0; x < 5; x++ {
				set(x, y, 255)
				set(w-1-x, y, 128)
			}
		case 5: // lone pixels, one with the faintest alpha
			set(w/3, y, 1)
			set(w-1, y, 255)
		}
	}
	return img
}

func TestIndexListsTheVisibleRuns(t *testing.T) {
	img := sparse(120, 12)
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
	if ix.Runs() == 0 {
		t.Error("Runs is zero")
	}
}

func TestIndexReadsASubImageAtItsOwnBounds(t *testing.T) {
	full := sparse(120, 12)
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
	src := sparse(131, 29)
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
	full := sparse(120, 24)
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
	if ix.Runs() != 0 {
		t.Fatalf("Runs = %d for a transparent image", ix.Runs())
	}
	want, got := backdrop(60, 60), backdrop(60, 60)
	CompositeIndexed(got, ix, image.Pt(3, 3), Normal, 1)
	sameBits(t, got, want, "transparent image")
}

func TestCompositeIndexedIgnoresNonPositiveOpacity(t *testing.T) {
	ix := Index(sparse(60, 30))
	want, got := backdrop(70, 40), backdrop(70, 40)
	CompositeIndexed(got, ix, image.Point{}, Normal, 0)
	sameBits(t, got, want, "opacity 0")
}

// hollowImage is a frame of the given thickness, transparent inside
func hollowImage(w, h, thickness int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < thickness || y < thickness || x >= w-thickness || y >= h-thickness {
				img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
			}
		}
	}
	return img
}

func BenchmarkCompositeHollowFrame(b *testing.B) {
	frame := hollowImage(2048, 2048, 60)
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
	frame := hollowImage(3264, 4440, 100)
	b.SetBytes(int64(len(frame.Pix)))
	for b.Loop() {
		Index(frame)
	}
}
