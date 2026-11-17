package color

import (
	"math"
	"testing"
)

// The temperature table is the base every other colour in the product derives
// from, so its endpoints are pinned: if these move, every screenshot in the
// design document becomes wrong.
func TestTempToRGBEndpoints(t *testing.T) {
	cases := []struct {
		kelvin float64
		want   [3]float64
		tol    float64
	}{
		{DayKelvin, [3]float64{1, 0.9965, 0.9804}, 0.002},
		{NightKelvin, [3]float64{1, 0.744, 0.530}, 0.004},
	}
	for _, c := range cases {
		r, g, b := TempToRGB(c.kelvin)
		got := [3]float64{r, g, b}
		for i := range got {
			if math.Abs(got[i]-c.want[i]) > c.tol {
				t.Errorf("TempToRGB(%.0f)[%d] = %.4f, want %.4f +/- %.4f",
					c.kelvin, i, got[i], c.want[i], c.tol)
			}
		}
	}
}

// The gain triple is normalised to a peak of 1, which is what lets the ramp
// change hue without changing exposure.
func TestTempToRGBPeakIsOne(t *testing.T) {
	for k := 2700.0; k <= 8000.0; k += 100 {
		r, g, b := TempToRGB(k)
		peak := math.Max(r, math.Max(g, b))
		if math.Abs(peak-1) > 1e-9 {
			t.Errorf("TempToRGB(%.0f) peak = %.6f, want 1", k, peak)
		}
	}
}

// Warmer must mean less blue, monotonically. A non-monotonic table would make
// the slider feel like it jumps.
func TestTempToRGBBlueFallsMonotonically(t *testing.T) {
	prev := 1.0
	for k := 6500.0; k >= 2700; k -= 100 {
		_, _, b := TempToRGB(k)
		if b > prev+1e-9 {
			t.Fatalf("blue rose at %.0f K: %.4f after %.4f", k, b, prev)
		}
		prev = b
	}
}

// InverseGain is the basis of the accent pre-compensation in DESIGN 2.7: the
// value written to CSS must display as the intended colour once the screen
// applies its own gain. Round-tripping through both must land within a couple
// of 8-bit steps.
func TestInverseGainRoundTrip(t *testing.T) {
	for _, k := range []float64{6500, 5725, 4950, 4175, 3400} {
		gr, gg, gb := TempToRGB(k)
		gains := [3]float64{gr, gg, gb}
		want := [3]uint8{0x5A, 0xA4, 0xAE}
		r, g, b := InverseGain(want[0], want[1], want[2], k)
		got := [3]uint8{r, g, b}
		for c := 0; c < 3; c++ {
			// The screen multiplies by the gain; that is what the eye sees.
			seen := float64(got[c]) * gains[c]
			if seen > 255 {
				seen = 255
			}
			// A dim channel has a ceiling no source colour can lift: at 3400 K
			// blue tops out at 255 * 0.530 = 135. That ceiling is the documented
			// reason the accent hue walks towards green instead of staying teal
			// (DESIGN 2.7), so the test asserts the ceiling rather than a
			// round trip that physics forbids.
			reachable := math.Min(float64(want[c]), 255*gains[c])
			if math.Abs(seen-reachable) > 2 {
				t.Errorf("%.0f K channel %d: source %d displays as %.1f, ceiling %.1f (want %d)",
					k, c, got[c], seen, reachable, want[c])
			}
		}
	}
}

func TestBuildRampShape(t *testing.T) {
	r := BuildRamp(6500, 1)
	if r[0][0] != 0 || r[0][255] != 65535 {
		t.Errorf("red ramp endpoints = %d..%d, want 0..65535", r[0][0], r[0][255])
	}
	// Every channel is non-decreasing: the ramp is a gain curve, not a curve
	// with structure.
	for c := 0; c < 3; c++ {
		for i := 1; i < 256; i++ {
			if r[c][i] < r[c][i-1] {
				t.Fatalf("channel %d fell at index %d", c, i)
			}
		}
	}
}

func TestBuildRampScale(t *testing.T) {
	full := BuildRamp(6500, 1)
	half := BuildRamp(6500, 0.5)
	if d := full.MaxDiff(half); d < 30000 {
		t.Errorf("half scale differs by only %d ramp units; expected a large drop", d)
	}
	if half[1][255] > 33000 {
		t.Errorf("green at full input = %d, want about half of 65535", half[1][255])
	}
	// Out-of-range scales clamp rather than produce nonsense.
	if BuildRamp(6500, 2) != BuildRamp(6500, 1) {
		t.Error("scale above 1 was not clamped")
	}
	if BuildRamp(6500, -1) != BuildRamp(6500, 0) {
		t.Error("negative scale was not clamped")
	}
}

func TestIdentityRampIsIdentity(t *testing.T) {
	r := IdentityRamp()
	for c := 0; c < 3; c++ {
		for i := 0; i < 256; i++ {
			if r[c][i] != uint16(i*257) {
				t.Fatalf("identity[%d][%d] = %d", c, i, r[c][i])
			}
		}
	}
	if r.MaxDiff(IdentityRamp()) != 0 {
		t.Error("identity ramp differs from itself")
	}
}

func TestDiffRatio(t *testing.T) {
	a := IdentityRamp()
	b := IdentityRamp()
	b[2][255] = 0
	if got := a.DiffRatio(b); math.Abs(got-1) > 1e-9 {
		t.Errorf("DiffRatio = %v, want 1", got)
	}
}
