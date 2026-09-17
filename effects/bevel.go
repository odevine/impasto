package effects

import (
	"image/color"
	"math"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/blur"
	"github.com/odevine/impasto/raster"
)

// BevelStyle selects where the bevel sits relative to the layer edge
type BevelStyle int

const (
	BevelInner BevelStyle = iota
	BevelOuter
	Emboss
)

// Direction flips the light so the surface reads as raised or carved
type Direction int

const (
	DirUp Direction = iota
	DirDown
)

// BevelEmboss shades the layer as a lit 3D surface. The alpha channel is read as
// a height field, surface normals come from a Sobel gradient of that field, and a
// directional light splits into a highlight and a shadow contribution. This is
// the one effect with no clean public spec, so it is close-and-configurable
// rather than pixel-exact, and it deliberately returns two contributions with
// their own blend modes.
type BevelEmboss struct {
	Style            BevelStyle
	Depth            float32 // steepness of the simulated relief
	Direction        Direction
	Size             float32 // blur radius applied to the height field
	Soften           float32 // blur radius applied to the lit result
	Angle            float32 // light azimuth in radians
	Altitude         float32 // light elevation in radians
	HighlightColor   color.Color
	HighlightOpacity float32
	HighlightMode    blend.Mode
	ShadowColor      color.Color
	ShadowOpacity    float32
	ShadowMode       blend.Mode
}

// Render produces the highlight and shadow buffers, both clipped to the layer
func (b *BevelEmboss) Render(layer *raster.Buffer) []Rendered {
	w, h := layer.Width, layer.Height
	alpha := extractAlpha(layer)

	// Build and soften the height field
	height := append([]float32(nil), alpha...)
	if b.Size > 0 {
		hb := raster.MustNewBuffer(w, h)
		for i, a := range height {
			hb.Pix[i*4+3] = a
		}
		blur.Gaussian(hb, b.Size)
		height = extractAlpha(hb)
	}

	depth := b.Depth
	if depth <= 0 {
		depth = 1
	}
	alt := b.Altitude
	if alt == 0 {
		alt = math.Pi / 4
	}
	lx := float32(math.Cos(float64(alt)) * math.Cos(float64(b.Angle)))
	ly := float32(math.Cos(float64(alt)) * math.Sin(float64(b.Angle)))
	lz := float32(math.Sin(float64(alt)))

	hiCov := make([]float32, w*h)
	shCov := make([]float32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			gx, gy := sobel(height, w, h, x, y)
			// Surface normal from the height gradient, z fixed at 1
			nx := -gx * depth
			ny := -gy * depth
			nz := float32(1)
			inv := float32(1) / float32(math.Sqrt(float64(nx*nx+ny*ny+nz*nz)))
			nx, ny, nz = nx*inv, ny*inv, nz*inv
			intensity := nx*lx + ny*ly + nz*lz
			if b.Direction == DirDown {
				intensity = -intensity
			}
			i := y*w + x
			clip := alpha[i]
			if intensity > 0 {
				hiCov[i] = intensity * clip
			} else if intensity < 0 {
				shCov[i] = -intensity * clip
			}
		}
	}

	hiRGB := colorOrDefault(b.HighlightColor, [3]float32{1, 1, 1})
	shRGB := colorOrDefault(b.ShadowColor, [3]float32{0, 0, 0})
	hiBuf := colorize(hiCov, w, h, hiRGB, opacityOr(b.HighlightOpacity))
	shBuf := colorize(shCov, w, h, shRGB, opacityOr(b.ShadowOpacity))
	if b.Soften > 0 {
		blur.Gaussian(hiBuf, b.Soften)
		blur.Gaussian(shBuf, b.Soften)
	}

	return []Rendered{
		{Pixels: hiBuf, Behind: false, Mode: modeOr(b.HighlightMode, blend.Screen), Opacity: 1},
		{Pixels: shBuf, Behind: false, Mode: modeOr(b.ShadowMode, blend.Multiply), Opacity: 1},
	}
}

// sobel returns the height gradient at (x,y) using the 3x3 Sobel operator
func sobel(h []float32, w, ht, x, y int) (gx, gy float32) {
	s := func(dx, dy int) float32 { return at(h, w, ht, x+dx, y+dy) }
	gx = (s(1, -1) + 2*s(1, 0) + s(1, 1)) - (s(-1, -1) + 2*s(-1, 0) + s(-1, 1))
	gy = (s(-1, 1) + 2*s(0, 1) + s(1, 1)) - (s(-1, -1) + 2*s(0, -1) + s(1, -1))
	return gx / 8, gy / 8
}

func colorOrDefault(c color.Color, def [3]float32) [3]float32 {
	if c == nil {
		return def
	}
	rgb, _ := linearColor(c)
	return rgb
}
