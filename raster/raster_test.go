package raster

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"math/rand/v2"
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

// encode8 must agree with the exact path for every input, so the table is only
// ever a shortcut
func TestEncode8MatchesExactPath(t *testing.T) {
	check := func(c float32) {
		for d := 0; d < 64; d++ {
			off := ditherOffset(d&7, d>>3)
			if got, want := encode8(c, off), quantize8(LinearToSRGB(c), off); got != want {
				t.Fatalf("c=%v dither=%v: got %d, want %d", c, off, got, want)
			}
		}
	}
	for i := 0; i <= 200000; i++ {
		check(float32(i) / 200000)
	}
	// Dense through the steep low end, where the table is least accurate
	for i := 0; i <= 100000; i++ {
		check(0.003 + float32(i)*1e-7)
	}
	for k := 0; k < 256; k++ {
		c := SRGBToLinear(float32(k) / 255)
		for _, v := range []float32{c, math.Nextafter32(c, 0), math.Nextafter32(c, 1)} {
			check(v)
		}
	}
	for _, c := range []float32{-1, 0, 1, 2, float32(math.NaN()), float32(math.Inf(1))} {
		off := ditherOffset(3, 5)
		if got, want := encode8(c, off), quantize8(LinearToSRGB(c), off); got != want {
			t.Fatalf("c=%v: got %d, want %d", c, got, want)
		}
	}
}

// The table's interpolation error stays well inside the margin that decides
// when encode8 must take the exact path
func TestEncodeTableErrorWithinMargin(t *testing.T) {
	worst := 0.0
	for i := 0; i < 2_000_000; i++ {
		c := 0.0031308 + float64(i)/2_000_000*(1-0.0031308)
		x := c * srgbSteps
		j := int(x)
		approx := float64(srgbEncodeTable[j]) + float64(srgbEncodeTable[j+1]-srgbEncodeTable[j])*(x-float64(j))
		exact := 255 * (1.055*math.Pow(c, 1/2.4) - 0.055)
		worst = max(worst, math.Abs(approx-exact))
	}
	if worst > encodeMargin/3 {
		t.Errorf("interpolation error %.5f codes, margin %.5f", worst, encodeMargin)
	}
}

func benchBuffer(b *testing.B) *Buffer {
	b.Helper()
	buf, err := NewBuffer(1200, 1600)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < len(buf.Pix); i += 4 {
		v := float32(i/4%997) / 997
		a := float32(1)
		if i/4%5 == 0 {
			a = 0.6
		}
		buf.Pix[i], buf.Pix[i+1], buf.Pix[i+2], buf.Pix[i+3] = v*a, v*v*a, (1-v)*a, a
	}
	return buf
}

func BenchmarkToImage8(b *testing.B) {
	buf := benchBuffer(b)
	b.SetBytes(int64(buf.Width * buf.Height * 4))
	for b.Loop() {
		buf.ToImage(8)
	}
}

// refToNRGBA is toNRGBA with the exact transfer function on every channel
func refToNRGBA(b *Buffer) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, b.Width, b.Height))
	si := 0
	for y := 0; y < b.Height; y++ {
		di := dst.PixOffset(0, y)
		for x := 0; x < b.Width; x++ {
			r, g, bl, a := unpremultiply(b.Pix[si], b.Pix[si+1], b.Pix[si+2], b.Pix[si+3])
			d := ditherOffset(x, y)
			dst.Pix[di] = quantize8(LinearToSRGB(r), d)
			dst.Pix[di+1] = quantize8(LinearToSRGB(g), d)
			dst.Pix[di+2] = quantize8(LinearToSRGB(bl), d)
			dst.Pix[di+3] = quantize8(a, d)
			si += 4
			di += 4
		}
	}
	return dst
}

func TestToImage8MatchesExactPath(t *testing.T) {
	buf, err := NewBuffer(257, 131)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < len(buf.Pix); i += 4 {
		a := rng.Float32()
		switch rng.IntN(4) {
		case 0:
			a = 1
		case 1:
			a = 0
		}
		// Values past the alpha and below zero are valid after blend modes
		buf.Pix[i] = (rng.Float32()*1.2 - 0.1) * a
		buf.Pix[i+1] = rng.Float32() * a * a
		buf.Pix[i+2] = rng.Float32() * a
		buf.Pix[i+3] = a
	}
	got := buf.ToImage(8).(*image.NRGBA)
	want := refToNRGBA(buf)
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatal("ToImage(8) differs from the exact transfer function")
	}
}

func BenchmarkToImage8Exact(b *testing.B) {
	buf := benchBuffer(b)
	b.SetBytes(int64(buf.Width * buf.Height * 4))
	for b.Loop() {
		refToNRGBA(buf)
	}
}

func TestFromRGBAMatchesGeneric(t *testing.T) {
	img := image.NewRGBA(image.Rect(3, 2, 130, 91))
	rng := rand.New(rand.NewPCG(3, 4))
	for i := range img.Pix {
		img.Pix[i] = uint8(rng.UintN(256))
	}
	// Alpha runs of zero and full, plus bytes that are not valid premultiplied
	for i := 3; i < len(img.Pix); i += 4 * 7 {
		img.Pix[i] = []uint8{0, 255}[rng.IntN(2)]
	}
	got, err := FromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := NewBuffer(img.Bounds().Dx(), img.Bounds().Dy())
	want.fromGeneric(img)
	for i := range got.Pix {
		if got.Pix[i] != want.Pix[i] {
			t.Fatalf("sample %d: got %v, want %v", i, got.Pix[i], want.Pix[i])
		}
	}
}

func BenchmarkFromRGBA(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 1600))
	for i := range img.Pix {
		img.Pix[i] = uint8(i)
	}
	for b.Loop() {
		FromImage(img)
	}
}
