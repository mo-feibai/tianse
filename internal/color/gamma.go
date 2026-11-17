package color

// Ramp is a single display's gamma ramp: one 256-entry table per channel.
// The memory layout matches the Win32 GAMMA_RAMP structure, so a Ramp can be
// handed straight to SetDeviceGammaRamp.
type Ramp [3][256]uint16

// BuildRamp produces the gamma ramp for a colour temperature at a brightness
// scale in [0,1]. scale is a multiplier applied on top of the colour gain; the
// colour temperature alone never dims the display because the gain triple is
// normalised to a peak of 1.
func BuildRamp(kelvin, scale float64) Ramp {
	if scale < 0 {
		scale = 0
	}
	if scale > 1 {
		scale = 1
	}
	gr, gg, gb := TempToRGB(kelvin)
	gains := [3]float64{gr * scale, gg * scale, gb * scale}

	var ramp Ramp
	for c := 0; c < 3; c++ {
		gain := gains[c]
		for i := 0; i < 256; i++ {
			v := (float64(i) / 255.0) * gain
			if v > 1 {
				v = 1
			}
			ramp[c][i] = uint16(v*65535 + 0.5)
		}
	}
	return ramp
}

// IdentityRamp is the untouched ramp: every channel passes through unchanged.
func IdentityRamp() Ramp {
	var ramp Ramp
	for c := 0; c < 3; c++ {
		for i := 0; i < 256; i++ {
			ramp[c][i] = uint16(i * 257)
		}
	}
	return ramp
}

// MaxDiff returns the largest absolute difference between two ramps in ramp
// units (0..65535). The engine uses it to skip writes smaller than 1%.
func (r Ramp) MaxDiff(other Ramp) int {
	max := 0
	for c := 0; c < 3; c++ {
		for i := 0; i < 256; i++ {
			d := int(r[c][i]) - int(other[c][i])
			if d < 0 {
				d = -d
			}
			if d > max {
				max = d
			}
		}
	}
	return max
}

// DiffRatio expresses MaxDiff as a fraction of the full ramp range.
func (r Ramp) DiffRatio(other Ramp) float64 {
	return float64(r.MaxDiff(other)) / 65535.0
}
