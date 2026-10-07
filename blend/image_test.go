package blend

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/odevine/impasto/raster"
)

// patterned is an image whose alpha takes the values the fast paths branch on,
// transparent, opaque and partial, spread across its pixels
func patterned(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	alphas := []uint8{0, 255, 128, 1, 254, 77, 0, 255, 200}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x*37 + y*5),
				G: uint8(y*23 + x),
				B: uint8(x*y + 9),
				A: alphas[(x*3+y*7)%len(alphas)],
			})
		}
	}
	return img
}

// backdrop is a buffer with a color and alpha that vary, so a blend has
// something to differ from
func backdrop(w, h int) *raster.Buffer {
	buf := raster.MustNewBuffer(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := 0.25 + 0.75*float32((x+y)%5)/4
			buf.Set(x, y, a*float32(x%7)/6, a*float32(y%5)/4, a*float32((x+y)%3)/2, a)
		}
	}
	return buf
}

func sameBits(t *testing.T, got, want *raster.Buffer, what string) {
	t.Helper()
	for i := range want.Pix {
		if math.Float32bits(got.Pix[i]) != math.Float32bits(want.Pix[i]) {
			t.Fatalf("%s: value %d (pixel %d, channel %d) = %v, want %v", what, i, i/4, i%4, got.Pix[i], want.Pix[i])
		}
	}
}

func TestCompositeNRGBAMatchesBuffer(t *testing.T) {
	const w, h = 48, 36
	src := patterned(31, 23)
	buf, err := raster.FromImage(src)
	if err != nil {
		t.Fatal(err)
	}
	origins := []image.Point{{0, 0}, {10, 7}, {-5, -3}, {30, 20}, {-40, 0}, {100, 100}}
	for m := Normal; m < numModes; m++ {
		for _, opacity := range []float32{1, 0.6, 0.01} {
			for _, at := range origins {
				want, got := backdrop(w, h), backdrop(w, h)
				CompositeRect(want, buf, at, m, opacity)
				CompositeNRGBA(got, src, at, m, opacity)
				sameBits(t, got, want, m.String())
			}
		}
	}
}

func TestCompositeNRGBAReadsASubImageAtItsOwnBounds(t *testing.T) {
	full := patterned(40, 30)
	sub := full.SubImage(image.Rect(7, 5, 29, 24)).(*image.NRGBA)
	buf, err := raster.FromImage(sub)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []Mode{Normal, Multiply, Hue} {
		want, got := backdrop(48, 36), backdrop(48, 36)
		CompositeRect(want, buf, image.Pt(6, 4), m, 0.8)
		CompositeNRGBA(got, sub, image.Pt(6, 4), m, 0.8)
		sameBits(t, got, want, m.String())
	}
}

func TestCompositeNRGBATouchesOnlyItsRect(t *testing.T) {
	src := patterned(10, 8)
	want, got := backdrop(30, 30), backdrop(30, 30)
	CompositeNRGBA(got, src, image.Pt(12, 9), Normal, 1)
	for y := 0; y < 30; y++ {
		for x := 0; x < 30; x++ {
			if image.Pt(x, y).In(image.Rect(12, 9, 22, 17)) {
				continue
			}
			r1, g1, b1, a1 := got.At(x, y)
			r2, g2, b2, a2 := want.At(x, y)
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				t.Fatalf("pixel (%d,%d) outside the image changed", x, y)
			}
		}
	}
}

func TestCompositeNRGBAIgnoresNonPositiveOpacityAndClampsAboveOne(t *testing.T) {
	src := patterned(10, 8)
	before := backdrop(20, 20)
	for _, o := range []float32{0, -1} {
		got := backdrop(20, 20)
		CompositeNRGBA(got, src, image.Point{}, Normal, o)
		sameBits(t, got, before, "opacity")
	}
	want, got := backdrop(20, 20), backdrop(20, 20)
	CompositeNRGBA(want, src, image.Point{}, Normal, 1)
	CompositeNRGBA(got, src, image.Point{}, Normal, 3)
	sameBits(t, got, want, "opacity above one")
}

func TestCompositeNRGBANeverWritesToItsSource(t *testing.T) {
	src := patterned(16, 16)
	pix := append([]uint8(nil), src.Pix...)
	CompositeNRGBA(backdrop(20, 20), src, image.Point{}, Overlay, 0.5)
	for i := range pix {
		if src.Pix[i] != pix[i] {
			t.Fatal("the source image was written to")
		}
	}
}

func BenchmarkCompositeNRGBA(b *testing.B) {
	src := patterned(2048, 2048)
	dst := raster.MustNewBuffer(2048, 2048)
	b.SetBytes(2048 * 2048 * 4)
	for i := 0; i < b.N; i++ {
		CompositeNRGBA(dst, src, image.Point{}, Normal, 1)
	}
}

func BenchmarkCompositeRectFromImage(b *testing.B) {
	src := patterned(2048, 2048)
	dst := raster.MustNewBuffer(2048, 2048)
	b.SetBytes(2048 * 2048 * 4)
	for i := 0; i < b.N; i++ {
		buf, _ := raster.FromImage(src)
		CompositeRect(dst, buf, image.Point{}, Normal, 1)
	}
}
