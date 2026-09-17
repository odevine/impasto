package path

import (
	"math"
	"testing"
)

func covAt(cov []float32, w, x, y int) float32 { return cov[y*w+x] }

func TestFullRectCoverage(t *testing.T) {
	p := New().Rect(0, 0, 10, 10)
	cov := p.Coverage(10, 10, NonZero, DefaultTolerance)
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			if math.Abs(float64(covAt(cov, 10, x, y)-1)) > 1e-4 {
				t.Fatalf("interior pixel (%d,%d) coverage = %v want 1", x, y, covAt(cov, 10, x, y))
			}
		}
	}
}

func TestHalfRectCoverage(t *testing.T) {
	// A rectangle covering the left five columns exactly
	p := New().Rect(0, 0, 5, 10)
	cov := p.Coverage(10, 10, NonZero, DefaultTolerance)
	for y := 0; y < 10; y++ {
		for x := 0; x < 5; x++ {
			if math.Abs(float64(covAt(cov, 10, x, y)-1)) > 1e-4 {
				t.Fatalf("left pixel (%d,%d) = %v want 1", x, y, covAt(cov, 10, x, y))
			}
		}
		for x := 5; x < 10; x++ {
			if covAt(cov, 10, x, y) > 1e-4 {
				t.Fatalf("right pixel (%d,%d) = %v want 0", x, y, covAt(cov, 10, x, y))
			}
		}
	}
}

func TestPartialPixelCoverage(t *testing.T) {
	// A thin rectangle covering 30% of column 0
	p := New().Rect(0, 0, 0.3, 4)
	cov := p.Coverage(4, 4, NonZero, DefaultTolerance)
	for y := 0; y < 4; y++ {
		got := covAt(cov, 4, 0, y)
		if math.Abs(float64(got-0.3)) > 1e-4 {
			t.Fatalf("partial pixel (0,%d) = %v want 0.3", y, got)
		}
	}
}

func TestTriangleAreaConservation(t *testing.T) {
	// A right triangle with legs 40, its total coverage must equal its area
	p := New().MoveTo(0, 0).LineTo(40, 0).LineTo(0, 40).Close()
	cov := p.Coverage(64, 64, NonZero, DefaultTolerance)
	var sum float64
	for _, c := range cov {
		sum += float64(c)
	}
	want := 0.5 * 40 * 40
	if math.Abs(sum-want) > 1.0 {
		t.Fatalf("triangle coverage sum = %v want ~%v", sum, want)
	}
}

func TestEvenOddHole(t *testing.T) {
	// An outer square with a concentric inner square, even-odd leaves a hole
	p := New().Rect(0, 0, 20, 20).Rect(5, 5, 15, 15)
	cov := p.Coverage(20, 20, EvenOdd, DefaultTolerance)
	if covAt(cov, 20, 10, 10) > 1e-3 {
		t.Fatalf("center should be a hole, got %v", covAt(cov, 20, 10, 10))
	}
	if math.Abs(float64(covAt(cov, 20, 2, 2)-1)) > 1e-3 {
		t.Fatalf("ring should be filled, got %v", covAt(cov, 20, 2, 2))
	}
}

func TestNonZeroNoHole(t *testing.T) {
	// Same nested squares wound the same way fill solid under nonzero
	p := New().Rect(0, 0, 20, 20).Rect(5, 5, 15, 15)
	cov := p.Coverage(20, 20, NonZero, DefaultTolerance)
	if math.Abs(float64(covAt(cov, 20, 10, 10)-1)) > 1e-3 {
		t.Fatalf("center should be filled under nonzero, got %v", covAt(cov, 20, 10, 10))
	}
}

func TestCircleAreaConservation(t *testing.T) {
	p := New().Ellipse(50, 50, 30, 30)
	cov := p.Coverage(100, 100, NonZero, DefaultTolerance)
	var sum float64
	for _, c := range cov {
		sum += float64(c)
	}
	want := math.Pi * 30 * 30
	if math.Abs(sum-want) > 5.0 {
		t.Fatalf("circle coverage sum = %v want ~%v", sum, want)
	}
}
