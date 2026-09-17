package gradient

import (
	"image/color"
	"testing"

	"github.com/odevine/impasto/path"
)

// FuzzGradient checks gradient construction and sampling against degenerate
// geometry and stop lists: zero or one stop, coincident endpoints, unsorted
// positions, extreme coordinates
func FuzzGradient(f *testing.F) {
	f.Add(uint8(0), float32(0), float32(0), float32(10), float32(0), uint8(3))
	f.Add(uint8(2), float32(5), float32(5), float32(5), float32(5), uint8(1))
	f.Fuzz(func(t *testing.T, kind uint8, x0, y0, x1, y1 float32, nstops uint8) {
		stops := make([]Stop, int(nstops)%6)
		for i := range stops {
			stops[i] = Stop{
				Pos:     float32(i) / float32(len(stops)+1),
				Color:   color.NRGBA{uint8(i * 40), 100, 200, 255},
				Opacity: float32(i%2) * 0.5,
			}
		}
		g := New(Kind(int(kind)%5), Pad, path.Point{X: x0, Y: y0}, path.Point{X: x1, Y: y1}, stops)
		// Sampling must never panic, including with no stops
		for _, xy := range [][2]int{{0, 0}, {5, 5}, {-3, 7}, {1000, 1000}} {
			g.ColorAt(xy[0], xy[1])
		}
	})
}
