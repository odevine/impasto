// Package raster is the substrate every other package builds on. It defines the
// working pixel representation (float32, premultiplied alpha, linear light) and
// the only sanctioned bridge to and from standard library images. No other
// package should touch pixels without going through a Buffer.
package raster

import (
	"errors"
	"fmt"
)

// MaxDimension caps any single buffer edge. Untrusted image headers can declare
// enormous dimensions to force a huge allocation, so callers validate against
// this before allocating, never after. See the security notes in the spec
const MaxDimension = 1 << 16

// ErrDimensions reports a buffer size that is negative, zero, or beyond the
// safe allocation ceiling
var ErrDimensions = errors.New("raster: invalid buffer dimensions")

// Buffer is the working representation for all compositing: float32 RGBA,
// premultiplied alpha, linear light. Pix is row-major and interleaved, four
// floats per pixel, length Width*Height*4. Callers that hold a Buffer across
// goroutines must not mutate it concurrently, but independent Buffers are
// entirely independent.
type Buffer struct {
	Pix    []float32
	Width  int
	Height int
}

// NewBuffer allocates a zeroed (fully transparent) Buffer. It returns an error
// rather than panicking when the requested size is invalid or would overflow,
// so decoders can reject hostile inputs cleanly.
func NewBuffer(w, h int) (*Buffer, error) {
	if err := validateDimensions(w, h); err != nil {
		return nil, err
	}
	return &Buffer{
		Pix:    make([]float32, w*h*4),
		Width:  w,
		Height: h,
	}, nil
}

// MustNewBuffer is [NewBuffer] for trusted internal sizes where an invalid
// dimension is a programmer error, not attacker input
func MustNewBuffer(w, h int) *Buffer {
	b, err := NewBuffer(w, h)
	if err != nil {
		panic(err)
	}
	return b
}

// validateDimensions rejects non-positive, oversized, or overflowing sizes
// before any allocation is attempted
func validateDimensions(w, h int) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("%w: %dx%d must be positive", ErrDimensions, w, h)
	}
	if w > MaxDimension || h > MaxDimension {
		return fmt.Errorf("%w: %dx%d exceeds max edge %d", ErrDimensions, w, h, MaxDimension)
	}
	// Guard the w*h*4 element count against int overflow before make sees it
	if int64(w)*int64(h) > (1<<62)/4 {
		return fmt.Errorf("%w: %dx%d pixel count overflows", ErrDimensions, w, h)
	}
	return nil
}

// Bounds returns the pixel rectangle as (0,0)-(Width,Height)
func (b *Buffer) Bounds() (w, h int) {
	return b.Width, b.Height
}

// index returns the offset of pixel (x,y)'s first channel in Pix. It does no
// bounds checking, callers stay within Width and Height
func (b *Buffer) index(x, y int) int {
	return (y*b.Width + x) * 4
}

// At returns the premultiplied linear RGBA of pixel (x,y). Out-of-range
// coordinates return transparent black rather than panicking, which keeps
// sampling loops that read past an edge well defined.
func (b *Buffer) At(x, y int) (r, g, bl, a float32) {
	if x < 0 || y < 0 || x >= b.Width || y >= b.Height {
		return 0, 0, 0, 0
	}
	i := b.index(x, y)
	return b.Pix[i], b.Pix[i+1], b.Pix[i+2], b.Pix[i+3]
}

// Set writes premultiplied linear RGBA into pixel (x,y). Out-of-range
// coordinates are ignored
func (b *Buffer) Set(x, y int, r, g, bl, a float32) {
	if x < 0 || y < 0 || x >= b.Width || y >= b.Height {
		return
	}
	i := b.index(x, y)
	b.Pix[i] = r
	b.Pix[i+1] = g
	b.Pix[i+2] = bl
	b.Pix[i+3] = a
}

// Clone returns a deep copy that shares no backing storage with the original
func (b *Buffer) Clone() *Buffer {
	dst := &Buffer{
		Pix:    make([]float32, len(b.Pix)),
		Width:  b.Width,
		Height: b.Height,
	}
	copy(dst.Pix, b.Pix)
	return dst
}

// SameSize reports whether two buffers have identical dimensions, the
// precondition for most pixelwise operations
func (b *Buffer) SameSize(o *Buffer) bool {
	return b.Width == o.Width && b.Height == o.Height
}

// Clear resets every pixel to transparent black
func (b *Buffer) Clear() {
	for i := range b.Pix {
		b.Pix[i] = 0
	}
}
