package canvas

import (
	"image"
	"image/color"
	"sync"
	"testing"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/raster"
)

// The images below stand in for layers decoded from 8-bit files: some fill
// their rectangle, some are hollow frames, and each sits inside a print-size
// document at its own origin. They are built once and shared, as a decoder's
// cache would share them

var (
	imageSceneOnce sync.Once
	imageSceneDoc  *Document
)

// hollow is an opaque ring of the given thickness filling r, transparent inside
// and out
func hollow(r image.Rectangle, thickness int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			if x < thickness || y < thickness || x >= r.Dx()-thickness || y >= r.Dy()-thickness {
				setPattern(img, x, y, 255)
			}
		}
	}
	return img
}

// solid is an image of w by h whose pixels all have the given alpha
func solid(w, h int, alpha uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			setPattern(img, x, y, alpha)
		}
	}
	return img
}

// setPattern writes a deterministic color that varies across the image
func setPattern(img *image.NRGBA, x, y int, alpha uint8) {
	img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 7), G: uint8(y * 3), B: uint8(x + y), A: alpha})
}

// imageLayer places img at (x, y) and builds its content from the image each time
// it is composited, the way a layer backed by a decoded file does
func imageLayer(img *image.NRGBA, x, y int, mode blend.Mode, opacity float32) *Layer {
	return &Layer{
		Mode:    mode,
		Opacity: opacity,
		Load: func() (*raster.Buffer, image.Point, error) {
			buf, err := raster.FromImage(img)
			return buf, image.Pt(x, y), err
		},
	}
}

// imageScene is a print-size document of layers backed by 8-bit images, from a
// large opaque panel and hollow frames down to a strip and a small square, in
// Normal, Multiply and Overlay modes
func imageScene() *Document {
	imageSceneOnce.Do(func() {
		w, h := sceneW, sceneH
		layers := []Node{
			imageLayer(solid(w*7/10, h*7/10, 255), w*15/100, h*15/100, blend.Normal, 1),
			imageLayer(hollow(image.Rect(0, 0, w, h*85/100), w/30), 0, 0, blend.Normal, 1),
			imageLayer(solid(w*81/100, h*54/100, 200), w*9/100, h*8/100, blend.Normal, 1),
			imageLayer(hollow(image.Rect(0, 0, w*82/100, h*83/100), w/40), w*9/100, h*8/100, blend.Multiply, 1),
			imageLayer(hollow(image.Rect(0, 0, w*88/100, h*15/100), w/60), w*6/100, h*5/100, blend.Normal, 1),
			imageLayer(solid(w*17/100, h*7/100, 255), w*75/100, h*87/100, blend.Normal, 1),
			imageLayer(solid(w*77/100, h/130, 220), w*11/100, h*60/100, blend.Normal, 1),
			imageLayer(solid(w, h, 90), 0, 0, blend.Overlay, 0.5),
		}
		imageSceneDoc = &Document{Width: w, Height: h, Root: Group{PassThrough: true, Layers: layers}}
	})
	return imageSceneDoc
}

// benchRenders reports how many documents a render loop finishes per second
// beside the usual time and allocation figures
func benchRenders(b *testing.B) {
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "renders/s")
}

// BenchmarkRenderImages renders the image-backed scene with one goroutine
// issuing renders, so it measures one document's cost end to end
func BenchmarkRenderImages(b *testing.B) {
	if testing.Short() {
		b.Skip("builds print-size images")
	}
	d := imageScene()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MustRender(d)
	}
	benchRenders(b)
}

// BenchmarkRenderImagesParallel renders the same scene from GOMAXPROCS goroutines
// at once, which is how a batch uses the library, so it measures throughput when
// every core is busy and the renders compete for memory bandwidth. Run it with
// -cpu to vary the goroutine count
func BenchmarkRenderImagesParallel(b *testing.B) {
	if testing.Short() {
		b.Skip("builds print-size images and holds one document per core")
	}
	d := imageScene()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			MustRender(d)
		}
	})
	benchRenders(b)
}
