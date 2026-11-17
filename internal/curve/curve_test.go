package curve

import (
	"math"
	"testing"
	"time"

	"tianse/internal/suncalc"
)

var hangzhou = suncalc.Location{Latitude: 30.2741, Longitude: 120.1551}

func at(t *testing.T, hh, mm int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("no timezone data: %v", err)
	}
	return time.Date(2026, 11, 17, hh, mm, 0, 0, loc)
}

// The ramp must reach the day value exactly at the start of its window and the
// night value exactly at the end, with no overshoot in between: the endpoints
// are what the settings page promises.
func TestEvaluateHitsBothEnds(t *testing.T) {
	p := Defaults()
	// Sunset is about 17:02; the ramp opens two hours earlier.
	start := at(t, 15, 2)
	mid := at(t, 17, 2)
	end := at(t, 17, 32)

	if st := Evaluate(start, hangzhou, p); math.Abs(st.Kelvin-p.DayKelvin) > 1 {
		t.Errorf("at the window start kelvin = %.1f, want %.1f", st.Kelvin, p.DayKelvin)
	}
	if st := Evaluate(end, hangzhou, p); math.Abs(st.Kelvin-p.NightKelvin) > 1 {
		t.Errorf("at the window end kelvin = %.1f, want %.1f", st.Kelvin, p.NightKelvin)
	}
	if st := Evaluate(mid, hangzhou, p); st.Kelvin > p.DayKelvin || st.Kelvin < p.NightKelvin {
		t.Errorf("midpoint kelvin %.1f is outside the ramp", st.Kelvin)
	}
}

// The cosine ease means the ramp is slow at both ends and fast in the middle.
// A linear ramp would pass a midpoint check but fail this one.
func TestEvaluateEases(t *testing.T) {
	p := Defaults()
	// The evening window runs 15:02 to 17:32, so these land at 25/50/75%.
	quarter := Evaluate(at(t, 15, 39), hangzhou, p)
	half := Evaluate(at(t, 16, 17), hangzhou, p)
	threeQ := Evaluate(at(t, 16, 54), hangzhou, p)

	span := p.DayKelvin - p.NightKelvin
	if drop := (p.DayKelvin - quarter.Kelvin) / span; drop > 0.20 {
		t.Errorf("25%% through the ramp dropped %.2f of the span; a cosine ease gives about 0.15", drop)
	}
	if drop := (p.DayKelvin - half.Kelvin) / span; math.Abs(drop-0.5) > 0.02 {
		t.Errorf("50%% through dropped %.2f, want 0.5", drop)
	}
	if drop := (p.DayKelvin - threeQ.Kelvin) / span; drop < 0.80 {
		t.Errorf("75%% through dropped only %.2f; a cosine ease gives about 0.85", drop)
	}
}

// Kelvin must never increase during the evening or decrease during the morning.
func TestEvaluateIsMonotonic(t *testing.T) {
	p := Defaults()
	prev := p.DayKelvin + 1
	for m := 15 * 60; m <= 17*60+30; m += 5 {
		st := Evaluate(at(t, m/60, m%60), hangzhou, p)
		if st.Kelvin > prev+0.5 {
			t.Fatalf("evening kelvin rose at %02d:%02d: %.1f after %.1f", m/60, m%60, st.Kelvin, prev)
		}
		prev = st.Kelvin
	}
	prev = p.NightKelvin - 1
	for m := 5 * 60; m <= 8*60+30; m += 5 {
		st := Evaluate(at(t, m/60, m%60), hangzhou, p)
		if st.Kelvin < prev-0.5 {
			t.Fatalf("morning kelvin fell at %02d:%02d: %.1f after %.1f", m/60, m%60, st.Kelvin, prev)
		}
		prev = st.Kelvin
	}
}

func TestEvaluatePhases(t *testing.T) {
	p := Defaults()
	cases := []struct {
		hh, mm int
		want   Phase
	}{
		{12, 0, PhaseDay},
		{16, 0, PhaseEvening},
		{22, 0, PhaseNight},
		{6, 45, PhaseMorning},
	}
	for _, c := range cases {
		if st := Evaluate(at(t, c.hh, c.mm), hangzhou, p); st.Phase != c.want {
			t.Errorf("%02d:%02d phase = %s, want %s", c.hh, c.mm, st.Phase, c.want)
		}
	}
}

// NextChange must always be in the future, or the status card shows a time that
// has already passed.
func TestNextChangeIsAhead(t *testing.T) {
	p := Defaults()
	for m := 0; m < 24*60; m += 10 {
		now := at(t, m/60, m%60)
		st := Evaluate(now, hangzhou, p)
		if st.Polar {
			continue
		}
		if !st.NextChange.After(now) {
			t.Fatalf("at %02d:%02d next change %s is not ahead", m/60, m%60, st.NextChange)
		}
	}
}

// Progress is 0 at the day end and 1 at the night end, and is clamped outside
// the ramp.
func TestProgressBounds(t *testing.T) {
	p := Defaults()
	if st := Evaluate(at(t, 12, 0), hangzhou, p); st.Progress != 0 {
		t.Errorf("noon progress = %v, want 0", st.Progress)
	}
	if st := Evaluate(at(t, 23, 0), hangzhou, p); st.Progress != 1 {
		t.Errorf("late night progress = %v, want 1", st.Progress)
	}
}

func TestFixedEvaluate(t *testing.T) {
	p := Defaults()
	nightStart := 19 * time.Hour
	nightEnd := 7 * time.Hour
	// 21:00 is inside the fixed night window.
	if st := FixedEvaluate(at(t, 21, 0), p, nightStart, nightEnd); st.Phase != PhaseNight {
		t.Errorf("21:00 phase = %s, want night", st.Phase)
	}
	// 12:00 is the day.
	if st := FixedEvaluate(at(t, 12, 0), p, nightStart, nightEnd); st.Phase != PhaseDay {
		t.Errorf("12:00 phase = %s, want day", st.Phase)
	}
	// 02:00 is also inside the window, which crosses midnight.
	if st := FixedEvaluate(at(t, 2, 0), p, nightStart, nightEnd); st.Phase == PhaseDay {
		t.Errorf("02:00 phase = %s, want not day", st.Phase)
	}
	// 19:00 exactly is the start of the ramp, not yet night.
	if st := FixedEvaluate(at(t, 19, 0), p, nightStart, nightEnd); st.Phase != PhaseEvening {
		t.Errorf("19:00 phase = %s, want evening", st.Phase)
	}
}

func TestBlendEndpoints(t *testing.T) {
	if got := blend(6500, 3400, 0); math.Abs(got-6500) > 1e-9 {
		t.Errorf("blend(0) = %v", got)
	}
	if got := blend(6500, 3400, 1); math.Abs(got-3400) > 1e-9 {
		t.Errorf("blend(1) = %v", got)
	}
	if got := blend(6500, 3400, 0.5); math.Abs(got-4950) > 1e-9 {
		t.Errorf("blend(0.5) = %v, want the midpoint", got)
	}
}

// A polar location must degrade rather than produce a temperature from a zero
// hour angle.
func TestEvaluatePolar(t *testing.T) {
	tromso := suncalc.Location{Latitude: 69.6492, Longitude: 18.9553}
	p := Defaults()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skip("no tz")
	}
	st := Evaluate(time.Date(2026, 12, 21, 12, 0, 0, 0, loc), tromso, p)
	if !st.Polar {
		t.Fatal("Tromso in December was not reported polar")
	}
	if st.Kelvin != p.NightKelvin {
		t.Errorf("polar kelvin = %v, want the night value", st.Kelvin)
	}
}
