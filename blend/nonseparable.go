package blend

// The non-separable modes (Hue, Saturation, Color, Luminosity) act on the whole
// RGB triple at once. ISO 32000-2 section 11.3.5.3 defines them in terms of four
// helpers, so each mode is a short composition once the helpers exist.

type rgb [3]float32

// lum is the luminance of a color using the spec's fixed coefficients
func lum(c rgb) float32 {
	return 0.3*c[0] + 0.59*c[1] + 0.11*c[2]
}

// clipColor pulls any out-of-gamut channel back into [0,1] while preserving
// luminance, the operation SetLum relies on to stay valid
func clipColor(c rgb) rgb {
	l := lum(c)
	n := minf(c[0], minf(c[1], c[2]))
	x := maxf(c[0], maxf(c[1], c[2]))
	if n < 0 {
		d := l - n
		if d != 0 {
			c[0] = l + (c[0]-l)*l/d
			c[1] = l + (c[1]-l)*l/d
			c[2] = l + (c[2]-l)*l/d
		}
	}
	if x > 1 {
		d := x - l
		if d != 0 {
			c[0] = l + (c[0]-l)*(1-l)/d
			c[1] = l + (c[1]-l)*(1-l)/d
			c[2] = l + (c[2]-l)*(1-l)/d
		}
	}
	return c
}

// setLum shifts a color to a target luminance, then clips it back into gamut
func setLum(c rgb, l float32) rgb {
	d := l - lum(c)
	c[0] += d
	c[1] += d
	c[2] += d
	return clipColor(c)
}

// sat is the saturation of a color, the span between its brightest and darkest
// channels
func sat(c rgb) float32 {
	return maxf(c[0], maxf(c[1], c[2])) - minf(c[0], minf(c[1], c[2]))
}

// setSat rescales a color to a target saturation, keeping the relative ordering
// of its channels. It works on indices so the min, mid, and max channels are
// handled without sorting the values in place
func setSat(c rgb, s float32) rgb {
	iMin, iMid, iMax := 0, 1, 2
	if c[iMin] > c[iMid] {
		iMin, iMid = iMid, iMin
	}
	if c[iMid] > c[iMax] {
		iMid, iMax = iMax, iMid
	}
	if c[iMin] > c[iMid] {
		iMin, iMid = iMid, iMin
	}
	var out rgb
	if c[iMax] > c[iMin] {
		out[iMid] = (c[iMid] - c[iMin]) / (c[iMax] - c[iMin]) * s
		out[iMax] = s
	}
	out[iMin] = 0
	return out
}

// nonSeparable applies one of the four whole-triple modes
func nonSeparable(m Mode, cb, cs rgb) rgb {
	switch m {
	case Hue:
		return setLum(setSat(cs, sat(cb)), lum(cb))
	case Saturation:
		return setLum(setSat(cb, sat(cs)), lum(cb))
	case Color:
		return setLum(cs, lum(cb))
	case Luminosity:
		return setLum(cb, lum(cs))
	default:
		return cs
	}
}
