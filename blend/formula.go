package blend

import "math"

// These are the per-channel separable blend functions B(cb, cs) from ISO
// 32000-2 section 11.3.5.2, operating on straight (non-premultiplied) color
// values in [0,1]. cb is the backdrop, cs is the source.
// See https://www.w3.org/TR/compositing-1/#blendingseparable

func mul(cb, cs float32) float32    { return cb * cs }
func screen(cb, cs float32) float32 { return cb + cs - cb*cs }

// hardLight decides on the source: darken where the source is dark, lighten
// where it is bright
func hardLight(cb, cs float32) float32 {
	if cs <= 0.5 {
		return mul(cb, 2*cs)
	}
	return screen(cb, 2*cs-1)
}

// overlay is hard light with the operands swapped, so the decision falls on the
// backdrop instead
func overlay(cb, cs float32) float32 {
	return hardLight(cs, cb)
}

// softLight uses the piecewise D(x) helper for its upper branch, matching the
// ISO formula rather than a closed-form approximation.
// See https://www.w3.org/TR/compositing-1/#blendingsoftlight
func softLight(cb, cs float32) float32 {
	if cs <= 0.5 {
		return cb - (1-2*cs)*cb*(1-cb)
	}
	var d float32
	if cb <= 0.25 {
		d = ((16*cb-12)*cb + 4) * cb
	} else {
		d = float32(math.Sqrt(float64(cb)))
	}
	return cb + (2*cs-1)*(d-cb)
}

func colorDodge(cb, cs float32) float32 {
	if cb == 0 {
		return 0
	}
	if cs == 1 {
		return 1
	}
	return minf(1, cb/(1-cs))
}

func colorBurn(cb, cs float32) float32 {
	if cb == 1 {
		return 1
	}
	if cs == 0 {
		return 0
	}
	return 1 - minf(1, (1-cb)/cs)
}

func darken(cb, cs float32) float32     { return minf(cb, cs) }
func lighten(cb, cs float32) float32    { return maxf(cb, cs) }
func difference(cb, cs float32) float32 { return absf(cb - cs) }
func exclusion(cb, cs float32) float32  { return cb + cs - 2*cb*cs }

// separable applies the channel-wise blend function for mode m. It is only
// called for separable modes, the non-separable ones go through nonSeparable
func separable(m Mode, cb, cs float32) float32 {
	switch m {
	case Normal:
		return cs
	case Multiply:
		return mul(cb, cs)
	case Screen:
		return screen(cb, cs)
	case Overlay:
		return overlay(cb, cs)
	case SoftLight:
		return softLight(cb, cs)
	case HardLight:
		return hardLight(cb, cs)
	case ColorDodge:
		return colorDodge(cb, cs)
	case ColorBurn:
		return colorBurn(cb, cs)
	case Darken:
		return darken(cb, cs)
	case Lighten:
		return lighten(cb, cs)
	case Difference:
		return difference(cb, cs)
	case Exclusion:
		return exclusion(cb, cs)
	default:
		return cs
	}
}

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func absf(a float32) float32 {
	if a < 0 {
		return -a
	}
	return a
}
