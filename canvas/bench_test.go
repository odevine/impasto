package canvas

import (
	"image/color"
	"testing"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/path"
)

// buildDoc assembles a representative document at the given size: a background,
// several blended layers, and a shape carrying a masked drop shadow and stroke
func buildDoc(w, h int) *Document {
	layers := []Node{
		&Layer{Content: fill(w, h, 0.15, 0.18, 0.22, 1), Opacity: 1},
	}
	for i := 0; i < 12; i++ {
		layers = append(layers, &Layer{
			Content: fill(w, h, 0.5, 0.3, 0.2, 0.4),
			Opacity: 0.7,
			Mode:    blend.Overlay,
		})
	}
	shape := path.New().Ellipse(float32(w)/2, float32(h)/2, float32(w)/4, float32(h)/4)
	layers = append(layers, &Layer{
		Content: fill(w, h, 0.9, 0.9, 0.95, 1),
		Opacity: 1,
		Mask:    mask.NewVectorMask(shape, w, h, path.NonZero),
		Effects: []effects.Effect{
			&effects.DropShadow{Color: color.Black, Opacity: 0.7, Angle: 2.36, Distance: 20, BlurRadius: 24},
			&effects.Stroke{Width: 6, Color: color.White, Alignment: effects.StrokeOutside, Opacity: 1},
		},
	})
	return &Document{Width: w, Height: h, Root: Group{PassThrough: true, Opacity: 1, Layers: layers}}
}

// BenchmarkRenderMidsize is a quick signal used in normal runs
func BenchmarkRenderMidsize(b *testing.B) {
	d := buildDoc(1024, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MustRender(d)
	}
}

// BenchmarkRenderPrintSize covers the heaviest realistic workload: a high-DPI
// print canvas with many layers and active effects, aimed at well under a
// second per render
func BenchmarkRenderPrintSize(b *testing.B) {
	d := buildDoc(3300, 4400)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MustRender(d)
	}
}
