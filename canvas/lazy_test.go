package canvas

import (
	"errors"
	"image"
	"image/color"
	"slices"
	"strings"
	"testing"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/raster"
)

// loader returns a Load that hands out a copy of b at origin and counts calls
func loader(b *raster.Buffer, at image.Point, calls *int) func() (*raster.Buffer, image.Point, error) {
	return func() (*raster.Buffer, image.Point, error) {
		*calls++
		return b.Clone(), at, nil
	}
}

func lazyDoc(layers ...Node) *Document {
	return &Document{Width: docW, Height: docH, Root: Group{PassThrough: true, Layers: layers}}
}

func TestLoadErrorStopsRender(t *testing.T) {
	boom := errors.New("decode failed")
	var first, last int
	doc := lazyDoc(
		&Layer{Load: loader(blob(10, 10, 0), image.Pt(5, 5), &first)},
		&Layer{Load: func() (*raster.Buffer, image.Point, error) { return nil, image.Point{}, boom }},
		&Layer{Load: loader(blob(10, 10, 0), image.Pt(5, 5), &last)},
	)
	out, err := Render(doc)
	if err == nil {
		t.Fatal("expected an error")
	}
	if out != nil {
		t.Error("a failed render must not return a buffer")
	}
	if !errors.Is(err, boom) {
		t.Errorf("error %q does not wrap the Load error", err)
	}
	if !strings.Contains(err.Error(), "layer 1") {
		t.Errorf("error %q does not name the layer's position", err)
	}
	if first != 1 || last != 0 {
		t.Errorf("Load calls before the failure %d and after %d, want 1 and 0", first, last)
	}
}

func TestLoadErrorNamesNestedPosition(t *testing.T) {
	boom := errors.New("missing file")
	failing := &Layer{Load: func() (*raster.Buffer, image.Point, error) { return nil, image.Point{}, boom }}
	for _, isolated := range []bool{false, true} {
		doc := lazyDoc(
			&Layer{Content: blob(8, 8, 0)},
			&Group{PassThrough: !isolated, Layers: []Node{&Layer{Content: blob(8, 8, 0)}, failing}},
		)
		_, err := Render(doc)
		if !errors.Is(err, boom) {
			t.Fatalf("isolated=%v: error %v does not wrap the Load error", isolated, err)
		}
		if !strings.Contains(err.Error(), "group 1: layer 1") {
			t.Errorf("isolated=%v: error %q does not name group 1, layer 1", isolated, err)
		}
	}
}

func TestMustRenderPanicsOnLoadError(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic")
		}
	}()
	MustRender(lazyDoc(&Layer{Load: func() (*raster.Buffer, image.Point, error) {
		return nil, image.Point{}, errors.New("boom")
	}}))
}

func TestLoadMalformedBufferIsAnError(t *testing.T) {
	bad := &raster.Buffer{Pix: make([]float32, 8), Width: 10, Height: 10}
	_, err := Render(lazyDoc(&Layer{Load: func() (*raster.Buffer, image.Point, error) { return bad, image.Point{}, nil }}))
	if err == nil {
		t.Fatal("expected an error for a buffer whose pixels do not match its size")
	}
}

func TestLoadCalledOncePerRenderInOrder(t *testing.T) {
	var order []int
	layer := func(id int, at image.Point) *Layer {
		return &Layer{Load: func() (*raster.Buffer, image.Point, error) {
			order = append(order, id)
			return blob(10, 10, 0), at, nil
		}}
	}
	doc := lazyDoc(layer(0, image.Pt(0, 0)), &Group{Layers: []Node{layer(1, image.Pt(8, 8)), layer(2, image.Pt(16, 16))}}, layer(3, image.Pt(24, 24)))
	MustRender(doc)
	MustRender(doc)
	if want := []int{0, 1, 2, 3, 0, 1, 2, 3}; !slices.Equal(order, want) {
		t.Fatalf("Load order %v, want %v", order, want)
	}
}

// TestLoadedBufferIsWrittenInPlace pins that a buffer from Load is not copied
// before a mask or clip writes to it, which is what saves the allocation
func TestLoadedBufferIsWrittenInPlace(t *testing.T) {
	var loaded *raster.Buffer
	l := &Layer{
		Load: func() (*raster.Buffer, image.Point, error) {
			loaded = blob(20, 16, 0)
			return loaded, image.Pt(10, 10), nil
		},
		Mask: mask.FuncMask(func(x, y int) float32 { return 0 }),
	}
	MustRender(lazyDoc(l))
	for i, v := range loaded.Pix {
		if v != 0 {
			t.Fatalf("pixel %d is %v, the mask should have been applied to the loaded buffer itself", i, v)
		}
	}
}

// TestLoadDoesNotDisturbSecondRender renders twice from sources that Load copies,
// with masks, clipping and effects that all write to the loaded buffer, and
// requires the same output and untouched sources
func TestLoadDoesNotDisturbSecondRender(t *testing.T) {
	srcs := []*raster.Buffer{blob(30, 24, 0), blob(24, 20, 0.2), blob(24, 20, 0.1)}
	keep := make([][]float32, len(srcs))
	for i, s := range srcs {
		keep[i] = slices.Clone(s.Pix)
	}
	var calls int
	base := &Layer{Load: loader(srcs[0], image.Pt(20, 15), &calls)}
	masked := &Layer{Load: loader(srcs[1], image.Pt(30, 22), &calls), Mask: mask.FuncMask(func(x, y int) float32 { return 0.5 })}
	clipped := &Layer{
		Load:        loader(srcs[2], image.Pt(25, 18), &calls),
		ClipToBelow: true,
		Effects:     []effects.Effect{&effects.DropShadow{Color: color.Black, Opacity: 1, Distance: 3}},
	}
	doc := lazyDoc(&Layer{Content: fill(docW, docH, 0.1, 0.2, 0.3, 1)}, base, masked, clipped)
	first := MustRender(doc)
	second := MustRender(doc)
	requireSame(t, 0, first, second)
	for i, s := range srcs {
		if !slices.Equal(keep[i], s.Pix) {
			t.Errorf("source %d was modified", i)
		}
	}
	if calls != 6 {
		t.Errorf("%d Load calls over two renders of three lazy layers, want 6", calls)
	}
}

func TestLoadNilBufferContributesNothing(t *testing.T) {
	bg := &Layer{Content: fill(docW, docH, 0.1, 0.2, 0.3, 1)}
	want := MustRender(lazyDoc(bg))
	empty := &Layer{Load: func() (*raster.Buffer, image.Point, error) { return nil, image.Point{}, nil }}
	requireSame(t, 0, want, MustRender(lazyDoc(bg, empty)))

	// A clipped layer after an empty layer has no base to attach to, as with nil Content
	clipped := &Layer{Content: blob(10, 10, 0), Origin: image.Pt(5, 5), ClipToBelow: true, Mode: blend.Normal}
	eager := &Layer{}
	requireSame(t, 0, MustRender(lazyDoc(bg, eager, clipped)), MustRender(lazyDoc(bg, empty, clipped)))
}

func TestContentWinsOverLoad(t *testing.T) {
	var calls int
	l := &Layer{Content: blob(10, 10, 0), Origin: image.Pt(3, 3), Load: loader(blob(10, 10, 0.5), image.Pt(40, 40), &calls)}
	MustRender(lazyDoc(l))
	if calls != 0 {
		t.Fatalf("Load was called %d times for a layer with Content", calls)
	}
}

// TestLoadOriginReplacesLayerOrigin pins that a lazy layer sits where Load says
func TestLoadOriginReplacesLayerOrigin(t *testing.T) {
	var calls int
	l := &Layer{Origin: image.Pt(50, 50), Load: loader(blob(10, 10, 0), image.Pt(5, 5), &calls)}
	out := MustRender(lazyDoc(l))
	if _, _, _, a := out.At(10, 10); a == 0 {
		t.Error("nothing drawn where Load placed the content")
	}
	if _, _, _, a := out.At(55, 55); a != 0 {
		t.Error("the layer's own Origin was used")
	}
}
