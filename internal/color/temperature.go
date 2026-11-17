package color

import "math"

// Day and night colour temperatures that bound the daily curve.
const (
	DayKelvin   = 6500.0
	NightKelvin = 3400.0
)

func clamp255(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// TempToRGB converts a black-body colour temperature in kelvin to a linear RGB
// gain triple in [0,1], normalised so the brightest channel is exactly 1.
//
// The curve is Tanner Helland's approximation of the Planckian locus. At 6500K
// it returns (1.0000, 0.9965, 0.9804) and at 3400K (1.0000, 0.7440, 0.5300),
// which is the #FFBE87 gain the design document assumes.
func TempToRGB(kelvin float64) (float64, float64, float64) {
	if kelvin < 1000 {
		kelvin = 1000
	}
	if kelvin > 40000 {
		kelvin = 40000
	}
	t := kelvin / 100

	var r, g, b float64
	if t <= 66 {
		r = 255
	} else {
		r = 329.698727446 * math.Pow(t-60, -0.1332047592)
	}
	if t <= 66 {
		g = 99.4708025861*math.Log(t) - 161.1195681661
	} else {
		g = 288.1221695283 * math.Pow(t-60, -0.0755148492)
	}
	switch {
	case t >= 66:
		b = 255
	case t <= 19:
		b = 0
	default:
		b = 138.5177312231*math.Log(t-10) - 305.0447927307
	}

	r, g, b = clamp255(r), clamp255(g), clamp255(b)
	m := math.Max(r, math.Max(g, b))
	if m <= 0 {
		return 1, 1, 1
	}
	return r / m, g / m, b / m
}

// Gain returns the per-channel multipliers a display at this colour temperature
// applies to whatever the application draws. Multiply a CSS source colour by
// this to obtain what the user actually sees.
func Gain(kelvin float64) (float64, float64, float64) {
	return TempToRGB(kelvin)
}

// InverseGain divides a target display colour by the gain at kelvin, which is
// how a CSS source colour is pre-compensated. Channels that would need more
// than 255 are clamped, so callers must keep the result in range themselves.
func InverseGain(r, g, b uint8, kelvin float64) (uint8, uint8, uint8) {
	gr, gg, gb := TempToRGB(kelvin)
	return clampByte(float64(r) / gr), clampByte(float64(g) / gg), clampByte(float64(b) / gb)
}

func clampByte(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}
