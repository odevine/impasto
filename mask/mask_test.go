package mask

import (
	"image"
	"testing"

	"github.com/odevine/impasto/raster"
)

func TestRasterMaskCoverage(t *testing.T) {
	cov := []float32{0, 0.5, 1, 0.25}
	m := NewRasterMask(cov, 2, 2)
	if m.Coverage(0, 0) != 0 || m.Coverage(1, 0) != 0.5 || m.Coverage(0, 1) != 1 {
		t.Fatal("coverage read back wrong")
	}
	// Outside the extent reveals nothing
	if m.Coverage(-1, 0) != 0 || m.Coverage(2, 0) != 0 || m.Coverage(0, 2) != 0 {
		t.Fatal("out of bounds should be zero coverage")
	}
}

func TestMultiMultiplies(t *testing.T) {
	a := FuncMask(func(x, y int) float32 { return 0.5 })
	b := FuncMask(func(x, y int) float32 { return 0.4 })
	m := Multi{a, b}
	got := m.Coverage(0, 0)
	if got < 0.2-1e-6 || got > 0.2+1e-6 {
		t.Fatalf("multi coverage = %v want 0.2", got)
	}
	// Empty Multi is fully opaque
	if (Multi{}).Coverage(5, 5) != 1 {
		t.Fatal("empty multi should be opaque")
	}
}

func TestApplyAttenuatesPremultiplied(t *testing.T) {
	buf := raster.MustNewBuffer(2, 1)
	// Opaque pixel, premultiplied color equals straight color at alpha 1
	buf.Set(0, 0, 0.8, 0.6, 0.4, 1)
	buf.Set(1, 0, 0.8, 0.6, 0.4, 1)
	m := NewRasterMask([]float32{0.5, 0}, 2, 1)
	Apply(buf, m)

	r, g, b, a := buf.At(0, 0)
	if !eq(r, 0.4) || !eq(g, 0.3) || !eq(b, 0.2) || !eq(a, 0.5) {
		t.Errorf("half coverage = %v,%v,%v,%v", r, g, b, a)
	}
	// The pixel must stay premultiplied: color <= alpha
	if r > a+1e-6 {
		t.Error("mask broke premultiplication invariant")
	}
	r2, _, _, a2 := buf.At(1, 0)
	if r2 != 0 || a2 != 0 {
		t.Error("zero coverage should clear the pixel")
	}
}

func eq(a, b float32) bool {
	d := a - b
	return d < 1e-6 && d > -1e-6
}

// TestApplyAtReadsDocumentCoordinates requires a buffer cut from a larger area to
// be masked as its pixels would be in place
func TestApplyAtReadsDocumentCoordinates(t *testing.T) {
	m := FuncMask(func(x, y int) float32 { return float32((x*3+y*5)%7) / 6 })
	full := raster.MustNewBuffer(30, 20)
	for i := range full.Pix {
		full.Pix[i] = 0.5
	}
	want := full.Clone()
	Apply(want, m)

	const ox, oy, w, h = 9, 4, 11, 8
	part := raster.MustNewBuffer(w, h)
	for i := range part.Pix {
		part.Pix[i] = 0.5
	}
	ApplyAt(part, m, image.Pt(ox, oy))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			wr, _, _, wa := want.At(x+ox, y+oy)
			gr, _, _, ga := part.At(x, y)
			if wr != gr || wa != ga {
				t.Fatalf("(%d,%d): got %v %v, want %v %v", x, y, gr, ga, wr, wa)
			}
		}
	}
}
