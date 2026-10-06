package effects

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/odevine/impasto/raster"
)

// TestDropShadowStaysWithinBleed renders a shadow on a buffer much larger than
// the content, so nothing is clipped, and requires every visible pixel to sit
// within Bleed of the content's rectangle. A Bleed that is too small would clip
// a shadow in canvas without any error, so the sweep covers distances, blurs,
// angles and chokes up to and beyond the sizes in normal use
func TestDropShadowStaysWithinBleed(t *testing.T) {
	const size, cw, ch = 200, 20, 16
	content := image.Rect(90, 90, 90+cw, 90+ch)
	layer := raster.MustNewBuffer(size, size)
	for y := content.Min.Y; y < content.Max.Y; y++ {
		for x := content.Min.X; x < content.Max.X; x++ {
			layer.Set(x, y, 1, 1, 1, 1)
		}
	}
	for _, dist := range []float32{0, 0.4, 1, 7.5, 20} {
		for _, blurR := range []float32{0, 0.3, 1, 2.5, 5, 12} {
			for _, choke := range []float32{0, 3} {
				for _, ang := range []float32{0, 0.6, 1.57, 2.5, 3.14, 4.2, 4.71, 5.9} {
					t.Run(fmt.Sprintf("d%g/b%g/c%g/a%g", dist, blurR, choke, ang), func(t *testing.T) {
						d := &DropShadow{Color: color.Black, Opacity: 1, Angle: ang, Distance: dist, BlurRadius: blurR, Choke: choke}
						px := d.Render(layer)[0].Pixels
						reach := content.Inset(-d.Bleed())
						seen := false
						for y := 0; y < size; y++ {
							for x := 0; x < size; x++ {
								_, _, _, a := px.At(x, y)
								if a > 1e-5 && !image.Pt(x, y).In(reach) {
									t.Fatalf("shadow alpha %g at (%d,%d), outside content %v grown by Bleed %d", a, x, y, content, d.Bleed())
								}
								if a > 1e-3 && !image.Pt(x, y).In(content) {
									seen = true
								}
							}
						}
						if choke == 0 && dist+blurR > 1 && !seen {
							t.Fatal("no shadow was drawn outside the content, the sweep is not testing anything")
						}
					})
				}
			}
		}
	}
}

func TestMaxBleed(t *testing.T) {
	if n, ok := MaxBleed(nil); n != 0 || !ok {
		t.Fatalf("no effects: got %d, %v", n, ok)
	}
	small := &DropShadow{Distance: 2}
	big := &DropShadow{Distance: 10, BlurRadius: 4}
	if n, ok := MaxBleed([]Effect{small, &ColorOverlay{}, big}); !ok || n != big.Bleed() {
		t.Fatalf("got %d, %v, want %d, true", n, ok, big.Bleed())
	}
	if _, ok := MaxBleed([]Effect{small, &BevelEmboss{}}); ok {
		t.Fatal("an effect without Bleed must make the set unbounded")
	}
}

func TestBleedValues(t *testing.T) {
	d := &DropShadow{Distance: 6, BlurRadius: 2, Choke: 1}
	if got, want := d.Bleed(), 6+6+1+2; got != want {
		t.Fatalf("DropShadow.Bleed = %d, want %d", got, want)
	}
	hard := &DropShadow{Distance: 4.2}
	if got, want := hard.Bleed(), 5+2; got != want {
		t.Fatalf("hard shadow Bleed = %d, want %d (ceil(Distance) + 2)", got, want)
	}
	if (&ColorOverlay{}).Bleed() != 0 {
		t.Fatal("ColorOverlay.Bleed must be 0")
	}
}
