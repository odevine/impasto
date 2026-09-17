package raster

import "math"

// The whole pipeline works in linear light. Source pixels arrive sRGB encoded
// and must be decoded on ingest, results are re-encoded on egress. Alpha is
// always linear and never passes through these transfer functions.

// SRGBToLinear decodes a single sRGB-encoded channel value in [0,1] to linear
// light. This is the IEC 61966-2-1 transfer function
func SRGBToLinear(c float32) float32 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return float32(math.Pow((float64(c)+0.055)/1.055, 2.4))
}

// LinearToSRGB encodes a single linear-light channel value in [0,1] back to
// sRGB. Inverse of [SRGBToLinear]
func LinearToSRGB(c float32) float32 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return float32(1.055*math.Pow(float64(c), 1.0/2.4) - 0.055)
}

// srgb8ToLinearLUT maps every 8-bit sRGB code to its linear-light value so the
// common decode path is a table lookup rather than a pow per channel.
var srgb8ToLinearLUT [256]float32

// srgb16ToLinearLUT does the same for 16-bit sRGB codes. It costs 256 KiB and
// keeps 16-bit ingest branch-free and deterministic
var srgb16ToLinearLUT [65536]float32

func init() {
	for i := range srgb8ToLinearLUT {
		srgb8ToLinearLUT[i] = SRGBToLinear(float32(i) / 255.0)
	}
	for i := range srgb16ToLinearLUT {
		srgb16ToLinearLUT[i] = SRGBToLinear(float32(i) / 65535.0)
	}
}

// clamp01 constrains a value to [0,1], the valid range for a channel
func clamp01(c float32) float32 {
	if c < 0 {
		return 0
	}
	if c > 1 {
		return 1
	}
	return c
}
