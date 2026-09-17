package path

import (
	"math"
	"testing"
)

func coverageSum(p *Path, w, h int) float64 {
	cov := p.Coverage(w, h, NonZero, DefaultTolerance)
	var s float64
	for _, c := range cov {
		s += float64(c)
	}
	return s
}

func TestStrokeButtLineArea(t *testing.T) {
	p := New().MoveTo(10, 20).LineTo(30, 20)
	out := Stroke(p, StrokeStyle{Width: 4, Cap: CapButt})
	got := coverageSum(out, 40, 40)
	want := 20.0 * 4.0
	if math.Abs(got-want) > 2 {
		t.Fatalf("butt line area = %v want ~%v", got, want)
	}
}

func TestStrokeRoundCapArea(t *testing.T) {
	p := New().MoveTo(10, 20).LineTo(30, 20)
	out := Stroke(p, StrokeStyle{Width: 4, Cap: CapRound})
	got := coverageSum(out, 40, 40)
	// Body plus one full disc from the two half-disc caps
	want := 20.0*4.0 + math.Pi*4.0
	if math.Abs(got-want) > 3 {
		t.Fatalf("round cap area = %v want ~%v", got, want)
	}
}

func TestStrokeSquareCapArea(t *testing.T) {
	p := New().MoveTo(10, 20).LineTo(30, 20)
	out := Stroke(p, StrokeStyle{Width: 4, Cap: CapSquare})
	got := coverageSum(out, 40, 40)
	// Each square cap extends the body by half-width
	want := (20.0 + 4.0) * 4.0
	if math.Abs(got-want) > 3 {
		t.Fatalf("square cap area = %v want ~%v", got, want)
	}
}

func TestStrokeClosedRectFrame(t *testing.T) {
	p := New().Rect(10, 10, 30, 30)
	out := Stroke(p, StrokeStyle{Width: 4, Join: JoinMiter})
	cov := out.Coverage(40, 40, NonZero, DefaultTolerance)
	// A point on the frame line is painted
	if covAt(cov, 40, 10, 20) < 0.9 {
		t.Errorf("frame edge not painted: %v", covAt(cov, 40, 10, 20))
	}
	// The interior of the frame is empty
	if covAt(cov, 40, 20, 20) > 0.05 {
		t.Errorf("frame interior should be empty: %v", covAt(cov, 40, 20, 20))
	}
	// Outside the frame is empty
	if covAt(cov, 40, 2, 2) > 0.05 {
		t.Errorf("outside frame should be empty: %v", covAt(cov, 40, 2, 2))
	}
}

func TestStrokeMiterVsBevelCorner(t *testing.T) {
	// A right-angle corner, the miter tip adds area a bevel does not
	mk := func(j Join) float64 {
		p := New().MoveTo(10, 10).LineTo(30, 10).LineTo(30, 30)
		return coverageSum(Stroke(p, StrokeStyle{Width: 6, Join: j, Cap: CapButt, MiterLimit: 10}), 50, 50)
	}
	miter := mk(JoinMiter)
	bevel := mk(JoinBevel)
	if miter <= bevel {
		t.Fatalf("miter area %v should exceed bevel area %v at a sharp corner", miter, bevel)
	}
}

func TestStrokeDashCoverage(t *testing.T) {
	// A 40-long line dashed 4 on 4 off leaves roughly half painted
	p := New().MoveTo(5, 20).LineTo(45, 20)
	solid := coverageSum(Stroke(p, StrokeStyle{Width: 2, Cap: CapButt}), 50, 40)
	dashed := coverageSum(Stroke(p, StrokeStyle{Width: 2, Cap: CapButt, Dash: []float32{4, 4}}), 50, 40)
	ratio := dashed / solid
	if ratio < 0.4 || ratio > 0.65 {
		t.Fatalf("dashed/solid ratio = %v want ~0.5", ratio)
	}
}
