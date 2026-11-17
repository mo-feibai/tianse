package color

import (
	"math"
	"testing"
)

// The five anchors are what DESIGN 2.7 prints as a table; if an endpoint moves,
// the document is wrong, so both ends are pinned here.
func TestDarkPaletteEndpoints(t *testing.T) {
	day := DarkPalette(DayKelvin)
	if day.Accent != "#5AA5B2" {
		t.Errorf("accent at 6500 K = %s, want #5AA5B2", day.Accent)
	}
	if day.Xiang != DarkXiang {
		t.Errorf("xiang = %s, want %s (it is never compensated)", day.Xiang, DarkXiang)
	}
	night := DarkPalette(NightKelvin)
	if night.Accent != "#49E4FF" {
		t.Errorf("accent at 3400 K = %s, want #49E4FF", night.Accent)
	}
}

// displayedHue is what the eye actually sees: the source colour after the
// screen's own gain. The source colour itself is clamped at 255 in the blue
// channel at low temperatures, so its raw hue is not a meaningful series.
func displayedHue(kelvin float64) float64 {
	return Hue(Preview(DarkPalette(kelvin).Accent, kelvin))
}

// The accent must walk from teal towards sea green as the screen warms: that
// direction is the only one reachable once the blue gain collapses.
func TestAccentHueFallsWithKelvin(t *testing.T) {
	prev := displayedHue(6500)
	for k := 6400.0; k >= 3400; k -= 100 {
		h := displayedHue(k)
		if h > prev+0.5 {
			t.Fatalf("displayed hue rose at %.0f K: %.1f after %.1f", k, h, prev)
		}
		prev = h
	}
	if h := displayedHue(6500); math.Abs(h-187) > 1.5 {
		t.Errorf("day displayed hue = %.1f, want 187", h)
	}
	if h := displayedHue(3400); math.Abs(h-159) > 1.5 {
		t.Errorf("night displayed hue = %.1f, want 159", h)
	}
}

// Neighbouring stops must not step by more than a few degrees, or the 3-second
// crossfade would show banding.
func TestAccentHueIsSmooth(t *testing.T) {
	for k := 6400.0; k >= 3400; k -= 100 {
		a := displayedHue(k)
		b := displayedHue(k - 100)
		if d := math.Abs(a - b); d > 4 {
			t.Errorf("displayed hue jumps %.1f deg between %.0f and %.0f K", d, k, k-100)
		}
	}
}

// Ink is compensated for luminance only: its hue must not swing, because the
// secondary text is a neutral, not a tinted accent.
func TestInkStaysNeutralish(t *testing.T) {
	for k := 6500.0; k >= 3400; k -= 100 {
		p := DarkPalette(k)
		r, g, b := hexToRGB(p.Ink)
		// Blue must remain the largest channel: a warm-grey secondary text would
		// read as a second accent colour.
		if !(b >= g && g >= r) {
			t.Fatalf("ink at %.0f K = %s is not cool-ordered", k, p.Ink)
		}
	}
}

// Interpolation must be monotonic in each channel between two anchors.
func TestInterpolateMonotonic(t *testing.T) {
	table := []Anchor{{6500, "#000000"}, {3400, "#FFFFFF"}}
	prev := -1.0
	for k := 6500.0; k >= 3400; k -= 50 {
		r, _, _ := hexToRGB(interpolate(table, k))
		if float64(r) < prev {
			t.Fatalf("red fell at %.0f K", k)
		}
		prev = float64(r)
	}
	if got := interpolate(table, 9000); got != "#000000" {
		t.Errorf("above the table = %s, want the first stop", got)
	}
	if got := interpolate(table, 1000); got != "#FFFFFF" {
		t.Errorf("below the table = %s, want the last stop", got)
	}
}

// The light surface keeps its hue: a green accent on frost reads as a different
// product rather than as evening.
func TestLightPaletteHueIsStable(t *testing.T) {
	day := Hue(LightPalette(6500).Accent)
	night := Hue(LightPalette(3400).Accent)
	if math.Abs(day-night) > 3 {
		t.Errorf("light accent hue moved %.1f deg between day and night", math.Abs(day-night))
	}
}

func TestHueWraps(t *testing.T) {
	// Pure red is hue 0; a value just below it must not read as 360.
	if h := Hue("#FF0000"); h != 0 {
		t.Errorf("red hue = %v, want 0", h)
	}
	if h := Hue("#FF0080"); h < 300 || h > 360 {
		t.Errorf("magenta hue = %v, want near 330", h)
	}
}
