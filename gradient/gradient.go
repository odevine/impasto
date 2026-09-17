// Package gradient generates linear, radial, angle, reflected, and diamond
// gradients with multi-stop color and independent per-stop opacity. A Gradient
// satisfies path.Paint, so it can fill a shape directly, and it can fill a whole
// buffer as a color field for overlay effects. Stops are interpolated in sRGB to
// match Photoshop, then converted to the linear working space, so results carry
// no 8-bit banding of their own, dithering happens only at final quantization.
package gradient

import (
	"image/color"
	"math"
	"sort"

	"github.com/odevine/impasto/internal/parallel"
	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

// Kind selects the gradient geometry
type Kind int

const (
	Linear Kind = iota
	Radial
	Angle
	Reflected
	Diamond
)

// Spread selects what happens outside the [0,1] gradient parameter range
type Spread int

const (
	// Pad clamps to the end stops
	Pad Spread = iota
	// Repeat tiles the gradient
	Repeat
	// Reflect mirrors the gradient on each repeat
	Reflect
)

// Stop is one color and opacity at a normalized position along the gradient.
// Opacity is authoritative and in [0,1], set it explicitly, the stop color's own
// alpha channel is ignored
type Stop struct {
	Pos     float32
	Color   color.Color
	Opacity float32
}

// Gradient is an immutable gradient description. Build one with New so the stops
// are sorted and precomputed. P0 is the start point or center, P1 is the end
// point or a point on the defining radius or axis
type Gradient struct {
	kind   Kind
	spread Spread
	p0, p1 path.Point

	pos []float32
	r   []float32
	g   []float32
	b   []float32
	op  []float32

	// Precomputed geometry
	dx, dy  float32
	len2    float32
	radius  float32
	ax, ay  float32 // unit axis for diamond
	baseAng float64
}

// New builds a gradient from stops, sorting them and decoding stop colors to
// straight sRGB for interpolation
func New(kind Kind, spread Spread, p0, p1 path.Point, stops []Stop) *Gradient {
	s := append([]Stop(nil), stops...)
	sort.SliceStable(s, func(i, j int) bool { return s[i].Pos < s[j].Pos })

	grad := &Gradient{kind: kind, spread: spread, p0: p0, p1: p1}
	for _, st := range s {
		var nc color.NRGBA
		if st.Color != nil {
			nc = color.NRGBAModel.Convert(st.Color).(color.NRGBA)
		}
		grad.pos = append(grad.pos, st.Pos)
		grad.r = append(grad.r, float32(nc.R)/255.0)
		grad.g = append(grad.g, float32(nc.G)/255.0)
		grad.b = append(grad.b, float32(nc.B)/255.0)
		grad.op = append(grad.op, clamp01(st.Opacity))
	}

	grad.dx = p1.X - p0.X
	grad.dy = p1.Y - p0.Y
	grad.len2 = grad.dx*grad.dx + grad.dy*grad.dy
	grad.radius = float32(math.Sqrt(float64(grad.len2)))
	if grad.radius > 0 {
		grad.ax = grad.dx / grad.radius
		grad.ay = grad.dy / grad.radius
	}
	grad.baseAng = math.Atan2(float64(grad.dy), float64(grad.dx))
	return grad
}

// param returns the raw gradient parameter at a pixel center before spread
func (g *Gradient) param(x, y int) float32 {
	px := float32(x) + 0.5
	py := float32(y) + 0.5
	vx := px - g.p0.X
	vy := py - g.p0.Y
	switch g.kind {
	case Radial:
		if g.radius == 0 {
			return 0
		}
		return float32(math.Sqrt(float64(vx*vx+vy*vy))) / g.radius
	case Angle:
		a := math.Atan2(float64(vy), float64(vx)) - g.baseAng
		t := a / (2 * math.Pi)
		t -= math.Floor(t)
		return float32(t)
	case Reflected:
		if g.len2 == 0 {
			return 0
		}
		proj := (vx*g.dx + vy*g.dy) / g.len2
		if proj < 0 {
			proj = -proj
		}
		return proj
	case Diamond:
		if g.radius == 0 {
			return 0
		}
		u := vx*g.ax + vy*g.ay
		w := -vx*g.ay + vy*g.ax
		return fmax(fabs(u), fabs(w)) / g.radius
	default: // Linear
		if g.len2 == 0 {
			return 0
		}
		return (vx*g.dx + vy*g.dy) / g.len2
	}
}

// applySpread folds the raw parameter into [0,1]
func (g *Gradient) applySpread(t float32) float32 {
	switch g.spread {
	case Repeat:
		return t - float32(math.Floor(float64(t)))
	case Reflect:
		tt := t - 2*float32(math.Floor(float64(t)*0.5))
		if tt > 1 {
			tt = 2 - tt
		}
		return tt
	default:
		return clamp01(t)
	}
}

// ColorAt returns the premultiplied linear RGBA of the gradient at pixel (x,y),
// satisfying path.Paint
func (g *Gradient) ColorAt(x, y int) [4]float32 {
	if len(g.pos) == 0 {
		return [4]float32{}
	}
	t := g.applySpread(g.param(x, y))
	sr, sg, sb, op := g.sample(t)
	// Interpolated color is straight sRGB, convert to linear then premultiply
	lr := raster.SRGBToLinear(sr)
	lg := raster.SRGBToLinear(sg)
	lb := raster.SRGBToLinear(sb)
	return [4]float32{lr * op, lg * op, lb * op, op}
}

// sample interpolates straight sRGB color and opacity at parameter t
func (g *Gradient) sample(t float32) (r, gg, b, op float32) {
	n := len(g.pos)
	if t <= g.pos[0] {
		return g.r[0], g.g[0], g.b[0], g.op[0]
	}
	if t >= g.pos[n-1] {
		return g.r[n-1], g.g[n-1], g.b[n-1], g.op[n-1]
	}
	// Binary search for the interval containing t
	i := sort.Search(n, func(i int) bool { return g.pos[i] > t })
	lo := i - 1
	hi := i
	span := g.pos[hi] - g.pos[lo]
	f := float32(0)
	if span > 0 {
		f = (t - g.pos[lo]) / span
	}
	return lerp(g.r[lo], g.r[hi], f),
		lerp(g.g[lo], g.g[hi], f),
		lerp(g.b[lo], g.b[hi], f),
		lerp(g.op[lo], g.op[hi], f)
}

// Render overwrites dst with the gradient field, ignoring dst's prior contents.
// Use this to build a color field, compositing and masking are the caller's job
func (g *Gradient) Render(dst *raster.Buffer) {
	w := dst.Width
	parallel.Rows(dst.Height, func(lo, hi int) {
		for y := lo; y < hi; y++ {
			i := y * w * 4
			for x := 0; x < w; x++ {
				c := g.ColorAt(x, y)
				dst.Pix[i] = c[0]
				dst.Pix[i+1] = c[1]
				dst.Pix[i+2] = c[2]
				dst.Pix[i+3] = c[3]
				i += 4
			}
		}
	})
}

func lerp(a, b, t float32) float32 { return a + (b-a)*t }

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func fmax(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func fabs(a float32) float32 {
	if a < 0 {
		return -a
	}
	return a
}
