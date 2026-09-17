package blur

import (
	"testing"

	"github.com/odevine/impasto/raster"
)

// BenchmarkGaussian measures blur throughput at several radii. The sliding-window
// box passes should keep the cost nearly flat as the radius grows
func BenchmarkGaussian(b *testing.B) {
	radii := []float32{2, 8, 32}
	for _, r := range radii {
		b.Run(name(r), func(b *testing.B) {
			buf := raster.MustNewBuffer(1024, 1024)
			for i := 3; i < len(buf.Pix); i += 4 {
				buf.Pix[i] = 0.5
			}
			b.ResetTimer()
			b.SetBytes(int64(len(buf.Pix)) * 4)
			for i := 0; i < b.N; i++ {
				Gaussian(buf, r)
			}
		})
	}
}

func name(r float32) string {
	switch {
	case r <= 2:
		return "r2"
	case r <= 8:
		return "r8"
	default:
		return "r32"
	}
}
