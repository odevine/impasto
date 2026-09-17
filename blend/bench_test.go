package blend

import (
	"testing"

	"github.com/odevine/impasto/raster"
)

// BenchmarkComposite measures raw blend throughput per mode over a mid-size
// buffer, the number that governs how many layers a render can afford
func BenchmarkComposite(b *testing.B) {
	modes := []Mode{Normal, Multiply, Overlay, SoftLight, Color}
	for _, m := range modes {
		b.Run(m.String(), func(b *testing.B) {
			dst := raster.MustNewBuffer(1024, 1024)
			src := raster.MustNewBuffer(1024, 1024)
			for i := 0; i < len(src.Pix); i += 4 {
				src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3] = 0.6, 0.3, 0.2, 0.8
				dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = 0.2, 0.4, 0.6, 1
			}
			b.ResetTimer()
			b.SetBytes(int64(len(dst.Pix)) * 4)
			for i := 0; i < b.N; i++ {
				Composite(dst, src, m, 0.9)
			}
		})
	}
}
