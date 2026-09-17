// Package demo holds helpers shared by the example programs: saving buffers to
// PNG, building solid and checkerboard content, and a few handy path shapes.
package demo

import (
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

// Save encodes a buffer to an 8-bit PNG, creating parent directories as needed
func Save(buf *raster.Buffer, name string) {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		panic(err)
	}
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, buf.ToImage(8)); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s\n", name)
}

// Solid returns a buffer filled with one sRGB color, converted to premultiplied
// linear
func Solid(w, h int, c color.Color) *raster.Buffer {
	fc := path.FlatColorSRGB(c)
	buf := raster.MustNewBuffer(w, h)
	for i := 0; i < len(buf.Pix); i += 4 {
		buf.Pix[i], buf.Pix[i+1], buf.Pix[i+2], buf.Pix[i+3] = fc[0], fc[1], fc[2], fc[3]
	}
	return buf
}

// Checker returns a checkerboard of two colors with the given tile size, useful
// as a backdrop for showing transparency
func Checker(w, h, size int, a, b color.Color) *raster.Buffer {
	ca := path.FlatColorSRGB(a)
	cb := path.FlatColorSRGB(b)
	buf := raster.MustNewBuffer(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := ca
			if ((x/size)+(y/size))%2 == 1 {
				c = cb
			}
			i := (y*w + x) * 4
			buf.Pix[i], buf.Pix[i+1], buf.Pix[i+2], buf.Pix[i+3] = c[0], c[1], c[2], c[3]
		}
	}
	return buf
}

// RoundRect builds a closed rounded-rectangle path with quadratic corners
func RoundRect(x0, y0, x1, y1, r float32) *path.Path {
	p := path.New()
	p.MoveTo(x0+r, y0)
	p.LineTo(x1-r, y0)
	p.QuadTo(x1, y0, x1, y0+r)
	p.LineTo(x1, y1-r)
	p.QuadTo(x1, y1, x1-r, y1)
	p.LineTo(x0+r, y1)
	p.QuadTo(x0, y1, x0, y1-r)
	p.LineTo(x0, y0+r)
	p.QuadTo(x0, y0, x0+r, y0)
	return p.Close()
}

// Pentagram builds a closed five-point star by connecting outer vertices in
// skip-one order, so the outline self-intersects. Filled nonzero it is solid,
// filled even-odd its center pentagon is hollow, which is what makes the two fill
// rules visibly differ
func Pentagram(cx, cy, r float32) *path.Path {
	const tau = 6.283185307179586
	p := path.New()
	for i := 0; i < 5; i++ {
		idx := (i * 2) % 5
		ang := float32(-tau/4) + float32(idx)*float32(tau)/5
		x := cx + r*cos(ang)
		y := cy + r*sin(ang)
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	return p.Close()
}

// Star builds a closed star with the given point count, outer radius, and inner
// radius, centered at (cx,cy)
func Star(cx, cy, outer, inner float32, points int) *path.Path {
	p := path.New()
	const tau = 6.283185307179586
	for i := 0; i < points*2; i++ {
		r := outer
		if i%2 == 1 {
			r = inner
		}
		ang := float32(-tau/4) + float32(i)*float32(tau)/float32(points*2)
		x := cx + r*cos(ang)
		y := cy + r*sin(ang)
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	return p.Close()
}
