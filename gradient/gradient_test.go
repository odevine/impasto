package gradient

import (
	"image/color"
	"math"
	"testing"

	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

func black() color.Color { return color.NRGBA{0, 0, 0, 255} }
func white() color.Color { return color.NRGBA{255, 255, 255, 255} }

func TestLinearEndpoints(t *testing.T) {
	g := New(Linear, Pad, path.Point{X: 0, Y: 0}, path.Point{X: 10, Y: 0}, []Stop{
		{Pos: 0, Color: black(), Opacity: 1},
		{Pos: 1, Color: white(), Opacity: 1},
	})
	// Far left is near black, far right is near white, in linear light
	l0 := g.ColorAt(0, 0)
	l1 := g.ColorAt(9, 0)
	if l0[0] > 0.05 {
		t.Errorf("left end not black: %v", l0)
	}
	// Pixel center 9.5 puts param at 0.95, so linear value is near but not 1
	if l1[0] < 0.85 {
		t.Errorf("right end not white: %v", l1)
	}
	// Alpha is opaque across the ramp
	if l0[3] != 1 || l1[3] != 1 {
		t.Errorf("expected opaque, got %v %v", l0[3], l1[3])
	}
}

func TestLinearMidpointMonotonic(t *testing.T) {
	g := New(Linear, Pad, path.Point{X: 0, Y: 0}, path.Point{X: 100, Y: 0}, []Stop{
		{Pos: 0, Color: black(), Opacity: 1},
		{Pos: 1, Color: white(), Opacity: 1},
	})
	prev := float32(-1)
	for x := 0; x < 100; x++ {
		v := g.ColorAt(x, 0)[0]
		if v < prev-1e-6 {
			t.Fatalf("gradient not monotonic at x=%d: %v < %v", x, v, prev)
		}
		prev = v
	}
}

func TestPerStopOpacity(t *testing.T) {
	g := New(Linear, Pad, path.Point{X: 0, Y: 0}, path.Point{X: 10, Y: 0}, []Stop{
		{Pos: 0, Color: white(), Opacity: 1},
		{Pos: 1, Color: white(), Opacity: 0},
	})
	a0 := g.ColorAt(0, 0)[3]
	a1 := g.ColorAt(9, 0)[3]
	if a0 < 0.9 {
		t.Errorf("start opacity = %v want ~1", a0)
	}
	if a1 > 0.1 {
		t.Errorf("end opacity = %v want ~0", a1)
	}
	// Premultiplied color must track the fading alpha
	c1 := g.ColorAt(9, 0)
	if c1[0] > c1[3]+1e-6 {
		t.Error("premultiplication invariant violated at faded end")
	}
}

func TestRadialConcentric(t *testing.T) {
	g := New(Radial, Pad, path.Point{X: 50, Y: 50}, path.Point{X: 50, Y: 0}, []Stop{
		{Pos: 0, Color: white(), Opacity: 1},
		{Pos: 1, Color: black(), Opacity: 1},
	})
	center := g.ColorAt(50, 50)[0]
	edge := g.ColorAt(50, 5)[0]
	if center < 0.9 {
		t.Errorf("center should be white: %v", center)
	}
	if edge > 0.1 {
		t.Errorf("radius edge should be black: %v", edge)
	}
}

func TestRepeatSpread(t *testing.T) {
	g := New(Linear, Repeat, path.Point{X: 0, Y: 0}, path.Point{X: 10, Y: 0}, []Stop{
		{Pos: 0, Color: black(), Opacity: 1},
		{Pos: 1, Color: white(), Opacity: 1},
	})
	// Parameter wraps, so x=0 and x=10 land on the same phase
	a := g.ColorAt(0, 0)[0]
	b := g.ColorAt(10, 0)[0]
	if math.Abs(float64(a-b)) > 1e-3 {
		t.Errorf("repeat phase mismatch: %v vs %v", a, b)
	}
}

func TestGradientAsPaint(t *testing.T) {
	// A gradient must be usable as a fill paint for a path
	var _ path.Paint = New(Linear, Pad, path.Point{}, path.Point{X: 1}, nil)
	buf := raster.MustNewBuffer(10, 10)
	g := New(Linear, Pad, path.Point{X: 0, Y: 0}, path.Point{X: 10, Y: 0}, []Stop{
		{Pos: 0, Color: black(), Opacity: 1},
		{Pos: 1, Color: white(), Opacity: 1},
	})
	p := path.New().Rect(0, 0, 10, 10)
	path.Fill(buf, p, path.NonZero, g)
	_, _, _, a := buf.At(5, 5)
	if a < 0.9 {
		t.Errorf("gradient fill should be opaque: %v", a)
	}
}
