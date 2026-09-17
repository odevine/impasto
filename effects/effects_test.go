package effects

import (
	"image/color"
	"testing"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/raster"
)

// solidSquare returns a buffer with an opaque white square in the middle
func solidSquare() *raster.Buffer {
	b := raster.MustNewBuffer(60, 60)
	for y := 20; y < 40; y++ {
		for x := 20; x < 40; x++ {
			b.Set(x, y, 1, 1, 1, 1)
		}
	}
	return b
}

func alphaAt(b *raster.Buffer, x, y int) float32 {
	_, _, _, a := b.At(x, y)
	return a
}

func TestDropShadowPlacement(t *testing.T) {
	layer := solidSquare()
	d := &DropShadow{Color: color.Black, Opacity: 1, Angle: 0, Distance: 6, BlurRadius: 3}
	rs := d.Render(layer)
	if len(rs) != 1 || !rs[0].Behind {
		t.Fatal("drop shadow should be a single behind contribution")
	}
	if rs[0].Mode != blend.Multiply {
		t.Errorf("default drop shadow mode = %v want Multiply", rs[0].Mode)
	}
	// The shadow shifts right, so there is coverage past the square's right edge
	if alphaAt(rs[0].Pixels, 44, 30) < 0.05 {
		t.Errorf("expected shadow to the right of the square, got %v", alphaAt(rs[0].Pixels, 44, 30))
	}
	// Far left of the square there is none
	if alphaAt(rs[0].Pixels, 10, 30) > 0.01 {
		t.Errorf("did not expect shadow far left, got %v", alphaAt(rs[0].Pixels, 10, 30))
	}
}

func TestInnerShadowClipped(t *testing.T) {
	layer := solidSquare()
	s := &InnerShadow{Color: color.Black, Opacity: 1, Angle: 0, Distance: 5, BlurRadius: 3}
	rs := s.Render(layer)
	if rs[0].Behind {
		t.Fatal("inner shadow should be in front")
	}
	// Nothing outside the square, it is clipped to the layer
	if alphaAt(rs[0].Pixels, 5, 5) > 0.001 {
		t.Errorf("inner shadow leaked outside layer: %v", alphaAt(rs[0].Pixels, 5, 5))
	}
	// Something along the inner left edge where the offset reveals the rim
	if alphaAt(rs[0].Pixels, 21, 30) < 0.02 {
		t.Errorf("expected inner shadow near the rim, got %v", alphaAt(rs[0].Pixels, 21, 30))
	}
}

func TestOuterGlowBehindAndSpreads(t *testing.T) {
	layer := solidSquare()
	g := &OuterGlow{Color: color.White, Opacity: 1, BlurRadius: 4, Spread: 2}
	rs := g.Render(layer)
	if !rs[0].Behind || rs[0].Mode != blend.Screen {
		t.Fatal("outer glow should be behind with Screen mode")
	}
	// Glow reaches just outside the square edge
	if alphaAt(rs[0].Pixels, 42, 30) < 0.02 {
		t.Errorf("expected glow outside the square, got %v", alphaAt(rs[0].Pixels, 42, 30))
	}
}

func TestColorOverlayClipped(t *testing.T) {
	layer := solidSquare()
	o := &ColorOverlay{Color: color.NRGBA{255, 0, 0, 255}, Opacity: 1}
	rs := o.Render(layer)
	// Inside the square it is opaque red
	r, _, _, a := rs[0].Pixels.At(30, 30)
	if a < 0.99 || r < 0.99 {
		t.Errorf("overlay inside = r%v a%v want opaque red", r, a)
	}
	// Outside is clear
	if alphaAt(rs[0].Pixels, 5, 5) > 0.001 {
		t.Error("overlay leaked outside layer")
	}
}

func TestStrokeOutsideRing(t *testing.T) {
	layer := solidSquare()
	s := &Stroke{Width: 3, Color: color.NRGBA{0, 0, 255, 255}, Alignment: StrokeOutside, Opacity: 1}
	rs := s.Render(layer)
	// The ring sits just outside the edge, not deep inside
	if alphaAt(rs[0].Pixels, 41, 30) < 0.5 {
		t.Errorf("expected stroke just outside edge, got %v", alphaAt(rs[0].Pixels, 41, 30))
	}
	if alphaAt(rs[0].Pixels, 30, 30) > 0.01 {
		t.Errorf("outside stroke should not paint the interior, got %v", alphaAt(rs[0].Pixels, 30, 30))
	}
}

func TestStrokeInsideRing(t *testing.T) {
	layer := solidSquare()
	s := &Stroke{Width: 3, Color: color.NRGBA{0, 0, 255, 255}, Alignment: StrokeInside, Opacity: 1}
	rs := s.Render(layer)
	// Inside stroke never leaves the shape
	if alphaAt(rs[0].Pixels, 42, 30) > 0.01 {
		t.Errorf("inside stroke leaked outside, got %v", alphaAt(rs[0].Pixels, 42, 30))
	}
	if alphaAt(rs[0].Pixels, 21, 30) < 0.5 {
		t.Errorf("expected stroke on inner edge, got %v", alphaAt(rs[0].Pixels, 21, 30))
	}
}

func TestBevelTwoContributions(t *testing.T) {
	layer := solidSquare()
	b := &BevelEmboss{Depth: 2, Size: 3, Angle: 2.36, Altitude: 0.6}
	rs := b.Render(layer)
	if len(rs) != 2 {
		t.Fatalf("bevel should return highlight and shadow, got %d", len(rs))
	}
	if rs[0].Mode != blend.Screen || rs[1].Mode != blend.Multiply {
		t.Errorf("bevel modes = %v,%v want Screen,Multiply", rs[0].Mode, rs[1].Mode)
	}
	// Both should stay inside the layer
	if alphaAt(rs[0].Pixels, 2, 2) > 0.001 || alphaAt(rs[1].Pixels, 2, 2) > 0.001 {
		t.Error("bevel leaked outside layer")
	}
}

func TestSortOrder(t *testing.T) {
	list := []Effect{
		&Stroke{},
		&DropShadow{},
		&BevelEmboss{},
		&OuterGlow{},
	}
	got := Sort(list)
	// Drop shadow first, bevel last
	if _, ok := got[0].(*DropShadow); !ok {
		t.Errorf("expected DropShadow first, got %T", got[0])
	}
	if _, ok := got[len(got)-1].(*BevelEmboss); !ok {
		t.Errorf("expected BevelEmboss last, got %T", got[len(got)-1])
	}
}
