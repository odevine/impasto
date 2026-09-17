package canvas

import (
	"math"
	"testing"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

func fill(w, h int, r, g, b, a float32) *raster.Buffer {
	buf := raster.MustNewBuffer(w, h)
	for i := 0; i < len(buf.Pix); i += 4 {
		buf.Pix[i], buf.Pix[i+1], buf.Pix[i+2], buf.Pix[i+3] = r, g, b, a
	}
	return buf
}

func doc(w, h int, nodes ...Node) *Document {
	return &Document{Width: w, Height: h, Root: Group{PassThrough: true, Opacity: 1, Layers: nodes}}
}

func TestTopLayerCoversBottom(t *testing.T) {
	bottom := &Layer{Content: fill(8, 8, 0.9, 0, 0, 1), Opacity: 1, Mode: blend.Normal}
	top := &Layer{Content: fill(8, 8, 0, 0, 0.9, 1), Opacity: 1, Mode: blend.Normal}
	out := MustRender(doc(8, 8, bottom, top))
	r, _, b, a := out.At(4, 4)
	if a < 0.99 || b < 0.89 || r > 0.01 {
		t.Errorf("top should cover bottom: r%v b%v a%v", r, b, a)
	}
}

func TestMultiplyBlend(t *testing.T) {
	bottom := &Layer{Content: fill(4, 4, 0.8, 0.8, 0.8, 1), Opacity: 1, Mode: blend.Normal}
	top := &Layer{Content: fill(4, 4, 0.5, 0.5, 0.5, 1), Opacity: 1, Mode: blend.Multiply}
	out := MustRender(doc(4, 4, bottom, top))
	r, _, _, _ := out.At(2, 2)
	if math.Abs(float64(r-0.4)) > 1e-4 {
		t.Errorf("multiply result = %v want 0.4", r)
	}
}

func TestVectorMask(t *testing.T) {
	bottom := &Layer{Content: fill(20, 20, 0.9, 0, 0, 1), Opacity: 1}
	// Top blue layer masked to the left half only
	p := path.New().Rect(0, 0, 10, 20)
	top := &Layer{
		Content: fill(20, 20, 0, 0, 0.9, 1),
		Opacity: 1,
		Mask:    mask.NewVectorMask(p, 20, 20, path.NonZero),
	}
	out := MustRender(doc(20, 20, bottom, top))
	// Left half shows blue
	_, _, bl, _ := out.At(5, 10)
	if bl < 0.89 {
		t.Errorf("masked-in region should be blue: %v", bl)
	}
	// Right half shows the red bottom
	r, _, _, _ := out.At(15, 10)
	if r < 0.89 {
		t.Errorf("masked-out region should be red: %v", r)
	}
}

func TestContentImmutable(t *testing.T) {
	content := fill(8, 8, 0.5, 0.5, 0.5, 1)
	before := content.Clone()
	top := &Layer{
		Content: content,
		Opacity: 0.5,
		Mask:    mask.NewRasterMask(halfCoverage(64), 8, 8),
	}
	MustRender(doc(8, 8, top))
	for i := range content.Pix {
		if content.Pix[i] != before.Pix[i] {
			t.Fatalf("layer content mutated at %d", i)
		}
	}
}

func TestDeterministicRender(t *testing.T) {
	build := func() *Document {
		return doc(64, 64,
			&Layer{Content: fill(64, 64, 0.3, 0.4, 0.5, 1), Opacity: 1},
			&Layer{Content: fill(64, 64, 0.6, 0.2, 0.1, 0.7), Opacity: 0.8, Mode: blend.Overlay},
		)
	}
	a := MustRender(build())
	b := MustRender(build())
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			t.Fatalf("render not deterministic at %d", i)
		}
	}
}

func TestClipToBelow(t *testing.T) {
	// Base occupies the left half, clipped layer is full but should only show
	// where the base is present
	base := &Layer{Content: leftHalf(20, 20, 0.9, 0, 0), Opacity: 1}
	clip := &Layer{Content: fill(20, 20, 0, 0.9, 0, 1), Opacity: 1, ClipToBelow: true}
	out := MustRender(doc(20, 20, base, clip))
	// Left half is green (clip shows over base)
	_, g, _, a := out.At(5, 10)
	if g < 0.89 || a < 0.99 {
		t.Errorf("clip should show over base: g%v a%v", g, a)
	}
	// Right half is empty, the clip was masked away with the absent base
	if _, _, _, a := out.At(15, 10); a > 0.01 {
		t.Errorf("clip should be hidden where base is absent: a%v", a)
	}
}

func TestIsolatedGroupOpacity(t *testing.T) {
	inner := Group{
		PassThrough: false,
		Opacity:     0.5,
		Layers: []Node{
			&Layer{Content: fill(8, 8, 1, 1, 1, 1), Opacity: 1},
		},
	}
	d := &Document{Width: 8, Height: 8, Root: Group{PassThrough: true, Opacity: 1, Layers: []Node{&inner}}}
	out := MustRender(d)
	_, _, _, a := out.At(4, 4)
	if math.Abs(float64(a-0.5)) > 1e-4 {
		t.Errorf("isolated group opacity = %v want 0.5", a)
	}
}

func TestPlaceInto(t *testing.T) {
	src := fill(4, 4, 1, 0, 0, 1)
	placed := Place(20, 20, src, 8, 6)
	if _, _, _, a := placed.At(9, 7); a < 0.99 {
		t.Error("expected placed pixel to be opaque")
	}
	if _, _, _, a := placed.At(0, 0); a != 0 {
		t.Error("expected area outside placement to be transparent")
	}
}

func halfCoverage(n int) []float32 {
	c := make([]float32, n)
	for i := range c {
		c[i] = 0.5
	}
	return c
}

func leftHalf(w, h int, r, g, b float32) *raster.Buffer {
	buf := raster.MustNewBuffer(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w/2; x++ {
			buf.Set(x, y, r, g, b, 1)
		}
	}
	return buf
}
