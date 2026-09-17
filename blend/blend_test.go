package blend

import (
	"math"
	"testing"

	"github.com/odevine/impasto/raster"
)

// refSeparable recomputes each separable mode independently from the ISO
// formulas so the test does not just mirror the implementation
func refSeparable(m Mode, cb, cs float64) float64 {
	switch m {
	case Normal:
		return cs
	case Multiply:
		return cb * cs
	case Screen:
		return cb + cs - cb*cs
	case Overlay:
		return refHardLight(cs, cb)
	case HardLight:
		return refHardLight(cb, cs)
	case SoftLight:
		if cs <= 0.5 {
			return cb - (1-2*cs)*cb*(1-cb)
		}
		var d float64
		if cb <= 0.25 {
			d = ((16*cb-12)*cb + 4) * cb
		} else {
			d = math.Sqrt(cb)
		}
		return cb + (2*cs-1)*(d-cb)
	case ColorDodge:
		if cb == 0 {
			return 0
		}
		if cs == 1 {
			return 1
		}
		return math.Min(1, cb/(1-cs))
	case ColorBurn:
		if cb == 1 {
			return 1
		}
		if cs == 0 {
			return 0
		}
		return 1 - math.Min(1, (1-cb)/cs)
	case Darken:
		return math.Min(cb, cs)
	case Lighten:
		return math.Max(cb, cs)
	case Difference:
		return math.Abs(cb - cs)
	case Exclusion:
		return cb + cs - 2*cb*cs
	}
	return cs
}

func refHardLight(cb, cs float64) float64 {
	if cs <= 0.5 {
		return cb * 2 * cs
	}
	s := 2*cs - 1
	return cb + s - cb*s
}

var separableModes = []Mode{
	Normal, Multiply, Screen, Overlay, SoftLight, HardLight,
	ColorDodge, ColorBurn, Darken, Lighten, Difference, Exclusion,
}

// TestSeparableAgainstReference walks a grid of (Cb,Cs) including the
// discontinuity endpoints and checks opaque-over-opaque compositing equals the
// independently computed formula
func TestSeparableAgainstReference(t *testing.T) {
	grid := []float64{0, 0.25, 0.5, 0.75, 1}
	for _, m := range separableModes {
		for _, cb := range grid {
			for _, cs := range grid {
				out := BlendPixel(
					[4]float32{f(cb), f(cb), f(cb), 1},
					[4]float32{f(cs), f(cs), f(cs), 1},
					m,
				)
				want := refSeparable(m, cb, cs)
				if math.Abs(float64(out[0])-want) > 1e-6 {
					t.Errorf("%v(cb=%v,cs=%v) = %v want %v", m, cb, cs, out[0], want)
				}
			}
		}
	}
}

// TestInRange asserts every mode keeps results within [0,1] for in-range inputs,
// which catches formula transcription errors immediately
func TestInRange(t *testing.T) {
	grid := []float64{0, 0.1, 0.33, 0.5, 0.67, 0.9, 1}
	for m := Normal; m < numModes; m++ {
		for _, cb := range grid {
			for _, cs := range grid {
				out := BlendPixel(
					[4]float32{f(cb) * 0.8, f(cb) * 0.6, f(cb) * 0.4, 0.8},
					[4]float32{f(cs) * 0.7, f(cs) * 0.5, f(cs) * 0.3, 0.7},
					m,
				)
				for i := 0; i < 4; i++ {
					if out[i] < -1e-5 || out[i] > 1+1e-5 {
						t.Errorf("%v out[%d]=%v out of range (cb=%v cs=%v)", m, i, out[i], cb, cs)
					}
				}
			}
		}
	}
}

// TestNonSeparableLuminance checks the defining luminance invariants of the
// whole-triple modes on opaque pixels
func TestNonSeparableLuminance(t *testing.T) {
	cb := [3]float32{0.2, 0.6, 0.9}
	cs := [3]float32{0.8, 0.3, 0.1}
	lumOf := func(r, g, b float32) float64 {
		return float64(0.3*r + 0.59*g + 0.11*b)
	}
	backLum := lumOf(cb[0], cb[1], cb[2])
	srcLum := lumOf(cs[0], cs[1], cs[2])

	check := func(m Mode, want float64) {
		out := BlendPixel(
			[4]float32{cb[0], cb[1], cb[2], 1},
			[4]float32{cs[0], cs[1], cs[2], 1},
			m,
		)
		got := lumOf(out[0], out[1], out[2])
		if math.Abs(got-want) > 1e-5 {
			t.Errorf("%v luminance = %v want %v", m, got, want)
		}
	}
	check(Hue, backLum)
	check(Saturation, backLum)
	check(Color, backLum)
	check(Luminosity, srcLum)
}

func TestIdentityAndZeroOpacity(t *testing.T) {
	dst := raster.MustNewBuffer(32, 32)
	src := raster.MustNewBuffer(32, 32)
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i] = 0.4
		src.Pix[i+1] = 0.5
		src.Pix[i+2] = 0.6
		src.Pix[i+3] = 1
	}

	// Normal at full opacity over transparent backdrop reproduces the source
	got := dst.Clone()
	Composite(got, src, Normal, 1.0)
	for i := range got.Pix {
		if math.Abs(float64(got.Pix[i]-src.Pix[i])) > 1e-6 {
			t.Fatalf("identity failed at %d: %v vs %v", i, got.Pix[i], src.Pix[i])
		}
	}

	// Zero opacity is a no-op for every mode
	base := raster.MustNewBuffer(32, 32)
	for i := 0; i < len(base.Pix); i += 4 {
		base.Pix[i] = 0.1
		base.Pix[i+1] = 0.2
		base.Pix[i+2] = 0.3
		base.Pix[i+3] = 0.5
	}
	for m := Normal; m < numModes; m++ {
		g := base.Clone()
		Composite(g, src, m, 0.0)
		for i := range g.Pix {
			if g.Pix[i] != base.Pix[i] {
				t.Fatalf("%v zero-opacity changed pixel %d", m, i)
			}
		}
	}
}

func TestCompositeDeterministic(t *testing.T) {
	mk := func() (*raster.Buffer, *raster.Buffer) {
		d := raster.MustNewBuffer(200, 200)
		s := raster.MustNewBuffer(200, 200)
		for i := 0; i < len(d.Pix); i += 4 {
			d.Pix[i], d.Pix[i+1], d.Pix[i+2], d.Pix[i+3] = 0.3, 0.4, 0.5, 0.8
			s.Pix[i], s.Pix[i+1], s.Pix[i+2], s.Pix[i+3] = 0.6, 0.2, 0.1, 0.7
		}
		return d, s
	}
	d1, s1 := mk()
	d2, s2 := mk()
	Composite(d1, s1, Overlay, 0.9)
	Composite(d2, s2, Overlay, 0.9)
	for i := range d1.Pix {
		if d1.Pix[i] != d2.Pix[i] {
			t.Fatalf("nondeterministic composite at %d", i)
		}
	}
}

func f(v float64) float32 { return float32(v) }
