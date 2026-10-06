package canvas

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/gradient"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

// blob is a w by h buffer holding an ellipse with soft, varying color so that
// any shift, crop or missed pixel changes the render
func blob(w, h int, seed float32) *raster.Buffer {
	b := raster.MustNewBuffer(w, h)
	cx, cy := float32(w)/2, float32(h)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := (float32(x)+0.5-cx)/cx, (float32(y)+0.5-cy)/cy
			d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
			if d >= 1 {
				continue
			}
			a := 0.3 + 0.7*(1-d)
			r := (0.2 + 0.6*float32((x*7+y*3)%11)/11 + seed) * a
			g := (0.1 + 0.5*float32((x*5+y*9)%13)/13) * a
			bl := (0.3 + 0.4*float32((x+y*11)%7)/7) * a
			b.Set(x, y, r, g, bl, a)
		}
	}
	return b
}

// shadowNoise is the float difference tolerated for a layer with a bounded
// shadow. The shadow is computed on a buffer with a different origin and extent
// than the document, so the offset's bilinear weights round differently, and the
// blur's sliding window sum leaves ulp-sized noise in different places. It is
// far below one 8-bit step, and the 8-bit output must still match exactly
const shadowNoise = 1e-6

// renderPair renders the same scene twice, once with every layer placed into a
// document-sized buffer and once with the content bounded at an origin, and
// requires the two results to match exactly
func renderPair(t *testing.T, docW, docH int, build func(content func(b *raster.Buffer, x, y int) *Layer) []Node) {
	t.Helper()
	renderPairTol(t, 0, docW, docH, build)
}

// renderPairTol is renderPair allowing each float to differ by up to tol, and
// also requiring the 8-bit renders to be identical when tol is set
func renderPairTol(t *testing.T, tol float32, docW, docH int, build func(content func(b *raster.Buffer, x, y int) *Layer) []Node) {
	t.Helper()
	placed := func(b *raster.Buffer, x, y int) *Layer {
		return &Layer{Content: Place(docW, docH, b, x, y)}
	}
	bounded := func(b *raster.Buffer, x, y int) *Layer {
		return &Layer{Content: b, Origin: image.Pt(x, y)}
	}
	render := func(content func(b *raster.Buffer, x, y int) *Layer) *raster.Buffer {
		layers := append([]Node{&Layer{Content: fill(docW, docH, 0.1, 0.2, 0.3, 1)}}, build(content)...)
		return MustRender(&Document{Width: docW, Height: docH, Root: Group{PassThrough: true, Layers: layers}})
	}
	want, got := render(placed), render(bounded)
	requireSame(t, tol, want, got)
	if tol > 0 && !bytes.Equal(want.ToImage(8).(*image.NRGBA).Pix, got.ToImage(8).(*image.NRGBA).Pix) {
		t.Fatal("8-bit output differs")
	}
}

// requireSame fails unless every float of two buffers is within tol
func requireSame(t *testing.T, tol float32, want, got *raster.Buffer) {
	t.Helper()
	if !want.SameSize(got) {
		t.Fatalf("size %dx%d, want %dx%d", got.Width, got.Height, want.Width, want.Height)
	}
	var worst float32
	first := -1
	for i := range want.Pix {
		d := float32(math.Abs(float64(want.Pix[i] - got.Pix[i])))
		if d > tol {
			if first < 0 {
				first = i
			}
			if d > worst {
				worst = d
			}
		}
	}
	if first >= 0 {
		px := first / 4
		t.Fatalf("buffers differ, first at (%d,%d) channel %d: got %v want %v, largest difference %g",
			px%want.Width, px/want.Width, first%4, got.Pix[first], want.Pix[first], worst)
	}
}

const docW, docH = 80, 64

// positions covers a layer inside the document, at each edge and corner, partly
// outside on each side, larger than the document, and fully outside
var positions = []struct {
	name string
	x, y int
}{
	{"inside", 20, 15},
	{"top left", 0, 0},
	{"top right", docW - 24, 0},
	{"bottom left", 0, docH - 20},
	{"bottom right", docW - 24, docH - 20},
	{"left edge", 0, 22},
	{"right edge", docW - 24, 22},
	{"top edge", 30, 0},
	{"bottom edge", 30, docH - 20},
	{"past left", -10, 20},
	{"past top", 30, -8},
	{"past right", docW - 14, 20},
	{"past bottom", 30, docH - 9},
	{"past top left corner", -11, -7},
	{"past bottom right corner", docW - 13, docH - 6},
	{"fully outside", docW + 5, 10},
	{"fully above", 10, -40},
}

func TestBoundedMatchesPlaced(t *testing.T) {
	for _, p := range positions {
		t.Run(p.name, func(t *testing.T) {
			renderPair(t, docW, docH, func(c func(*raster.Buffer, int, int) *Layer) []Node {
				return []Node{c(blob(24, 20, 0), p.x, p.y)}
			})
		})
	}
}

func TestBoundedLargerThanDocument(t *testing.T) {
	renderPair(t, docW, docH, func(c func(*raster.Buffer, int, int) *Layer) []Node {
		return []Node{c(blob(docW+30, docH+20, 0), -12, -9)}
	})
}

func TestBoundedModesAndOpacity(t *testing.T) {
	for _, m := range []blend.Mode{blend.Normal, blend.Multiply, blend.Overlay, blend.Hue} {
		t.Run(fmt.Sprint(m), func(t *testing.T) {
			renderPair(t, docW, docH, func(c func(*raster.Buffer, int, int) *Layer) []Node {
				l := c(blob(30, 22, 0.1), 25, 18)
				l.Mode, l.Opacity = m, 0.6
				return []Node{l}
			})
		})
	}
}

// TestBoundedMask checks that a mask is read in document coordinates, so a
// bounded layer is masked the way the same pixels would be in place
func TestBoundedMask(t *testing.T) {
	m := mask.FuncMask(func(x, y int) float32 {
		return float32((x*3+y*5)%17) / 16
	})
	for _, p := range positions {
		t.Run(p.name, func(t *testing.T) {
			renderPair(t, docW, docH, func(c func(*raster.Buffer, int, int) *Layer) []Node {
				l := c(blob(24, 20, 0), p.x, p.y)
				l.Mask = m
				return []Node{l}
			})
		})
	}
}

// TestBoundedEffects covers a layer at the document edges and in the middle with
// each kind of effect. The bounded ones run on a padded buffer, the rest fall
// back to the whole document, and both must match the placed layer
func TestBoundedEffects(t *testing.T) {
	grad := gradient.New(gradient.Linear, gradient.Pad, path.Point{}, path.Point{X: docW, Y: docH},
		[]gradient.Stop{{Pos: 0, Color: color.Black, Opacity: 1}, {Pos: 1, Color: color.White, Opacity: 1}})
	cases := map[string][]effects.Effect{
		"hard shadow":      {&effects.DropShadow{Color: color.Black, Opacity: 0.8, Angle: 0.8, Distance: 6}},
		"soft shadow":      {&effects.DropShadow{Color: color.Black, Opacity: 0.8, Angle: 2.4, Distance: 5, BlurRadius: 4}},
		"choked shadow":    {&effects.DropShadow{Color: color.Black, Opacity: 0.8, Angle: 5.5, Distance: 3, BlurRadius: 2, Choke: 2}},
		"color overlay":    {&effects.ColorOverlay{Color: color.RGBA{R: 255, A: 255}, Opacity: 0.5}},
		"shadow + overlay": {&effects.DropShadow{Color: color.Black, Opacity: 0.8, Angle: 1.2, Distance: 4, BlurRadius: 3}, &effects.ColorOverlay{Color: color.White, Opacity: 0.3}},
		"gradient overlay": {&effects.GradientOverlay{Gradient: grad}},
		"shadow + gradient": {
			&effects.DropShadow{Color: color.Black, Opacity: 0.8, Angle: 1.2, Distance: 4, BlurRadius: 3},
			&effects.GradientOverlay{Gradient: grad},
		},
		"stroke":       {&effects.Stroke{Width: 3, Color: color.White, Alignment: effects.StrokeOutside, Opacity: 1}},
		"inner shadow": {&effects.InnerShadow{Color: color.Black, Opacity: 0.6, Angle: 1, Distance: 3, BlurRadius: 2}},
		"bevel":        {&effects.BevelEmboss{}},
	}
	places := []struct {
		name string
		x, y int
	}{{"inside", 25, 18}, {"top left", 0, 0}, {"bottom right", docW - 24, docH - 20}, {"past left", -9, 20}}
	// Layers that fall back to the document must match exactly
	shadowed := map[string]bool{"hard shadow": true, "soft shadow": true, "choked shadow": true, "shadow + overlay": true}
	for name, list := range cases {
		var tol float32
		if shadowed[name] {
			tol = shadowNoise
		}
		for _, p := range places {
			t.Run(name+"/"+p.name, func(t *testing.T) {
				renderPairTol(t, tol, docW, docH, func(c func(*raster.Buffer, int, int) *Layer) []Node {
					l := c(blob(24, 20, 0), p.x, p.y)
					l.Effects = list
					l.Opacity = 0.9
					return []Node{l}
				})
			})
		}
	}
}

// TestBoundedShadowAtBleedLimit sweeps shadows whose reach is exactly what
// Bleed reports, against a layer sitting inside the document so the padding is
// not clipped by the document edge
func TestBoundedShadowAtBleedLimit(t *testing.T) {
	for _, dist := range []float32{0, 1, 4.5, 9, 17} {
		for _, blurR := range []float32{0, 0.5, 1, 2.5, 6} {
			for _, ang := range []float32{0, 0.7, 1.57, 3.14, 4.4, 5.9} {
				name := fmt.Sprintf("d%g/b%g/a%g", dist, blurR, ang)
				t.Run(name, func(t *testing.T) {
					renderPairTol(t, shadowNoise, 120, 100, func(c func(*raster.Buffer, int, int) *Layer) []Node {
						l := c(blob(22, 18, 0), 50, 40)
						l.Effects = []effects.Effect{&effects.DropShadow{Color: color.Black, Opacity: 0.9, Angle: ang, Distance: dist, BlurRadius: blurR}}
						return []Node{l}
					})
				})
			}
		}
	}
}

func TestBoundedClipToBelow(t *testing.T) {
	// Each case places the clipped layer's rect relative to the base's rect at
	// (20,15) size 30x24, so the two overlap partly, fully, or not at all
	cases := []struct {
		name string
		x, y int
		w, h int
	}{
		{"partial overlap", 35, 25, 24, 20},
		{"clipped inside base", 26, 20, 12, 10},
		{"base inside clipped", 10, 8, 56, 44},
		{"no overlap", 60, 50, 14, 10},
		{"touching edges", 50, 15, 10, 10},
		{"same rect", 20, 15, 30, 24},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			renderPair(t, docW, docH, func(c func(*raster.Buffer, int, int) *Layer) []Node {
				base := c(blob(30, 24, 0), 20, 15)
				clipped := c(blob(tc.w, tc.h, 0.2), tc.x, tc.y)
				clipped.ClipToBelow = true
				clipped.Mode = blend.Overlay
				return []Node{base, clipped}
			})
		})
	}
}

// TestBoundedClipChain checks a second clipped layer still attaches to the
// original base, and a clipped layer whose base is fully outside the document
func TestBoundedClipChain(t *testing.T) {
	t.Run("two clipped", func(t *testing.T) {
		renderPair(t, docW, docH, func(c func(*raster.Buffer, int, int) *Layer) []Node {
			base := c(blob(30, 24, 0), 20, 15)
			a := c(blob(20, 20, 0.1), 30, 20)
			b := c(blob(20, 20, 0.2), 10, 10)
			a.ClipToBelow, b.ClipToBelow = true, true
			return []Node{base, a, b}
		})
	})
	t.Run("base outside document", func(t *testing.T) {
		renderPair(t, docW, docH, func(c func(*raster.Buffer, int, int) *Layer) []Node {
			base := c(blob(30, 24, 0), docW+10, 15)
			clipped := c(blob(20, 20, 0.2), 30, 20)
			clipped.ClipToBelow = true
			return []Node{base, clipped}
		})
	})
	t.Run("masked base", func(t *testing.T) {
		m := mask.FuncMask(func(x, y int) float32 { return float32((x+y)%5) / 4 })
		renderPair(t, docW, docH, func(c func(*raster.Buffer, int, int) *Layer) []Node {
			base := c(blob(30, 24, 0), 20, 15)
			base.Mask = m
			clipped := c(blob(30, 24, 0.2), 28, 22)
			clipped.ClipToBelow = true
			return []Node{base, clipped}
		})
	})
}

func TestBoundedInGroups(t *testing.T) {
	for _, pass := range []bool{true, false} {
		t.Run(fmt.Sprint("passthrough ", pass), func(t *testing.T) {
			renderPairTol(t, shadowNoise, docW, docH, func(c func(*raster.Buffer, int, int) *Layer) []Node {
				a := c(blob(24, 20, 0), 5, 4)
				b := c(blob(24, 20, 0.2), docW-20, docH-14)
				b.Effects = []effects.Effect{&effects.DropShadow{Color: color.Black, Opacity: 0.7, Angle: 0.8, Distance: 5, BlurRadius: 2}}
				return []Node{&Group{Layers: []Node{a, b}, PassThrough: pass, Opacity: 0.8}}
			})
		})
	}
}
