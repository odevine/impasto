package raster

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// dirty sets every value of b to something other than zero
func dirty(b *Buffer) *Buffer {
	for i := range b.Pix {
		b.Pix[i] = float32(i%7) + 0.5
	}
	return b
}

func isClear(b *Buffer) bool {
	for _, v := range b.Pix {
		if v != 0 {
			return false
		}
	}
	return true
}

func TestBufferBoundsSizeAndClone(t *testing.T) {
	b := MustNewBuffer(5, 3)
	if w, h := b.Bounds(); w != 5 || h != 3 {
		t.Errorf("Bounds = %d, %d, want 5, 3", w, h)
	}
	if !b.SameSize(MustNewBuffer(5, 3)) || b.SameSize(MustNewBuffer(3, 5)) {
		t.Error("SameSize gave the wrong answer")
	}
	b.Set(2, 1, 0.1, 0.2, 0.3, 0.4)
	c := b.Clone()
	b.Set(2, 1, 0.9, 0.9, 0.9, 0.9)
	if r, g, bl, a := c.At(2, 1); r != 0.1 || g != 0.2 || bl != 0.3 || a != 0.4 {
		t.Errorf("a clone shares storage with its original: %v %v %v %v", r, g, bl, a)
	}
}

func TestBufferClear(t *testing.T) {
	b := dirty(MustNewBuffer(7, 9))
	b.Clear()
	if !isClear(b) {
		t.Error("Clear left a pixel set")
	}
}

func TestMustNewBufferPanicsOnABadSize(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustNewBuffer(0, 5) did not panic")
		}
	}()
	MustNewBuffer(0, 5)
}

func TestFromFileDecodesAPNG(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	src.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 128})
	path := filepath.Join(t.TempDir(), "in.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, src); err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, err := FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := FromImage(src)
	for i := range want.Pix {
		if got.Pix[i] != want.Pix[i] {
			t.Fatalf("value %d = %v, want %v", i, got.Pix[i], want.Pix[i])
		}
	}
}

func TestFromFileReportsAMissingOrUndecodableFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := FromFile(filepath.Join(dir, "absent.png")); err == nil {
		t.Error("a missing file decoded")
	}
	junk := filepath.Join(dir, "junk.png")
	if err := os.WriteFile(junk, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FromFile(junk); err == nil {
		t.Error("a file that is not an image decoded")
	}
}

func TestFromImageRefusesAnImageTooWide(t *testing.T) {
	if _, err := FromImage(image.NewNRGBA(image.Rect(0, 0, MaxDimension+1, 1))); err == nil {
		t.Error("FromImage accepted an image past MaxDimension")
	}
}

// Every image type must reach the same linear values as the generic path, which
// asks for straight 16-bit sRGB through the color model
func TestFromImageMatchesTheGenericPathForEachType(t *testing.T) {
	const w, h = 9, 7
	nrgba64 := image.NewNRGBA64(image.Rect(0, 0, w, h))
	gray16 := image.NewGray16(image.Rect(0, 0, w, h))
	gray := image.NewGray(image.Rect(0, 0, w, h))
	ycbcr := image.NewYCbCr(image.Rect(0, 0, w, h), image.YCbCrSubsampleRatio420)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			nrgba64.SetNRGBA64(x, y, color.NRGBA64{R: uint16(x * 7000), G: uint16(y * 9000), B: uint16(x*y*500 + 3), A: uint16(65535 - x*5000)})
			gray16.SetGray16(x, y, color.Gray16{Y: uint16(x*7000 + y*100)})
			gray.SetGray(x, y, color.Gray{Y: uint8(x*25 + y)})
		}
	}
	for i := range ycbcr.Y {
		ycbcr.Y[i] = uint8(i * 5)
	}
	for name, img := range map[string]image.Image{"nrgba64": nrgba64, "gray16": gray16, "gray": gray, "ycbcr": ycbcr} {
		got, err := FromImage(img)
		if err != nil {
			t.Fatal(err)
		}
		want := MustNewBuffer(w, h)
		want.fromGeneric(img)
		for i := range want.Pix {
			if d := math.Abs(float64(got.Pix[i] - want.Pix[i])); d > 1e-6 {
				t.Fatalf("%s: value %d (pixel %d, channel %d) = %v, generic path gives %v", name, i, i/4, i%4, got.Pix[i], want.Pix[i])
			}
		}
	}
}

func TestToImage16ClampsOutOfRangeValues(t *testing.T) {
	buf := MustNewBuffer(2, 1)
	buf.Set(0, 0, 3, -2, 0.5, 1)
	buf.Set(1, 0, 0.2, 0.2, 0.2, 1.7)
	out := buf.ToImage(16).(*image.NRGBA64)
	if got := out.NRGBA64At(0, 0); got.R != 65535 || got.G != 0 || got.A != 65535 {
		t.Errorf("clamped pixel = %+v, want R 65535, G 0, A 65535", got)
	}
	if got := out.NRGBA64At(1, 0); got.A != 65535 {
		t.Errorf("alpha 1.7 encoded as %d, want 65535", got.A)
	}
	// Alpha below zero clamps too, and a pixel with no alpha encodes as nothing
	buf.Set(0, 0, 1, 1, 1, -3)
	if got := buf.ToImage(16).(*image.NRGBA64).NRGBA64At(0, 0); got != (color.NRGBA64{}) {
		t.Errorf("negative alpha encoded as %+v, want transparent black", got)
	}
}
