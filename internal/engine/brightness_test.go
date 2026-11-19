package engine

import (
	"math"
	"testing"

	"tianse/internal/config"
)

func TestBrightnessMovedIgnoresTheNoiseFloor(t *testing.T) {
	cases := []struct {
		read, last float64
		moved      bool
	}{
		{0.23, 0.23, false},
		{0.25, 0.23, false},  // 2%: one DDC read-back notch
		{0.279, 0.23, false}, // 4.9%: still inside the slack
		{0.29, 0.23, true},   // 6%: somebody turned a knob
		{0.17, 0.23, true},   // and it moves in both directions
	}
	for _, c := range cases {
		if got := brightnessMoved(c.read, c.last); got != c.moved {
			t.Errorf("brightnessMoved(%v, %v) = %v, want %v", c.read, c.last, got, c.moved)
		}
	}
}

func TestBaselineOfUsesThePanelUnderThePanelSetting(t *testing.T) {
	// "panel" multiplies against the panel's own full scale, so a ratio of 1.0
	// asks for the whole panel no matter how dim the screen was at launch.
	e := &Engine{baseline: map[string]float64{"d1": 0.23}}
	cfg := config.Default()
	cfg.Brightness.Baseline = config.BaselinePanel
	if got := e.baselineOf(cfg, "d1"); got != 1 {
		t.Errorf("baselineOf = %v, want 1 for the panel baseline", got)
	}
}

func TestBaselineOfUsesTheLaunchReadingUnderTheStartupSetting(t *testing.T) {
	e := &Engine{baseline: map[string]float64{"d1": 0.23}}
	cfg := config.Default()
	cfg.Brightness.Baseline = config.BaselineStartup
	if got := e.baselineOf(cfg, "d1"); math.Abs(got-0.23) > 1e-9 {
		t.Errorf("baselineOf = %v, want the 0.23 launch reading", got)
	}
}

func TestBaselineOfFallsBackToThePanelWithNoReading(t *testing.T) {
	// A display that never answered DDC has no launch reading. Falling back to
	// the panel keeps the multiplier meaningful instead of zeroing the screen.
	e := &Engine{baseline: map[string]float64{}}
	cfg := config.Default()
	cfg.Brightness.Baseline = config.BaselineStartup
	if got := e.baselineOf(cfg, "d1"); got != 1 {
		t.Errorf("baselineOf = %v, want 1 when the launch reading is missing", got)
	}
}
