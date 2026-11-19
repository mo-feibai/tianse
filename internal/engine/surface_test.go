package engine

import (
	"testing"
	"time"

	"tianse/internal/config"
	"tianse/internal/curve"
	"tianse/internal/suncalc"
)

// A fixed afternoon: sunset 17:10, sunrise 06:41 local. The exact instants do
// not matter, only their order around the half-hour margins.
func skyAt(hour, minute int) (time.Time, curve.State) {
	base := time.Date(2026, 11, 18, 0, 0, 0, 0, time.Local)
	sunset := time.Date(2026, 11, 18, 17, 10, 0, 0, time.Local)
	sunrise := time.Date(2026, 11, 18, 6, 41, 0, 0, time.Local)
	cs := curve.State{Sunset: sunset, Sunrise: sunrise}
	return base.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute), cs
}

func TestSkyDarkTurnsOverHalfAnHourAfterSunset(t *testing.T) {
	// 17:39 is still light, 17:41 is dark. The margin exists so the flip does not
	// land on the minute the sun crosses the horizon.
	before, cs := skyAt(17, 39)
	if skyDark(before, cs) {
		t.Error("17:39 should still be light")
	}
	after, cs := skyAt(17, 41)
	if !skyDark(after, cs) {
		t.Error("17:41 should be dark")
	}
}

func TestSkyDarkLiftsHalfAnHourBeforeSunrise(t *testing.T) {
	before, cs := skyAt(6, 10)
	if !skyDark(before, cs) {
		t.Error("06:10 should still be dark")
	}
	after, cs := skyAt(6, 12)
	if skyDark(after, cs) {
		t.Error("06:12 should be light")
	}
}

func TestSkyDarkFallsBackToProgressWithoutSunTimes(t *testing.T) {
	// Polar day and polar night report no sunset or sunrise; progress is the only
	// signal left, and it is already the curve's own answer.
	now, _ := skyAt(12, 0)
	if skyDark(now, curve.State{Progress: 0.2}) {
		t.Error("progress 0.2 should be light")
	}
	if !skyDark(now, curve.State{Progress: 0.8}) {
		t.Error("progress 0.8 should be dark")
	}
}

func TestSurfaceDarkObeysTheForcedModes(t *testing.T) {
	// The forced modes must win over the clock in both directions: a user who
	// asked for dark at noon gets dark.
	noon, cs := skyAt(12, 0)
	midnight, csNight := skyAt(23, 30)

	cfg := config.Default()
	cfg.Theme.Enabled = true
	cfg.Theme.Mode = "always_dark"
	if !surfaceDark(noon, cs, cfg, false) {
		t.Error("always_dark at noon should be dark")
	}
	cfg.Theme.Mode = "always_light"
	if surfaceDark(midnight, csNight, cfg, true) {
		t.Error("always_light at 23:30 should be light")
	}
}

func TestSurfaceDarkTurnsDarkWithTheDesktopNotWithSunset(t *testing.T) {
	// The reported bug, as a test: at 17:17 the colour temperature is already
	// 3418 K, so the desktop has been dark since 16:50, while sunset plus thirty
	// minutes is still 24 minutes away. The window used to wait for the sunset
	// and sat white on a black desktop for 51 minutes.
	now, cs := skyAt(17, 17)
	cs.Kelvin = 3418

	cfg := config.Default()
	cfg.Theme.Enabled = true
	cfg.Theme.Mode = "follow_sky"

	if !surfaceDark(now, cs, cfg, false) {
		t.Error("17:17 at 3418 K should be dark, same as the desktop")
	}
}

func TestSurfaceDarkKeepsItsShadeBetweenTheThresholds(t *testing.T) {
	// Between dark_at_k and light_at_k neither answer is wrong, so the previous
	// frame decides. Without this the window would flap every time the colour
	// temperature wobbled around a threshold.
	now, cs := skyAt(18, 0)
	cs.Kelvin = 4800

	cfg := config.Default()
	cfg.Theme.Enabled = true
	cfg.Theme.Mode = "follow_sky"

	if surfaceDark(now, cs, cfg, false) {
		t.Error("4800 K after a light frame should stay light")
	}
	if !surfaceDark(now, cs, cfg, true) {
		t.Error("4800 K after a dark frame should stay dark")
	}
}

func TestSurfaceDarkFallsBackToTheSkyWithoutTheCurve(t *testing.T) {
	// With the temperature curve off the kelvin reading is pinned to the daytime
	// value and can never cross a threshold, so the sun becomes the only clock.
	evening, cs := skyAt(19, 0)
	cs.Kelvin = 6500

	cfg := config.Default()
	cfg.Theme.Enabled = true
	cfg.Theme.Mode = "follow_sky"
	cfg.Temp.Enabled = false

	if !surfaceDark(evening, cs, cfg, false) {
		t.Error("19:00 without the curve should still follow the sun and be dark")
	}
}

func TestSurfaceDarkFollowsTheSkyUnderPolarNight(t *testing.T) {
	// No sun times and no curve: progress is the only signal left, and it must
	// not be ignored just because the thresholds cannot be crossed.
	noon, _ := skyAt(12, 0)
	cs := curve.State{Progress: 0.9}

	cfg := config.Default()
	cfg.Theme.Enabled = true
	cfg.Theme.Mode = "follow_sky"
	cfg.Temp.Enabled = false

	if !surfaceDark(noon, cs, cfg, false) {
		t.Error("progress 0.9 should be dark")
	}
}
func TestNextShadeFlipNamesTheCrossingNotTheRampEnd(t *testing.T) {
	// The reported bug again, from the card's side: at 17:17 the window is
	// already dark, so "下次切换" must point at tomorrow morning, not at 17:40
	// (the end of the evening ramp, which the shade no longer waits for).
	now, _ := skyAt(17, 17)
	cfg := config.Default()
	cfg.Temp.Enabled = true

	// A ramp that mirrors the real one: 6500 K at noon, 3400 K by 17:00, back
	// up after 06:00.
	kelvinAt := func(t time.Time) float64 {
		h := float64(t.Hour()) + float64(t.Minute())/60
		if h >= 17 || h < 6 {
			return 3400
		}
		if h < 7 {
			return 6500 - (7-h)*3100
		}
		return 6500
	}

	flip := nextShadeFlip(now, true, cfg, kelvinAt)
	if flip.IsZero() {
		t.Fatal("a dark evening must still have a flip ahead")
	}
	// light_at_k (5200) is crossed at 06:35 on this ramp, not at 07:00 where
	// the morning ramp ends: the window turns light when it is light enough,
	// not when the ramp is over.
	if flip.Hour() != 6 || flip.Minute() < 30 || flip.Minute() > 40 {
		t.Fatalf("next flip should be the morning crossing near 06:35, got %02d:%02d", flip.Hour(), flip.Minute())
	}
	if flip.Before(now) {
		t.Error("the flip must be ahead of now")
	}
}

func TestNextShadeFlipFindsTheEveningCrossing(t *testing.T) {
	// From midday the next flip is the one the desktop makes at 16:50, not the
	// end of the ramp at 17:40.
	now, _ := skyAt(12, 0)
	cfg := config.Default()
	cfg.Temp.Enabled = true

	kelvinAt := func(t time.Time) float64 {
		h := float64(t.Hour()) + float64(t.Minute())/60
		if h < 16.5 {
			return 6500
		}
		if h > 17.5 {
			return 3400
		}
		return 6500 - (h-16.5)*3100
	}

	flip := nextShadeFlip(now, false, cfg, kelvinAt)
	if flip.IsZero() {
		t.Fatal("there must be an evening flip")
	}
	// dark_at_k (4500) is crossed at 17:09 on this ramp. The point is that it is
	// the crossing and not 17:40, the end of the evening ramp.
	if flip.Hour() != 17 || flip.Minute() < 5 || flip.Minute() > 15 {
		t.Fatalf("evening flip should be the dark_at_k crossing near 17:09, got %02d:%02d", flip.Hour(), flip.Minute())
	}
}

func TestNextShadeFlipGivesUpWhenNothingChanges(t *testing.T) {
	// Polar day with the curve pinned at the day value: no crossing in a day, so
	// the card must show nothing rather than a fabricated time.
	now, _ := skyAt(12, 0)
	cfg := config.Default()
	cfg.Temp.Enabled = true

	flip := nextShadeFlip(now, false, cfg, func(time.Time) float64 { return 6500 })
	if !flip.IsZero() {
		t.Fatalf("no crossing means no flip, got %v", flip)
	}
}
func TestNextShadeFlipUsesTheFixedScheduleWhenChosen(t *testing.T) {
	// A fixed table (19:00 -> 07:00) has its own windows and never consults the
	// sun, so the lookahead must sample the same curve evaluate does. Sampling
	// the solar one here would report a flip at sunset for a user who asked for
	// a clock.
	now, _ := skyAt(12, 0)
	cfg := config.Default()
	cfg.Temp.Enabled = true
	cfg.Schedule.Mode = "fixed"
	cfg.Schedule.Fixed.NightStart = "19:00"
	cfg.Schedule.Fixed.NightEnd = "07:00"

	loc := suncalc.Location{Latitude: 31.8206, Longitude: 117.2272}
	p := curve.Params{
		DayKelvin:    cfg.Temp.Day,
		NightKelvin:  cfg.Temp.Night,
		BeforeSunset: time.Duration(cfg.Temp.StartBeforeSunsetMin) * time.Minute,
		AfterSunset:  time.Duration(cfg.Temp.SettleAfterSunsetMin) * time.Minute,
	}

	flip := nextShadeFlip(now, false, cfg, kelvinAt(cfg, loc, p))
	if flip.IsZero() {
		t.Fatal("a fixed schedule still has an evening crossing")
	}
	// The evening ramp starts at 19:00 - 120 min = 17:00 and runs to 19:30, so
	// dark_at_k is crossed inside it, before 19:30.
	if flip.Hour() != 17 && flip.Hour() != 18 && !(flip.Hour() == 19 && flip.Minute() < 30) {
		t.Fatalf("fixed-schedule flip should land in the 17:00-19:30 ramp, got %02d:%02d", flip.Hour(), flip.Minute())
	}
}
