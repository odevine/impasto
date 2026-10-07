package canvas

import (
	"errors"
	"image"
	"image/color"
	"runtime"
	"strings"
	"testing"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/internal/testimg"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/raster"
)

const imgDocW, imgDocH = 64, 48

// below is a layer under the one being tested, so a blend has a backdrop
func below() *Layer { return &Layer{Content: fill(imgDocW, imgDocH, 0.4, 0.5, 0.2, 0.7)} }

func renderBoth(t *testing.T, mk func(viaImage bool) []Node) (viaBuffer, viaImage *raster.Buffer) {
	t.Helper()
	build := func(viaImage bool) *raster.Buffer {
		out, err := Render(&Document{Width: imgDocW, Height: imgDocH, Root: Group{PassThrough: true, Layers: mk(viaImage)}})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	return build(false), build(true)
}

func equalBits(t *testing.T, got, want *raster.Buffer, what string) {
	t.Helper()
	testimg.EqualBits(t, got.Pix, want.Pix, what)
}

// imageLayer is one layer built either from the converted buffer or from the
// image itself, which must render the same
func imageLayerFor(img image.Image, at image.Point, viaImage bool, set func(*Layer)) *Layer {
	l := &Layer{Origin: at}
	if viaImage {
		l.Image = img
	} else {
		buf, err := raster.FromImage(img)
		if err != nil {
			panic(err)
		}
		l.Content = buf
	}
	if set != nil {
		set(l)
	}
	return l
}

func TestImageLayerRendersLikeItsBuffer(t *testing.T) {
	img := testimg.Patterned(37, 29)
	for m := blend.Normal; m <= blend.Luminosity; m++ {
		for _, op := range []float32{0, 0.5, 1} {
			for _, at := range []image.Point{{0, 0}, {9, 6}, {-6, -4}, {50, 40}} {
				a, b := renderBoth(t, func(viaImage bool) []Node {
					return []Node{below(), imageLayerFor(img, at, viaImage, func(l *Layer) { l.Mode, l.Opacity = m, op })}
				})
				equalBits(t, b, a, m.String())
			}
		}
	}
}

// Anything that writes to the content, or needs its coverage, converts the image
// first and must still agree with the buffer path
func TestImageLayerFallbacksRenderLikeTheirBuffer(t *testing.T) {
	img := testimg.Patterned(37, 29)
	cases := map[string]func(viaImage bool) []Node{
		"mask": func(v bool) []Node {
			return []Node{below(), imageLayerFor(img, image.Pt(8, 5), v, func(l *Layer) {
				l.Mask = mask.FuncMask(func(x, y int) float32 { return float32((x+y)%4) / 3 })
			})}
		},
		"effects": func(v bool) []Node {
			return []Node{below(), imageLayerFor(img, image.Pt(8, 5), v, func(l *Layer) {
				l.Effects = []effects.Effect{&effects.DropShadow{Color: color.Black, Opacity: 0.6, Angle: 0.8, Distance: 3}}
			})}
		},
		"clip base": func(v bool) []Node {
			clipped := &Layer{Content: fill(imgDocW, imgDocH, 0.9, 0.1, 0.1, 0.9), ClipToBelow: true}
			return []Node{below(), imageLayerFor(img, image.Pt(8, 5), v, nil), clipped}
		},
		"clipped": func(v bool) []Node {
			return []Node{below(), imageLayerFor(img, image.Pt(8, 5), v, func(l *Layer) { l.ClipToBelow = true })}
		},
	}
	for name, mk := range cases {
		a, b := renderBoth(t, mk)
		equalBits(t, b, a, name)
	}
}

func TestImageLayerOfAnotherTypeRendersLikeItsBuffer(t *testing.T) {
	nrgba := testimg.Patterned(20, 16)
	rgba := image.NewRGBA(nrgba.Rect)
	gray := image.NewGray(nrgba.Rect)
	for y := 0; y < 16; y++ {
		for x := 0; x < 20; x++ {
			rgba.Set(x, y, nrgba.At(x, y))
			gray.Set(x, y, nrgba.At(x, y))
		}
	}
	for name, img := range map[string]image.Image{"rgba": rgba, "gray": gray} {
		a, b := renderBoth(t, func(viaImage bool) []Node {
			return []Node{below(), imageLayerFor(img, image.Pt(5, 5), viaImage, nil)}
		})
		equalBits(t, b, a, name)
	}
}

func TestLoadImageIsCalledOnceAndSuppliesItsOrigin(t *testing.T) {
	img := testimg.Patterned(20, 16)
	calls := 0
	lazy := &Layer{Origin: image.Pt(40, 40), LoadImage: func() (image.Image, image.Point, error) {
		calls++
		return img, image.Pt(5, 5), nil
	}}
	got, err := Render(&Document{Width: imgDocW, Height: imgDocH, Root: Group{PassThrough: true, Layers: []Node{below(), lazy}}})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("LoadImage called %d times, want 1", calls)
	}
	want, _ := renderBoth(t, func(bool) []Node { return []Node{below(), imageLayerFor(img, image.Pt(5, 5), false, nil)} })
	equalBits(t, got, want, "lazy image")
}

func TestLoadImageErrorStopsTheRender(t *testing.T) {
	boom := errors.New("decode failed")
	later := 0
	doc := &Document{Width: imgDocW, Height: imgDocH, Root: Group{PassThrough: true, Layers: []Node{
		below(),
		&Layer{LoadImage: func() (image.Image, image.Point, error) { return nil, image.Point{}, boom }},
		&Layer{LoadImage: func() (image.Image, image.Point, error) { later++; return testimg.Patterned(4, 4), image.Point{}, nil }},
	}}}
	out, err := Render(doc)
	if out != nil || !errors.Is(err, boom) || !strings.Contains(err.Error(), "layer 1") || later != 0 {
		t.Fatalf("Render = %v, %v with %d later loads, want no buffer, the wrapped error naming layer 1, and none", out, err, later)
	}
}

func TestEmptyImagesContributeNothing(t *testing.T) {
	want, _ := renderBoth(t, func(bool) []Node { return []Node{below()} })
	for name, l := range map[string]*Layer{
		"empty image": {Image: image.NewNRGBA(image.Rect(0, 0, 0, 0))},
		"no content":  {},
		"lazy nil":    {LoadImage: func() (image.Image, image.Point, error) { return nil, image.Point{}, nil }},
	} {
		got, err := Render(&Document{Width: imgDocW, Height: imgDocH, Root: Group{PassThrough: true, Layers: []Node{below(), l}}})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		equalBits(t, got, want, name)
	}
}

// Content and Load come first, so a layer that sets an image too draws its buffer
func TestBufferContentWinsOverAnImage(t *testing.T) {
	img := testimg.Patterned(12, 12)
	buf := fill(8, 8, 0.1, 0.9, 0.3, 1)
	want, err := Render(&Document{Width: imgDocW, Height: imgDocH, Root: Group{PassThrough: true, Layers: []Node{below(), &Layer{Content: buf}}}})
	if err != nil {
		t.Fatal(err)
	}
	for name, l := range map[string]*Layer{
		"content": {Content: buf, Image: img},
		"load":    {Load: func() (*raster.Buffer, image.Point, error) { return buf.Clone(), image.Point{}, nil }, Image: img},
	} {
		got, err := Render(&Document{Width: imgDocW, Height: imgDocH, Root: Group{PassThrough: true, Layers: []Node{below(), l}}})
		if err != nil {
			t.Fatal(err)
		}
		equalBits(t, got, want, name)
	}
}

// An image is never written to, even by a layer that is masked or has effects
func TestRenderLeavesImagesAlone(t *testing.T) {
	img := testimg.Patterned(30, 24)
	pix := append([]uint8(nil), img.Pix...)
	for _, set := range []func(*Layer){
		nil,
		func(l *Layer) { l.Mask = mask.FuncMask(func(x, y int) float32 { return 0.5 }) },
		func(l *Layer) {
			l.Effects = []effects.Effect{&effects.DropShadow{Color: color.Black, Opacity: 0.5, Distance: 2}}
		},
	} {
		l := &Layer{Image: img, Origin: image.Pt(3, 3)}
		if set != nil {
			set(l)
		}
		if _, err := Render(&Document{Width: imgDocW, Height: imgDocH, Root: Group{PassThrough: true, Layers: []Node{below(), l}}}); err != nil {
			t.Fatal(err)
		}
	}
	for i := range pix {
		if img.Pix[i] != pix[i] {
			t.Fatal("a render wrote to the image")
		}
	}
}

// Banding must not change a pixel, so the result is the same however many
// goroutines share the work
func TestImageLayerIsIndependentOfParallelism(t *testing.T) {
	img := testimg.Patterned(61, 45)
	render := func(procs int) *raster.Buffer {
		defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(procs))
		layers := []Node{&Layer{Content: fill(200, 400, 0.4, 0.5, 0.2, 0.7)}}
		for _, m := range []blend.Mode{blend.Normal, blend.Multiply, blend.Overlay} {
			layers = append(layers, &Layer{Image: img, Origin: image.Pt(int(m)*20, int(m)*60), Mode: m, Opacity: 0.8})
		}
		out, err := Render(&Document{Width: 200, Height: 400, Root: Group{PassThrough: true, Layers: layers}})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	one := render(1)
	for _, procs := range []int{2, 7, 16} {
		equalBits(t, render(procs), one, "GOMAXPROCS")
	}
}

func TestIndexedLayerRendersLikeItsBuffer(t *testing.T) {
	const big = 200
	img := testimg.Sparse(190, 150, 32)
	ix := blend.Index(img)
	for m := blend.Normal; m <= blend.Luminosity; m++ {
		for _, at := range []image.Point{{0, 0}, {5, 9}, {-30, -20}, {120, 100}} {
			render := func(l *Layer) *raster.Buffer {
				out, err := Render(&Document{Width: big, Height: big, Root: Group{PassThrough: true, Layers: []Node{
					&Layer{Content: fill(big, big, 0.3, 0.6, 0.4, 0.8)}, l,
				}}})
				if err != nil {
					t.Fatal(err)
				}
				return out
			}
			buf, _ := raster.FromImage(img)
			want := render(&Layer{Content: buf, Origin: at, Mode: m, Opacity: 0.7})
			equalBits(t, render(&Layer{Image: ix, Origin: at, Mode: m, Opacity: 0.7}), want, m.String())
			lazy := render(&Layer{LoadImage: func() (image.Image, image.Point, error) { return ix, at, nil }, Mode: m, Opacity: 0.7})
			equalBits(t, lazy, want, m.String()+" lazy")
		}
	}
}

// A layer that has to write to its content converts the image, and an index
// does not change what it converts
func TestIndexedLayerFallbacksRenderLikeTheirBuffer(t *testing.T) {
	img := testimg.Sparse(60, 40, 32)
	ix := blend.Index(img)
	for name, set := range map[string]func(*Layer){
		"mask": func(l *Layer) { l.Mask = mask.FuncMask(func(x, y int) float32 { return float32((x+y)%4) / 3 }) },
		"effects": func(l *Layer) {
			l.Effects = []effects.Effect{&effects.DropShadow{Color: color.Black, Opacity: 0.6, Distance: 3}}
		},
		"clipped": func(l *Layer) { l.ClipToBelow = true },
	} {
		render := func(mk func() *Layer) *raster.Buffer {
			l := mk()
			set(l)
			out, err := Render(&Document{Width: imgDocW, Height: imgDocH, Root: Group{PassThrough: true, Layers: []Node{below(), l}}})
			if err != nil {
				t.Fatal(err)
			}
			return out
		}
		buf, _ := raster.FromImage(img)
		want := render(func() *Layer { return &Layer{Content: buf, Origin: image.Pt(2, 3)} })
		got := render(func() *Layer { return &Layer{Image: ix, Origin: image.Pt(2, 3)} })
		equalBits(t, got, want, name)
	}
}

// An image too wide to convert is reported as a load error where a conversion is
// needed, and blended without one where it is not
func TestImageTooWideToConvert(t *testing.T) {
	wide := image.NewNRGBA(image.Rect(0, 0, raster.MaxDimension+1, 1))
	masked := &Layer{Image: wide, Mask: mask.FuncMask(func(x, y int) float32 { return 1 })}
	_, err := Render(&Document{Width: imgDocW, Height: imgDocH, Root: Group{PassThrough: true, Layers: []Node{masked}}})
	if err == nil || !strings.Contains(err.Error(), "load") || !errors.Is(err, raster.ErrDimensions) {
		t.Fatalf("masked wide image error = %v, want a load error wrapping ErrDimensions", err)
	}
	// Blended directly it is only read, so its size does not matter
	if _, err := Render(&Document{Width: imgDocW, Height: imgDocH, Root: Group{PassThrough: true, Layers: []Node{&Layer{Image: wide}}}}); err != nil {
		t.Errorf("a wide image blended directly failed: %v", err)
	}
}
