package canvas

import (
	"image"
	"image/color"
	"slices"
	"testing"

	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/raster"
)

// sizeSpy is an effect that records the size of the buffer it is given
type sizeSpy struct {
	bleed   int
	bounded bool
	w, h    int
}

func (s *sizeSpy) Render(layer *raster.Buffer) []effects.Rendered {
	s.w, s.h = layer.Width, layer.Height
	return nil
}

type boundedSpy struct{ sizeSpy }

func (s *boundedSpy) Bleed() int { return s.bleed }

func TestEffectBufferSize(t *testing.T) {
	render := func(l *Layer) {
		MustRender(&Document{Width: docW, Height: docH, Root: Group{PassThrough: true, Layers: []Node{l}}})
	}
	content := blob(20, 14, 0)

	t.Run("bounded gets the content plus its bleed", func(t *testing.T) {
		spy := &boundedSpy{sizeSpy{bleed: 5}}
		render(&Layer{Content: content, Origin: image.Pt(30, 25), Effects: []effects.Effect{spy}})
		if spy.w != 20+10 || spy.h != 14+10 {
			t.Fatalf("effect saw %dx%d, want %dx%d", spy.w, spy.h, 30, 24)
		}
	})
	t.Run("bounded padding stops at the document", func(t *testing.T) {
		spy := &boundedSpy{sizeSpy{bleed: 5}}
		render(&Layer{Content: content, Origin: image.Pt(2, 1), Effects: []effects.Effect{spy}})
		if spy.w != 2+20+5 || spy.h != 1+14+5 {
			t.Fatalf("effect saw %dx%d, want %dx%d", spy.w, spy.h, 27, 20)
		}
	})
	t.Run("zero bleed uses the content as is", func(t *testing.T) {
		spy := &boundedSpy{}
		render(&Layer{Content: content, Origin: image.Pt(30, 25), Effects: []effects.Effect{spy}})
		if spy.w != 20 || spy.h != 14 {
			t.Fatalf("effect saw %dx%d, want 20x14", spy.w, spy.h)
		}
	})
	t.Run("an effect without Bleed gets the whole document", func(t *testing.T) {
		spy := &sizeSpy{}
		bounded := &boundedSpy{sizeSpy{bleed: 3}}
		render(&Layer{Content: content, Origin: image.Pt(30, 25), Effects: []effects.Effect{bounded, spy}})
		if spy.w != docW || spy.h != docH {
			t.Fatalf("effect saw %dx%d, want the %dx%d document", spy.w, spy.h, docW, docH)
		}
		if bounded.w != docW || bounded.h != docH {
			t.Fatalf("a bounded effect sharing the layer saw %dx%d, want the document", bounded.w, bounded.h)
		}
	})
	t.Run("content cut by the document is cropped first", func(t *testing.T) {
		spy := &boundedSpy{}
		render(&Layer{Content: content, Origin: image.Pt(docW-8, -4), Effects: []effects.Effect{spy}})
		if spy.w != 8 || spy.h != 10 {
			t.Fatalf("effect saw %dx%d, want 8x10", spy.w, spy.h)
		}
	})
}

// TestBoundedLeavesContentUntouched pins that a bounded layer's buffer is never
// written to, including when it is masked, clipped or cut by the document edge
func TestBoundedLeavesContentUntouched(t *testing.T) {
	m := mask.FuncMask(func(x, y int) float32 { return 0.5 })
	mk := func(x, y int) *Layer { return &Layer{Content: blob(24, 20, 0), Origin: image.Pt(x, y)} }
	base := mk(10, 10)
	masked := mk(20, 20)
	masked.Mask = m
	clipped := mk(14, 12)
	clipped.ClipToBelow = true
	cut := mk(-6, docH-9)
	cut.Effects = []effects.Effect{&effects.DropShadow{Color: color.Black, Opacity: 1, Distance: 4, BlurRadius: 2}}
	layers := []*Layer{base, masked, clipped, cut}

	before := make([][]float32, len(layers))
	nodes := make([]Node, len(layers))
	for i, l := range layers {
		before[i] = slices.Clone(l.Content.Pix)
		nodes[i] = l
	}
	MustRender(&Document{Width: docW, Height: docH, Root: Group{PassThrough: true, Layers: nodes}})
	for i, l := range layers {
		if !slices.Equal(before[i], l.Content.Pix) {
			t.Errorf("layer %d content was modified by Render", i)
		}
	}
}
