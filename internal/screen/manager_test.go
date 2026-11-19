package screen

import "testing"

// TestScaleRampPullsTowardsIdentity checks that reducing the strength keeps the
// identity ramp fixed and moves a tinted ramp back towards it. The search that
// finds a driver's ceiling depends on this: at strength 1 nothing changes, and
// at 0 the screen is untouched.
func TestScaleRampPullsTowardsIdentity(t *testing.T) {
	identity := [3][256]uint16{}
	for c := 0; c < 3; c++ {
		for i := 0; i < 256; i++ {
			identity[c][i] = uint16(i * 257)
		}
	}
	tinted := identity
	for i := 0; i < 256; i++ {
		tinted[2][i] = uint16(float64(i*257) * 0.5)
	}

	if got := scaleRamp(identity, 0.5); got != identity {
		t.Fatalf("scaling the identity ramp changed it")
	}
	if got := scaleRamp(tinted, 1); got != tinted {
		t.Fatalf("strength 1 changed the ramp")
	}
	if got := scaleRamp(tinted, 0); got != identity {
		t.Fatalf("strength 0 should return the identity ramp, got %d at the last entry", got[2][255])
	}

	half := scaleRamp(tinted, 0.5)
	base := float64(identity[2][255])
	want := uint16(base + (float64(tinted[2][255])-base)*0.5 + 0.5)
	if half[2][255] != want {
		t.Fatalf("midpoint: got %d want %d", half[2][255], want)
	}
	// The red channel was untouched, so scaling must leave it alone.
	if half[0][255] != identity[0][255] {
		t.Fatalf("untouched channel moved: got %d want %d", half[0][255], identity[0][255])
	}
}

// TestScaleRampClamps checks the ends of the range, where rounding could push a
// value past what the driver accepts.
func TestScaleRampClamps(t *testing.T) {
	var ramp [3][256]uint16
	for c := 0; c < 3; c++ {
		for i := 0; i < 256; i++ {
			ramp[c][i] = 65535
		}
	}
	got := scaleRamp(ramp, 1.5)
	if got[0][0] != 65535 {
		t.Fatalf("strength above 1 should be ignored, got %d", got[0][0])
	}
	got = scaleRamp(ramp, -0.5)
	if got[0][0] != 0 {
		t.Fatalf("negative strength should clamp to the identity floor, got %d", got[0][0])
	}
}
