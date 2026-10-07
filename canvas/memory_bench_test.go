package canvas

import (
	"image"
	"image/color"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/raster"
)

const (
	sceneW, sceneH = 3264, 4440
	stripW, stripH = 2600, 260
)

// sceneKind selects how layeredScene supplies its layer content
type sceneKind int

const (
	// scenePlaced puts every strip and square layer in a document-sized buffer, the
	// only way to build the scene without Origin
	scenePlaced sceneKind = iota
	// sceneBounded gives them their own size and an Origin
	sceneBounded
	// sceneLazy also builds every layer's buffer in Load, as a decoder would
	sceneLazy
)

// layeredScene builds a print-size document of many layers: eight document-sized
// layers, ten strip-sized layers one of which has a hard offset shadow, and a
// small square
func layeredScene(kind sceneKind) *Document {
	// sized is a layer of w by h filled with a color at x,y, built up front or on
	// demand according to the kind
	sized := func(w, h, x, y int, r, g, b, a float32) *Layer {
		switch kind {
		case sceneLazy:
			return &Layer{Load: func() (*raster.Buffer, image.Point, error) {
				return fill(w, h, r, g, b, a), image.Pt(x, y), nil
			}}
		case sceneBounded:
			return &Layer{Content: fill(w, h, r, g, b, a), Origin: image.Pt(x, y)}
		default:
			return &Layer{Content: Place(sceneW, sceneH, fill(w, h, r, g, b, a), x, y)}
		}
	}
	var layers []Node
	for i := 0; i < 8; i++ {
		l := sized(sceneW, sceneH, 0, 0, 0.1+0.05*float32(i), 0.2, 0.3, 0.3)
		l.Opacity, l.Mode = 0.9, blend.Normal
		layers = append(layers, l)
	}
	small := func(w, h int, x, y int, shadow bool) *Layer {
		l := sized(w, h, x, y, 0.9, 0.9, 0.9, 0.8)
		if shadow {
			l.Effects = []effects.Effect{&effects.DropShadow{Color: color.Black, Opacity: 0.7, Angle: 0.8, Distance: 6}}
		}
		return l
	}
	for i := 0; i < 10; i++ {
		layers = append(layers, small(stripW, stripH, 300, 300+i*380, i == 3))
	}
	layers = append(layers, small(180, 180, 2900, 4000, false))
	return &Document{Width: sceneW, Height: sceneH, Root: Group{PassThrough: true, Layers: layers}}
}

// benchPeakHeap renders the document b.N times and reports the highest heap in
// use seen while doing so, which includes the document's own layer buffers. Run
// it with GOGC=10 so the figure is close to live memory
func benchPeakHeap(b *testing.B, d *Document) {
	if testing.Short() {
		b.Skip("allocates several GB")
	}
	runtime.GC()
	var peak uint64
	var mu sync.Mutex
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(2 * time.Millisecond)
		defer t.Stop()
		var ms runtime.MemStats
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				runtime.ReadMemStats(&ms)
				mu.Lock()
				if ms.HeapAlloc > peak {
					peak = ms.HeapAlloc
				}
				mu.Unlock()
			}
		}
	}()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out *raster.Buffer = MustRender(d)
		_ = out
	}
	b.StopTimer()
	close(stop)
	<-done
	mu.Lock()
	b.ReportMetric(float64(peak)/(1<<20), "peak-heap-MiB")
	mu.Unlock()
}

// BenchmarkMemoryPlaced is the layered scene with document-sized strip layers
func BenchmarkMemoryPlaced(b *testing.B) { benchPeakHeap(b, layeredScene(scenePlaced)) }

// BenchmarkMemoryBounded is the layered scene with the strip and square layers
// at their own size
func BenchmarkMemoryBounded(b *testing.B) { benchPeakHeap(b, layeredScene(sceneBounded)) }

// BenchmarkMemoryLazy is the bounded scene with every layer built by Load, so
// only the layer being composited is alive
func BenchmarkMemoryLazy(b *testing.B) { benchPeakHeap(b, layeredScene(sceneLazy)) }
