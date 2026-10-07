package raster

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"runtime"
	"testing"
)

// ramp fills a buffer with smooth color and an alpha that varies, premultiplied
func ramp(w, h int) *Buffer {
	buf := MustNewBuffer(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := 0.2 + 0.8*float32((x*3+y*5)%11)/10
			buf.Set(x, y, a*float32(x)/float32(w), a*float32(y)/float32(h), a*float32(x+y)/float32(w+h), a)
		}
	}
	return buf
}

// refCode is the exact 8-bit sRGB code of a linear value
func refCode(v float32) int32 {
	x := math.Min(math.Max(float64(v), 0), 1)
	if x <= 0.0031308 {
		return int32(math.Round(x * 12.92 * 255))
	}
	return int32(math.Round((1.055*math.Pow(x, 1.0/2.4) - 0.055) * 255))
}

// refYCbCr converts with plain loops and no banding, from the same definition
func refYCbCr(b *Buffer, bg [3]float32) *image.YCbCr {
	dst := image.NewYCbCr(image.Rect(0, 0, b.Width, b.Height), image.YCbCrSubsampleRatio420)
	px := func(x, y int) (r, g, bl int32) {
		x, y = min(x, b.Width-1), min(y, b.Height-1)
		pr, pg, pb, pa := b.At(x, y)
		show := max(1-pa, 0)
		return refCode(pr + show*bg[0]), refCode(pg + show*bg[1]), refCode(pb + show*bg[2])
	}
	for y := 0; y < b.Height; y += 2 {
		for x := 0; x < b.Width; x += 2 {
			var sr, sg, sb int32
			for _, d := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				r, g, bl := px(x+d[0], y+d[1])
				if x+d[0] < b.Width && y+d[1] < b.Height {
					dst.Y[(y+d[1])*dst.YStride+x+d[0]] = uint8((19595*r + 38470*g + 7471*bl + 1<<15) >> 16)
				}
				sr, sg, sb = sr+r, sg+g, sb+bl
			}
			c := y/2*dst.CStride + x/2
			dst.Cb[c] = chromaCode(-11059*sr - 21709*sg + 32768*sb)
			dst.Cr[c] = chromaCode(32768*sr - 27439*sg - 5329*sb)
		}
	}
	return dst
}

// closeYCbCr allows each sample to differ by one, which is what the lookup table
// does against the exact transfer function
func closeYCbCr(t *testing.T, got, want *image.YCbCr, what string) {
	t.Helper()
	if got.Rect != want.Rect || got.SubsampleRatio != want.SubsampleRatio {
		t.Fatalf("%s: %v %v, want %v %v", what, got.Rect, got.SubsampleRatio, want.Rect, want.SubsampleRatio)
	}
	for _, p := range []struct {
		name      string
		got, want []uint8
	}{{"Y", got.Y, want.Y}, {"Cb", got.Cb, want.Cb}, {"Cr", got.Cr, want.Cr}} {
		for i := range p.want {
			if d := int(p.got[i]) - int(p.want[i]); d < -1 || d > 1 {
				t.Fatalf("%s: %s[%d] = %d, want %d within one", what, p.name, i, p.got[i], p.want[i])
			}
		}
	}
}

func sameYCbCr(t *testing.T, got, want *image.YCbCr, what string) {
	t.Helper()
	if got.Rect != want.Rect || got.SubsampleRatio != want.SubsampleRatio {
		t.Fatalf("%s: %v %v, want %v %v", what, got.Rect, got.SubsampleRatio, want.Rect, want.SubsampleRatio)
	}
	for _, p := range []struct {
		name      string
		got, want []uint8
	}{{"Y", got.Y, want.Y}, {"Cb", got.Cb, want.Cb}, {"Cr", got.Cr, want.Cr}} {
		if !bytes.Equal(p.got, p.want) {
			t.Fatalf("%s: plane %s differs", what, p.name)
		}
	}
}

func TestToYCbCrMatchesAReference(t *testing.T) {
	// Even and odd sizes, since an edge block averages pixels that exist
	for _, size := range [][2]int{{64, 48}, {63, 47}, {1, 1}, {2, 1}, {1, 2}, {3, 5}} {
		buf := ramp(size[0], size[1])
		closeYCbCr(t, buf.ToYCbCr(), refYCbCr(buf, [3]float32{}), "default background")
	}
}

func TestToYCbCrIsCloseToToImage(t *testing.T) {
	// ToImage dithers and unpremultiplies, so only the luma of an opaque buffer
	// can be compared, to within the dither
	buf := MustNewBuffer(40, 30)
	for y := 0; y < 30; y++ {
		for x := 0; x < 40; x++ {
			buf.Set(x, y, float32(x)/40, float32(y)/30, 0.5, 1)
		}
	}
	got, ref := buf.ToYCbCr(), buf.ToImage(8)
	for y := 0; y < 30; y++ {
		for x := 0; x < 40; x++ {
			r, g, b, _ := ref.At(x, y).RGBA()
			want, _, _ := color.RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			if d := int(got.Y[y*got.YStride+x]) - int(want); d < -2 || d > 2 {
				t.Fatalf("Y at (%d,%d) = %d, ToImage gives %d", x, y, got.Y[y*got.YStride+x], want)
			}
		}
	}
}

func TestToYCbCrFlattensOverABackground(t *testing.T) {
	buf := MustNewBuffer(4, 4)
	buf.Set(0, 0, 1, 0, 0, 1) // opaque red, the rest transparent
	white := buf.ToYCbCr(color.White)
	// A transparent pixel shows the background in full
	if y := white.Y[1*white.YStride+1]; y != 255 {
		t.Errorf("transparent pixel over white has Y %d, want 255", y)
	}
	if cb, cr := white.Cb[1*white.CStride+1], white.Cr[1*white.CStride+1]; cb != 128 || cr != 128 {
		t.Errorf("a block of white has chroma %d,%d, want 128,128", cb, cr)
	}
	// An opaque pixel hides it
	black := buf.ToYCbCr()
	if white.Y[0] != black.Y[0] {
		t.Errorf("an opaque pixel changed with the background: %d against %d", white.Y[0], black.Y[0])
	}
	// The default is black, and black says the same thing
	sameYCbCr(t, buf.ToYCbCr(color.Black), black, "explicit black")
	sameYCbCr(t, buf.ToYCbCr(nil), black, "nil background")
	sameYCbCr(t, buf.ToYCbCr(color.Transparent), black, "transparent background")
	// A mid-gray lands between
	gray := buf.ToYCbCr(color.NRGBA{R: 128, G: 128, B: 128, A: 255})
	if y := gray.Y[1*gray.YStride+1]; y != 128 {
		t.Errorf("transparent pixel over gray 128 has Y %d, want 128", y)
	}
}

func TestToYCbCrMatchesAReferenceOverABackground(t *testing.T) {
	buf := ramp(37, 29)
	c := color.NRGBA{R: 200, G: 90, B: 30, A: 255}
	bg, _ := premultipliedLinear([]color.Color{c})
	closeYCbCr(t, buf.ToYCbCr(c), refYCbCr(buf, bg), "orange background")
}

func TestToYCbCrOfAnOutOfRangeBufferStaysInRange(t *testing.T) {
	buf := MustNewBuffer(3, 3)
	buf.Set(0, 0, 5, -2, float32(math.NaN()), 1)
	buf.Set(1, 1, float32(math.Inf(1)), float32(math.Inf(-1)), 0.5, float32(math.NaN()))
	out := buf.ToYCbCr(color.White)
	if len(out.Y) == 0 {
		t.Fatal("no luma")
	}
}

func TestToYCbCrDoesNotDependOnParallelism(t *testing.T) {
	buf := ramp(211, 997)
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	one := buf.ToYCbCr(color.White)
	for _, procs := range []int{2, 5, 16} {
		runtime.GOMAXPROCS(procs)
		sameYCbCr(t, buf.ToYCbCr(color.White), one, "GOMAXPROCS")
	}
}

func TestToYCbCrEncodesAsAJPEG(t *testing.T) {
	buf := ramp(100, 70)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, buf.ToYCbCr(color.White), &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	dec, err := jpeg.Decode(&out)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Bounds() != image.Rect(0, 0, 100, 70) {
		t.Errorf("decoded %v", dec.Bounds())
	}
}

func TestToImageDoesNotDependOnParallelism(t *testing.T) {
	buf := ramp(123, 1001)
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	one8, one16 := buf.ToImage(8).(*image.NRGBA), buf.ToImage(16).(*image.NRGBA64)
	for _, procs := range []int{2, 7, 16} {
		runtime.GOMAXPROCS(procs)
		if !bytes.Equal(buf.ToImage(8).(*image.NRGBA).Pix, one8.Pix) {
			t.Fatalf("8-bit ToImage differs at GOMAXPROCS %d", procs)
		}
		if !bytes.Equal(buf.ToImage(16).(*image.NRGBA64).Pix, one16.Pix) {
			t.Fatalf("16-bit ToImage differs at GOMAXPROCS %d", procs)
		}
	}
}

func BenchmarkToYCbCr(b *testing.B) {
	buf := benchBuffer(b)
	b.SetBytes(int64(buf.Width * buf.Height * 4))
	for b.Loop() {
		buf.ToYCbCr()
	}
}

// The table is the exact encoding of each quantized value, so its only error is
// the quantization itself
func TestSRGBCodeTableIsExactAtItsEntries(t *testing.T) {
	for i := range srgbCodes {
		if want := refCode(float32(i) / 65535); int32(srgbCodes[i]) != want {
			t.Fatalf("entry %d = %d, want %d", i, srgbCodes[i], want)
		}
	}
	if srgbCodes[0] != 0 || srgbCodes[65535] != 255 {
		t.Errorf("ends are %d and %d, want 0 and 255", srgbCodes[0], srgbCodes[65535])
	}
}

func TestCode8NeverLeavesItsRangeAndMapsNonNumbersToZero(t *testing.T) {
	for _, v := range []float32{float32(math.NaN()), float32(math.Inf(-1)), -3, 0} {
		if got := code8(v); got != 0 {
			t.Errorf("code8(%v) = %d, want 0", v, got)
		}
	}
	for _, v := range []float32{1, 1.5, float32(math.Inf(1))} {
		if got := code8(v); got != 255 {
			t.Errorf("code8(%v) = %d, want 255", v, got)
		}
	}
}
