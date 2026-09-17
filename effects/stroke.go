package effects

import (
	"image/color"
	"math"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

// StrokeAlignment places the stroke band relative to the layer's alpha edge
type StrokeAlignment int

const (
	StrokeOutside StrokeAlignment = iota
	StrokeInside
	StrokeCenter
)

// Stroke outlines the layer's shape. For a raster layer the band is derived from
// the alpha edge by morphology, which works for any layer content. Fill it with a
// flat Color or, if set, an arbitrary Paint such as a gradient. Mode defaults to
// Normal
type Stroke struct {
	Width     float32
	Color     color.Color
	Paint     path.Paint
	Alignment StrokeAlignment
	Opacity   float32
	Mode      blend.Mode
}

// Render produces the stroke buffer, placed over the layer
func (s *Stroke) Render(layer *raster.Buffer) []Rendered {
	w, h := layer.Width, layer.Height
	alpha := extractAlpha(layer)
	cov := s.band(alpha, w, h)

	buf := raster.MustNewBuffer(w, h)
	if s.Paint != nil {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := cov[y*w+x]
				if c <= 0 {
					continue
				}
				p := s.Paint.ColorAt(x, y)
				i := (y*w + x) * 4
				buf.Pix[i] = p[0] * c
				buf.Pix[i+1] = p[1] * c
				buf.Pix[i+2] = p[2] * c
				buf.Pix[i+3] = p[3] * c
			}
		}
	} else {
		rgb, _ := linearColor(s.Color)
		buf = colorize(cov, w, h, rgb, 1)
	}

	return []Rendered{{
		Pixels:  buf,
		Behind:  false,
		Mode:    s.Mode,
		Opacity: opacityOr(s.Opacity),
	}}
}

// band computes the stroke coverage ring from the layer alpha per alignment
func (s *Stroke) band(alpha []float32, w, h int) []float32 {
	out := make([]float32, len(alpha))
	switch s.Alignment {
	case StrokeInside:
		r := int(math.Round(float64(s.Width)))
		er := erode(alpha, w, h, r)
		for i := range out {
			out[i] = clampCov(alpha[i] - er[i])
		}
	case StrokeCenter:
		r := int(math.Round(float64(s.Width) / 2))
		di := dilate(alpha, w, h, r)
		er := erode(alpha, w, h, r)
		for i := range out {
			out[i] = clampCov(di[i] - er[i])
		}
	default: // StrokeOutside
		r := int(math.Round(float64(s.Width)))
		di := dilate(alpha, w, h, r)
		for i := range out {
			out[i] = clampCov(di[i] - alpha[i])
		}
	}
	return out
}

func clampCov(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
