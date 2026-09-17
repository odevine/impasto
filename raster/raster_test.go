package raster

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestSRGBLinearRoundTrip(t *testing.T) {
	for i := 0; i <= 1000; i++ {
		s := float32(i) / 1000.0
		got := LinearToSRGB(SRGBToLinear(s))
		if math.Abs(float64(got-s)) > 1e-5 {
			t.Fatalf("round trip at %v: got %v", s, got)
		}
	}
}

func TestSRGBKnownValues(t *testing.T) {
	// The transfer function must map the endpoints exactly and cross the
	// linear/power split at the documented threshold
	cases := []struct {
		s, want float32
	}{
		{0, 0},
		{1, 1},
		{0.5, 0.21404114},
	}
	for _, c := range cases {
		got := SRGBToLinear(c.s)
		if math.Abs(float64(got-c.want)) > 1e-6 {
			t.Errorf("SRGBToLinear(%v) = %v want %v", c.s, got, c.want)
		}
	}
}

func TestLUTMatchesFunction(t *testing.T) {
	for i := 0; i < 256; i++ {
		want := SRGBToLinear(float32(i) / 255.0)
		if srgb8ToLinearLUT[i] != want {
			t.Fatalf("8-bit LUT[%d] = %v want %v", i, srgb8ToLinearLUT[i], want)
		}
	}
}

func TestNewBufferValidation(t *testing.T) {
	cases := [][2]int{{0, 10}, {10, 0}, {-1, 5}, {MaxDimension + 1, 1}}
	for _, c := range cases {
		if _, err := NewBuffer(c[0], c[1]); err == nil {
			t.Errorf("NewBuffer(%d,%d) expected error", c[0], c[1])
		}
	}
	if _, err := NewBuffer(16, 16); err != nil {
		t.Errorf("NewBuffer(16,16) unexpected error: %v", err)
	}
}

func TestFromImagePremultiplies(t *testing.T) {
	// A half-transparent mid-gray pixel must be stored premultiplied and in
	// linear light
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.Set(0, 0, color.NRGBA{R: 128, G: 128, B: 128, A: 128})
	buf, err := FromImage(src)
	if err != nil {
		t.Fatal(err)
	}
	a := float32(128) / 255.0
	lin := SRGBToLinear(float32(128) / 255.0)
	wantPremul := lin * a
	r, g, bl, ga := buf.At(0, 0)
	if !approx(r, wantPremul) || !approx(g, wantPremul) || !approx(bl, wantPremul) {
		t.Errorf("premultiplied rgb = %v,%v,%v want %v", r, g, bl, wantPremul)
	}
	if !approx(ga, a) {
		t.Errorf("alpha = %v want %v", ga, a)
	}
}

func TestRoundTripImage8(t *testing.T) {
	// Opaque pixels must survive a full ingest and egress within one 8-bit code
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	vals := []color.NRGBA{
		{R: 0, G: 0, B: 0, A: 255},
		{R: 255, G: 255, B: 255, A: 255},
		{R: 200, G: 100, B: 50, A: 255},
		{R: 10, G: 20, B: 30, A: 255},
	}
	i := 0
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.Set(x, y, vals[i%len(vals)])
			i++
		}
	}
	buf, err := FromImage(src)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.ToImage(8).(*image.NRGBA)
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			w := src.NRGBAAt(x, y)
			g := out.NRGBAAt(x, y)
			if diff8(w.R, g.R) > 1 || diff8(w.G, g.G) > 1 || diff8(w.B, g.B) > 1 || diff8(w.A, g.A) > 1 {
				t.Errorf("pixel (%d,%d) = %v want %v", x, y, g, w)
			}
		}
	}
}

func TestToImageDeterministic(t *testing.T) {
	buf := MustNewBuffer(8, 8)
	for i := range buf.Pix {
		buf.Pix[i] = 0.3
	}
	a := buf.ToImage(8).(*image.NRGBA)
	b := buf.ToImage(8).(*image.NRGBA)
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			t.Fatalf("ToImage not deterministic at %d", i)
		}
	}
}

func approx(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

func diff8(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}
