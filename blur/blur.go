// Package blur provides a fast separable blur used as a primitive by the effects
// package. It approximates a Gaussian with three successive box blurs, a
// well-known technique: three boxes converge to within a visually indistinct
// error of a true Gaussian, and each box blur is O(n) per pixel via a sliding
// window, independent of radius. Both passes parallelize across fixed bands so
// the result is deterministic.
package blur

import (
	"math"

	"github.com/odevine/impasto/internal/parallel"
	"github.com/odevine/impasto/raster"
)

// Gaussian blurs the buffer in place, approximating a Gaussian of standard
// deviation sigma pixels. It operates on premultiplied channels, which is what
// shadow and glow effects need. A non-positive sigma is a no-op.
func Gaussian(b *raster.Buffer, sigma float32) {
	if sigma <= 0 || b.Width == 0 || b.Height == 0 {
		return
	}
	sizes := boxesForGauss(float64(sigma), 3)
	tmp := make([]float32, len(b.Pix))
	for _, s := range sizes {
		r := (s - 1) / 2
		if r <= 0 {
			continue
		}
		boxBlurH(b.Pix, tmp, b.Width, b.Height, r)
		boxBlurV(tmp, b.Pix, b.Width, b.Height, r)
	}
}

// BoxBlur applies a single box blur of the given pixel radius in place, exposed
// for callers that want the cheaper primitive directly
func BoxBlur(b *raster.Buffer, radius int) {
	if radius <= 0 {
		return
	}
	tmp := make([]float32, len(b.Pix))
	boxBlurH(b.Pix, tmp, b.Width, b.Height, radius)
	boxBlurV(tmp, b.Pix, b.Width, b.Height, radius)
}

// boxesForGauss returns n odd box widths whose combined blur approximates a
// Gaussian of the given sigma, following Kutskir's derivation
func boxesForGauss(sigma float64, n int) []int {
	if sigma <= 0 {
		return make([]int, n)
	}
	wIdeal := math.Sqrt(12*sigma*sigma/float64(n) + 1)
	wl := int(math.Floor(wIdeal))
	if wl%2 == 0 {
		wl--
	}
	wu := wl + 2
	mIdeal := (12*sigma*sigma - float64(n*wl*wl) - float64(4*n*wl) - float64(3*n)) /
		(-4*float64(wl) - 4)
	m := int(math.Round(mIdeal))

	sizes := make([]int, n)
	for i := 0; i < n; i++ {
		if i < m {
			sizes[i] = wl
		} else {
			sizes[i] = wu
		}
	}
	return sizes
}

// boxBlurH blurs horizontally, reading src and writing dst. Edge samples are
// clamped (replicated), which leaves a transparent margin transparent and avoids
// darkening a solid image at its borders
func boxBlurH(src, dst []float32, w, h, r int) {
	inv := 1.0 / float32(2*r+1)
	parallel.Rows(h, func(lo, hi int) {
		for y := lo; y < hi; y++ {
			row := y * w * 4
			for c := 0; c < 4; c++ {
				base := row + c
				// Seed the window over [-r, r] with left-edge clamping
				var sum float32
				sum += float32(r+1) * src[base]
				for i := 1; i <= r; i++ {
					sum += src[base+clampIdx(i, w)*4]
				}
				for x := 0; x < w; x++ {
					dst[base+x*4] = sum * inv
					addIdx := clampIdx(x+r+1, w)
					remIdx := clampIdx(x-r, w)
					sum += src[base+addIdx*4] - src[base+remIdx*4]
				}
			}
		}
	})
}

// boxBlurV blurs vertically, reading src and writing dst
func boxBlurV(src, dst []float32, w, h, r int) {
	inv := 1.0 / float32(2*r+1)
	parallel.Rows(w, func(lo, hi int) {
		for x := lo; x < hi; x++ {
			col := x * 4
			stride := w * 4
			for c := 0; c < 4; c++ {
				base := col + c
				var sum float32
				sum += float32(r+1) * src[base]
				for i := 1; i <= r; i++ {
					sum += src[base+clampIdx(i, h)*stride]
				}
				for y := 0; y < h; y++ {
					dst[base+y*stride] = sum * inv
					addIdx := clampIdx(y+r+1, h)
					remIdx := clampIdx(y-r, h)
					sum += src[base+addIdx*stride] - src[base+remIdx*stride]
				}
			}
		}
	})
}

// clampIdx clamps an index into [0,n-1] for edge replication
func clampIdx(i, n int) int {
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}
