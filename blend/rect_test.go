package blend

import (
	"image"
	"testing"

	"github.com/odevine/impasto/raster"
)

func rectTestBuffer(w, h int, seed float32) *raster.Buffer {
	b := raster.MustNewBuffer(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := 0.2 + 0.8*float32((x*3+y*7)%10)/10
			b.Set(x, y, (0.1+seed)*a, 0.5*a, float32((x+y)%5)/5*a, a)
		}
	}
	return b
}

// place copies src into a transparent buffer the size of dst at origin, clipped
func place(dst, src *raster.Buffer, origin image.Point) *raster.Buffer {
	out := raster.MustNewBuffer(dst.Width, dst.Height)
	for y := 0; y < src.Height; y++ {
		for x := 0; x < src.Width; x++ {
			r, g, b, a := src.At(x, y)
			out.Set(x+origin.X, y+origin.Y, r, g, b, a)
		}
	}
	return out
}

// TestCompositeRectMatchesPlaced requires blending a small buffer at an origin to
// equal blending the same pixels placed into a buffer the size of the
// destination, for every mode and for origins inside, on the edges of, partly
// outside and fully outside the destination
func TestCompositeRectMatchesPlaced(t *testing.T) {
	dst := rectTestBuffer(40, 30, 0.3)
	src := rectTestBuffer(12, 9, 0)
	origins := []image.Point{
		{0, 0}, {10, 8}, {28, 21}, {-5, -4}, {35, 25}, {-20, 5}, {45, 5}, {5, 40}, {-12, -9}, {40, 30},
	}
	for m := Normal; m < numModes; m++ {
		for _, o := range origins {
			want := dst.Clone()
			Composite(want, place(dst, src, o), m, 0.7)
			got := dst.Clone()
			CompositeRect(got, src, o, m, 0.7)
			for i := range want.Pix {
				if want.Pix[i] != got.Pix[i] {
					t.Fatalf("%v at %v: pixel %d is %v, want %v", m, o, i, got.Pix[i], want.Pix[i])
				}
			}
		}
	}
}

// TestCompositeRectTouchesOnlyItsRect checks pixels outside the source's rect are
// left exactly as they were
func TestCompositeRectTouchesOnlyItsRect(t *testing.T) {
	dst := rectTestBuffer(40, 30, 0.3)
	src := rectTestBuffer(12, 9, 0)
	o := image.Pt(10, 8)
	got := dst.Clone()
	CompositeRect(got, src, o, Overlay, 1)
	changed := 0
	for y := 0; y < dst.Height; y++ {
		for x := 0; x < dst.Width; x++ {
			r0, g0, b0, a0 := dst.At(x, y)
			r1, g1, b1, a1 := got.At(x, y)
			if r0 == r1 && g0 == g1 && b0 == b1 && a0 == a1 {
				continue
			}
			changed++
			if !image.Pt(x, y).In(image.Rect(o.X, o.Y, o.X+src.Width, o.Y+src.Height)) {
				t.Fatalf("pixel (%d,%d) outside the rect changed", x, y)
			}
		}
	}
	if changed == 0 {
		t.Fatal("nothing was composited")
	}
}

func TestCompositeRectIgnoresNonPositiveOpacity(t *testing.T) {
	dst := rectTestBuffer(10, 10, 0.3)
	got := dst.Clone()
	CompositeRect(got, rectTestBuffer(4, 4, 0), image.Pt(2, 2), Normal, 0)
	for i := range got.Pix {
		if got.Pix[i] != dst.Pix[i] {
			t.Fatalf("pixel %d changed", i)
		}
	}
}
