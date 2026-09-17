// Package blend implements the Photoshop blend modes as pure functions. The
// formulas come straight from ISO 32000-2 (the PDF spec) section 11.3.5, which
// standardized exactly this set, so the math is a transcription of a published
// reference rather than something reverse engineered from screenshots. The
// package knows nothing about layers or masks, it is only the color math.
package blend

// Mode selects a blend function. The ordering is fixed and part of the API,
// callers may persist the integer values
type Mode int

const (
	Normal Mode = iota
	Multiply
	Screen
	Overlay
	SoftLight
	HardLight
	ColorDodge
	ColorBurn
	Darken
	Lighten
	Difference
	Exclusion
	Hue
	Saturation
	Color
	Luminosity

	// numModes is the count of defined modes, used for validation
	numModes
)

var modeNames = [numModes]string{
	Normal:     "Normal",
	Multiply:   "Multiply",
	Screen:     "Screen",
	Overlay:    "Overlay",
	SoftLight:  "SoftLight",
	HardLight:  "HardLight",
	ColorDodge: "ColorDodge",
	ColorBurn:  "ColorBurn",
	Darken:     "Darken",
	Lighten:    "Lighten",
	Difference: "Difference",
	Exclusion:  "Exclusion",
	Hue:        "Hue",
	Saturation: "Saturation",
	Color:      "Color",
	Luminosity: "Luminosity",
}

// String returns the mode name, or a placeholder for an out-of-range value
func (m Mode) String() string {
	if m < 0 || m >= numModes {
		return "Mode(invalid)"
	}
	return modeNames[m]
}

// Valid reports whether m names a defined blend mode
func (m Mode) Valid() bool {
	return m >= 0 && m < numModes
}

// isNonSeparable reports whether a mode blends the whole RGB triple together
// rather than channel by channel
func (m Mode) isNonSeparable() bool {
	return m >= Hue && m <= Luminosity
}
