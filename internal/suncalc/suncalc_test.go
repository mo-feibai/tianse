package suncalc

import (
	"math"
	"testing"
	"time"
)

func mustLoad(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("no timezone data: %v", err)
	}
	return loc
}

// Hangzhou on 2026-11-17, checked against sunrise-sunset.org: sunrise 06:26:18,
// sunset 17:02:13 local. A minute of slack absorbs the almanac's rounding.
func TestForDayHangzhou(t *testing.T) {
	loc := mustLoad(t)
	date := time.Date(2026, 11, 17, 12, 0, 0, 0, loc)
	day, err := ForDay(date, Location{Latitude: 30.2741, Longitude: 120.1551})
	if err != nil {
		t.Fatalf("ForDay: %v", err)
	}
	if day.PolarDay || day.PolarNight {
		t.Fatal("Hangzhou reported a polar day")
	}
	sunrise := day.Sunrise.In(loc)
	sunset := day.Sunset.In(loc)
	wantRise := time.Date(2026, 11, 17, 6, 26, 0, 0, loc)
	wantSet := time.Date(2026, 11, 17, 17, 2, 0, 0, loc)
	if d := sunrise.Sub(wantRise); math.Abs(d.Minutes()) > 1.5 {
		t.Errorf("sunrise = %s, want about %s", sunrise.Format("15:04:05"), wantRise.Format("15:04:05"))
	}
	if d := sunset.Sub(wantSet); math.Abs(d.Minutes()) > 1.5 {
		t.Errorf("sunset = %s, want about %s", sunset.Format("15:04:05"), wantSet.Format("15:04:05"))
	}
	if !sunrise.Before(sunset) {
		t.Error("sunrise is not before sunset")
	}
}

// The sun rises earlier and sets later in summer; a table that never moves
// would pass a single-day check but fail this one.
func TestForDayFollowsSeasons(t *testing.T) {
	loc := mustLoad(t)
	place := Location{Latitude: 30.2741, Longitude: 120.1551}
	summer, err := ForDay(time.Date(2026, 6, 21, 12, 0, 0, 0, loc), place)
	if err != nil {
		t.Fatalf("summer: %v", err)
	}
	winter, err := ForDay(time.Date(2026, 12, 21, 12, 0, 0, 0, loc), place)
	if err != nil {
		t.Fatalf("winter: %v", err)
	}
	summerDay := summer.Sunset.Sub(summer.Sunrise)
	winterDay := winter.Sunset.Sub(winter.Sunrise)
	if summerDay <= winterDay {
		t.Errorf("summer daylight %v is not longer than winter %v", summerDay, winterDay)
	}
}

// Tromso in December has no sunrise at all: the product must degrade to a fixed
// schedule rather than guess.
func TestForDayPolarNight(t *testing.T) {
	loc := mustLoad(t)
	date := time.Date(2026, 12, 21, 12, 0, 0, 0, loc)
	day, err := ForDay(date, Location{Latitude: 69.6492, Longitude: 18.9553})
	if err != ErrNoSunriseSunset {
		t.Fatalf("ForDay above the Arctic circle = %v, want ErrNoSunriseSunset", err)
	}
	_ = day
}

func TestSunsetBeforeAndSunriseAfter(t *testing.T) {
	loc := mustLoad(t)
	place := Location{Latitude: 30.2741, Longitude: 120.1551}
	noon := time.Date(2026, 11, 17, 12, 0, 0, 0, loc)
	sunset, err := SunsetBefore(noon, place)
	if err != nil {
		t.Fatalf("SunsetBefore: %v", err)
	}
	if !sunset.Before(noon) {
		t.Errorf("SunsetBefore returned %s, which is not before the input", sunset)
	}
	if h := sunset.In(loc).Hour(); h != 17 {
		t.Errorf("sunset hour = %d, want 17", h)
	}
	sunrise, err := SunriseAfter(noon, place)
	if err != nil {
		t.Fatalf("SunriseAfter: %v", err)
	}
	if !sunrise.After(noon) {
		t.Errorf("SunriseAfter returned %s, which is not after the input", sunrise)
	}
}

// Just after midnight the "previous sunset" belongs to the day before, which is
// where a naive same-day lookup returns the future sunset and the ramp runs
// backwards.
func TestSunsetBeforeAfterMidnight(t *testing.T) {
	loc := mustLoad(t)
	place := Location{Latitude: 30.2741, Longitude: 120.1551}
	early := time.Date(2026, 11, 17, 1, 0, 0, 0, loc)
	sunset, err := SunsetBefore(early, place)
	if err != nil {
		t.Fatalf("SunsetBefore: %v", err)
	}
	if !sunset.Before(early) {
		t.Fatalf("sunset %s is not before %s", sunset, early)
	}
	if sunset.In(loc).Day() != 16 {
		t.Errorf("sunset fell on the %d, want the 16th", sunset.In(loc).Day())
	}
}

func TestJulianRoundTrip(t *testing.T) {
	loc := mustLoad(t)
	now := time.Date(2026, 11, 17, 9, 41, 30, 0, loc)
	back := TimeFromJulian(JulianFromTime(now)).In(loc)
	if d := back.Sub(now); math.Abs(d.Seconds()) > 1 {
		t.Errorf("julian round trip drifted %v", d)
	}
}
