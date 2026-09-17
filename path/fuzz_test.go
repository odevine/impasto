package path

import "testing"

// FuzzRasterize throws arbitrary coordinate streams at the fill rasterizer,
// which is a top spot for panics or infinite loops on degenerate input
// (zero-length segments, extreme or NaN-ish coordinates, unclosed subpaths)
func FuzzRasterize(f *testing.F) {
	f.Add([]byte{0, 0, 10, 10, 5, 20})
	f.Add([]byte{255, 0, 0, 255})
	f.Fuzz(func(t *testing.T, data []byte) {
		p := pathFromBytes(data)
		// Must not panic or hang on a small raster
		p.Coverage(32, 32, NonZero, DefaultTolerance)
		p.Coverage(32, 32, EvenOdd, DefaultTolerance)
	})
}

// FuzzStroke checks the stroker, which offsets and joins arbitrary geometry
func FuzzStroke(f *testing.F) {
	f.Add([]byte{0, 0, 20, 20, 10, 30}, uint8(4))
	f.Fuzz(func(t *testing.T, data []byte, width uint8) {
		p := pathFromBytes(data)
		out := Stroke(p, StrokeStyle{Width: float32(width) / 8, Cap: CapRound, Join: JoinRound})
		out.Coverage(32, 32, NonZero, DefaultTolerance)
	})
}

// pathFromBytes reads bytes as a sequence of coordinates mapped into a small
// range, turning every third pair into a curve so beziers get exercised too
func pathFromBytes(data []byte) *Path {
	p := New()
	coord := func(b byte) float32 { return float32(int(b)-64) / 4 }
	i := 0
	first := true
	for i+1 < len(data) {
		x := coord(data[i])
		y := coord(data[i+1])
		i += 2
		if first {
			p.MoveTo(x, y)
			first = false
			continue
		}
		switch (i / 2) % 3 {
		case 0:
			if i+3 < len(data) {
				p.CubicTo(x, y, coord(data[i]), coord(data[i+1]), coord(data[i+2]), coord(data[i+3]))
				i += 4
			} else {
				p.LineTo(x, y)
			}
		case 1:
			if i+1 < len(data) {
				p.QuadTo(x, y, coord(data[i]), coord(data[i+1]))
				i += 2
			} else {
				p.LineTo(x, y)
			}
		default:
			p.LineTo(x, y)
		}
	}
	p.Close()
	return p
}
