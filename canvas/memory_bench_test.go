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
	cardW, cardH = 3264, 4440
	textW, textH = 2600, 260
)

// cardScene builds a document shaped like a card render: eight document-sized
// layers, ten text-sized layers one of which has a hard offset shadow, and a
// small icon. With placed set every text and icon layer is first placed into a
// document-sized buffer, which is the only way to build it without Origin
func cardScene(placed bool) *Document {
	var layers []Node
	for i := 0; i < 8; i++ {
		layers = append(layers, &Layer{
			Content: fill(cardW, cardH, 0.1+0.05*float32(i), 0.2, 0.3, 0.3),
			Opacity: 0.9,
			Mode:    blend.Normal,
		})
	}
	small := func(w, h int, x, y int, shadow bool) *Layer {
		l := &Layer{Content: fill(w, h, 0.9, 0.9, 0.9, 0.8), Origin: image.Pt(x, y)}
		if shadow {
			l.Effects = []effects.Effect{&effects.DropShadow{Color: color.Black, Opacity: 0.7, Angle: 0.8, Distance: 6}}
		}
		if placed {
			l.Content, l.Origin = Place(cardW, cardH, l.Content, x, y), image.Point{}
		}
		return l
	}
	for i := 0; i < 10; i++ {
		layers = append(layers, small(textW, textH, 300, 300+i*380, i == 3))
	}
	layers = append(layers, small(180, 180, 2900, 4000, false))
	return &Document{Width: cardW, Height: cardH, Root: Group{PassThrough: true, Layers: layers}}
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

// BenchmarkMemoryCardPlaced is the card scene with document-sized text layers
func BenchmarkMemoryCardPlaced(b *testing.B) { benchPeakHeap(b, cardScene(true)) }

// BenchmarkMemoryCardBounded is the card scene with the text and icon layers at
// their own size
func BenchmarkMemoryCardBounded(b *testing.B) { benchPeakHeap(b, cardScene(false)) }
