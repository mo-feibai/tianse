package color

import (
	"math"
	"testing"
)

// The contrast floor is the one number the whole palette is built around, so it
// is asserted directly rather than trusted.
func TestContrastRatioKnownValues(t *testing.T) {
	if got := ContrastRatio("#FFFFFF", "#000000"); math.Abs(got-21) > 0.01 {
		t.Errorf("white on black = %.3f, want 21", got)
	}
	if got := ContrastRatio("#000000", "#000000"); math.Abs(got-1) > 0.01 {
		t.Errorf("black on black = %.3f, want 1", got)
	}
}

// Every text colour on the dark jade surface must clear 4.5:1 at every
// temperature the product can produce. This is the check that fails loudest if
// someone edits an anchor by eye.
func TestDarkPaletteContrast(t *testing.T) {
	const surface = "#1D2025"
	for k := 6500.0; k >= 3400; k -= 100 {
		p := DarkPalette(k)
		if got := ContrastRatio(p.Ink, surface); got < 4.5 {
			t.Errorf("ink at %.0f K = %s on jade = %.2f, want >= 4.5", k, p.Ink, got)
		}
		if got := ContrastRatio(p.Xiang, surface); got < 4.5 {
			t.Errorf("xiang at %.0f K on jade = %.2f, want >= 4.5", k, got)
		}
		// The accent is a non-text UI colour: WCAG 1.4.11 asks for 3:1.
		if got := ContrastRatio(p.Accent, surface); got < 3 {
			t.Errorf("accent at %.0f K on jade = %.2f, want >= 3", k, got)
		}
	}
}

func TestLightPaletteContrast(t *testing.T) {
	const surface = "#F5F8F8"
	for k := 6500.0; k >= 3400; k -= 100 {
		p := LightPalette(k)
		if got := ContrastRatio(p.Ink, surface); got < 4.5 {
			t.Errorf("light ink at %.0f K = %s on frost = %.2f, want >= 4.5", k, p.Ink, got)
		}
		if got := ContrastRatio(p.Accent, surface); got < 3 {
			t.Errorf("light accent at %.0f K = %.2f, want >= 3", k, got)
		}
		if got := ContrastRatio(p.Xiang, surface); got < 4.5 {
			t.Errorf("light xiang at %.0f K = %.2f, want >= 4.5", k, got)
		}
	}
}

// Relative luminance of the endpoints, so a change in the formula is caught.
func TestRelativeLuminance(t *testing.T) {
	if got := RelativeLuminance(255, 255, 255); math.Abs(got-1) > 1e-6 {
		t.Errorf("white = %v, want 1", got)
	}
	if got := RelativeLuminance(0, 0, 0); got != 0 {
		t.Errorf("black = %v, want 0", got)
	}
}
