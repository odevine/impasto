package blur

import (
	"math"
	"testing"

	"github.com/odevine/impasto/raster"
)

func TestConstantPreserved(t *testing.T) {
	// A flat field must survive a blur unchanged, edge clamping guarantees this
	b := raster.MustNewBuffer(64, 64)
	for i := range b.Pix {
		b.Pix[i] = 0.5
	}
	Gaussian(b, 6)
	for i, v := range b.Pix {
		if math.Abs(float64(v-0.5)) > 1e-4 {
			t.Fatalf("constant not preserved at %d: %v", i, v)
		}
	}
}

func TestImpulseMassConserved(t *testing.T) {
	// A single opaque pixel in the center should spread but keep its total mass
	b := raster.MustNewBuffer(64, 64)
	b.Set(32, 32, 1, 1, 1, 1)
	before := sumAlpha(b)
	Gaussian(b, 4)
	after := sumAlpha(b)
	if math.Abs(before-after) > 1e-3 {
		t.Fatalf("mass not conserved: before %v after %v", before, after)
	}
	// The impulse spread, so the center value dropped below 1
	if _, _, _, a := b.At(32, 32); a >= 1 {
		t.Fatalf("center should have spread, still %v", a)
	}
}

func TestImpulseSymmetry(t *testing.T) {
	b := raster.MustNewBuffer(65, 65)
	b.Set(32, 32, 1, 1, 1, 1)
	Gaussian(b, 5)
	_, _, _, left := b.At(20, 32)
	_, _, _, right := b.At(44, 32)
	_, _, _, up := b.At(32, 20)
	_, _, _, down := b.At(32, 44)
	if !close32(left, right) || !close32(up, down) || !close32(left, up) {
		t.Fatalf("blur not symmetric: L%v R%v U%v D%v", left, right, up, down)
	}
}

func TestBlurDeterministic(t *testing.T) {
	mk := func() *raster.Buffer {
		b := raster.MustNewBuffer(128, 128)
		for i := 0; i < len(b.Pix); i += 4 {
			b.Pix[i+3] = float32((i/4)%7) / 7
		}
		return b
	}
	a := mk()
	c := mk()
	Gaussian(a, 3.5)
	Gaussian(c, 3.5)
	for i := range a.Pix {
		if a.Pix[i] != c.Pix[i] {
			t.Fatalf("nondeterministic blur at %d", i)
		}
	}
}

func TestZeroSigmaNoop(t *testing.T) {
	b := raster.MustNewBuffer(16, 16)
	b.Set(8, 8, 1, 1, 1, 1)
	Gaussian(b, 0)
	if _, _, _, a := b.At(8, 8); a != 1 {
		t.Fatal("zero sigma should be a no-op")
	}
}

func sumAlpha(b *raster.Buffer) float64 {
	var s float64
	for i := 3; i < len(b.Pix); i += 4 {
		s += float64(b.Pix[i])
	}
	return s
}

func close32(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-5 }
