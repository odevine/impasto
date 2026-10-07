// Package testimg builds the 8-bit images and comparisons that tests in several
// packages share: images with transparent, opaque and partial regions to blend,
// and an exact comparison of pixel data. It imports nothing from the rest of the
// module, so any package's tests can use it, including raster's own
package testimg

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// Pixel is a deterministic color that varies across an image, with the alpha
// given
func Pixel(x, y int, alpha uint8) color.NRGBA {
	return color.NRGBA{R: uint8(x*37 + y*5), G: uint8(y*23 + x), B: uint8(x*y + 9), A: alpha}
}

// Patterned is an image whose alpha takes the values a blend branches on,
// transparent, opaque, one short of opaque, the faintest and partial, spread
// across its pixels
func Patterned(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	alphas := []uint8{0, 255, 128, 1, 254, 77, 0, 255, 200}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, Pixel(x, y, alphas[(x*3+y*7)%len(alphas)]))
		}
	}
	return img
}

// Solid is an image of w by h whose pixels all have the given alpha
func Solid(w, h int, alpha uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, Pixel(x, y, alpha))
		}
	}
	return img
}

// Hollow is an opaque frame of the given thickness around w by h, transparent
// inside
func Hollow(w, h, thickness int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < thickness || y < thickness || x >= w-thickness || y >= h-thickness {
				img.SetNRGBA(x, y, Pixel(x, y, 255))
			}
		}
	}
	return img
}

// Sparse is an image whose rows hold stretches of visible pixels separated by
// gaps shorter than, equal to and longer than minGap, plus rows that are empty
// and rows that are full. minGap is the merge distance the test is probing
func Sparse(w, h, minGap int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	set := func(x, y int, a uint8) { img.SetNRGBA(x, y, Pixel(x, y, a)) }
	for y := 0; y < h; y++ {
		switch y % 6 {
		case 0: // empty
		case 1: // full and opaque
			for x := 0; x < w; x++ {
				set(x, y, 255)
			}
		case 2: // two strips, a gap one short of minGap between them
			for x := 3; x < 10; x++ {
				set(x, y, 200)
			}
			for x := 10 + minGap - 1; x < 10+minGap+9; x++ {
				set(x, y, 90)
			}
		case 3: // two strips, a gap of exactly minGap between them
			for x := 3; x < 10; x++ {
				set(x, y, 255)
			}
			for x := 10 + minGap; x < 10+minGap+9; x++ {
				set(x, y, 255)
			}
		case 4: // a frame: a strip at each end and nothing between
			for x := 0; x < 5; x++ {
				set(x, y, 255)
				set(w-1-x, y, 128)
			}
		case 5: // lone pixels, one with the faintest alpha
			set(w/3, y, 1)
			set(w-1, y, 255)
		}
	}
	return img
}

// EqualBits fails the test unless got and want hold the same float32 bit
// patterns, which is stricter than equal values since it tells -0 from 0
func EqualBits(t testing.TB, got, want []float32, what string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d values, want %d", what, len(got), len(want))
	}
	for i := range want {
		if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
			t.Fatalf("%s: value %d (pixel %d, channel %d) = %v, want %v", what, i, i/4, i%4, got[i], want[i])
		}
	}
}
