// Package curve turns clock time plus solar events into the two numbers the
// engine actually writes: a colour temperature and a brightness ratio.
package curve

import (
	"math"
	"time"

	"tianse/internal/suncalc"
)

// Phase names the segment of the daily cycle the user is currently in.
type Phase string

const (
	PhaseDay     Phase = "day"
	PhaseEvening Phase = "evening"
	PhaseNight   Phase = "night"
	PhaseMorning Phase = "morning"
	PhasePolar   Phase = "polar"
)

// Params are the tunable durations of the ramp. They come straight from the
// configuration file.
type Params struct {
	DayKelvin    float64
	NightKelvin  float64
	BeforeSunset time.Duration
	AfterSunset  time.Duration
}

// Defaults returns the values documented in SPEC section 3.
func Defaults() Params {
	return Params{
		DayKelvin:    6500,
		NightKelvin:  3400,
		BeforeSunset: 120 * time.Minute,
		AfterSunset:  30 * time.Minute,
	}
}

// State is everything the UI needs to draw the current moment.
type State struct {
	Kelvin     float64
	Phase      Phase
	Progress   float64 // 0 at the day end of the ramp, 1 at the night end
	Sunset     time.Time
	Sunrise    time.Time
	NextChange time.Time
	Polar      bool
}

// Evaluate computes the state for an instant.
//
// The evening ramp runs from (sunset - BeforeSunset) to (sunset + AfterSunset).
// The morning ramp is its mirror image: (sunrise - AfterSunset) to
// (sunrise + BeforeSunset). Between them the temperature holds at the day
// value; through the night it holds at the night value.
func Evaluate(now time.Time, loc suncalc.Location, p Params) State {
	// The nearest sunset and the nearest sunrise in time bracket now from both
	// sides, which is exactly the pair the ramp is drawn between. Using "the
	// last sunset" instead would break in the afternoon: at 16:00 the evening
	// ramp has already started, so the governing sunset is the one ahead.
	sunset, sunrise, err := suncalc.Cycle(now, loc)
	if err != nil {
		return polarState(p)
	}

	eveningStart := sunset.Add(-p.BeforeSunset)
	eveningEnd := sunset.Add(p.AfterSunset)
	morningStart := sunrise.Add(-p.AfterSunset)
	morningEnd := sunrise.Add(p.BeforeSunset)

	st := State{Sunset: sunset, Sunrise: sunrise, Kelvin: p.DayKelvin, Phase: PhaseDay}

	// The four windows are ordered on the clock, so testing them in order is
	// the whole state machine: nothing before the morning ramp is day, and
	// nothing after the evening ramp is night again until tomorrow.
	switch {
	case now.Before(morningStart):
		st.Phase = PhaseNight
		st.Progress = 1
		st.Kelvin = p.NightKelvin
		st.NextChange = morningStart

	case now.Before(morningEnd):
		st.Phase = PhaseMorning
		st.Progress = 1 - progress(now, morningStart, morningEnd)
		st.Kelvin = blend(p.DayKelvin, p.NightKelvin, st.Progress)
		st.NextChange = morningEnd

	case now.Before(eveningStart):
		st.Phase = PhaseDay
		st.Progress = 0
		st.NextChange = eveningStart

	case now.Before(eveningEnd):
		st.Phase = PhaseEvening
		st.Progress = progress(now, eveningStart, eveningEnd)
		st.Kelvin = blend(p.DayKelvin, p.NightKelvin, st.Progress)
		st.NextChange = eveningEnd

	default:
		st.Phase = PhaseNight
		st.Progress = 1
		st.Kelvin = p.NightKelvin
		// The next morning belongs to the next cycle; the sunrise we already
		// have is behind us, so ask for the following one.
		if nextRise, err := suncalc.SunriseAfter(eveningEnd, loc); err == nil {
			st.NextChange = nextRise.Add(-p.AfterSunset)
		} else {
			st.NextChange = morningStart.AddDate(0, 0, 1)
		}
	}
	return st
}

// polarState is what a location without sunrise or sunset gets: the night
// value, held, with no next change to promise.
func polarState(p Params) State {
	return State{Kelvin: p.NightKelvin, Phase: PhasePolar, Progress: 1, Polar: true}
}

// progress maps an instant onto [0,1] across a window.
func progress(now, start, end time.Time) float64 {
	if !end.After(start) {
		return 0
	}
	f := float64(now.Sub(start)) / float64(end.Sub(start))
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// blend applies the cosine ease so the ramp leaves and arrives with zero
// slope; a linear ramp reads as a visible "snap" at both ends.
func blend(dayK, nightK, p float64) float64 {
	t := (1 - math.Cos(math.Pi*p)) / 2
	return dayK + (nightK-dayK)*t
}

// FixedEvaluate is the fallback used when the schedule is a fixed clock table
// rather than solar events. nightStart and nightEnd are minutes from midnight.
func FixedEvaluate(now time.Time, p Params, nightStart, nightEnd time.Duration) State {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := midnight.Add(nightStart)
	end := midnight.Add(nightEnd)
	if !end.After(start) {
		// The window crosses midnight.
		if now.Before(end) {
			start = start.AddDate(0, 0, -1)
		} else {
			end = end.AddDate(0, 0, 1)
		}
	}

	st := State{Kelvin: p.DayKelvin, Phase: PhaseDay, NextChange: start}
	eveningStart := start.Add(-p.BeforeSunset)
	eveningEnd := start.Add(p.AfterSunset)
	morningStart := end.Add(-p.AfterSunset)
	morningEnd := end.Add(p.BeforeSunset)

	switch {
	case !now.Before(eveningStart) && now.Before(eveningEnd):
		st.Phase = PhaseEvening
		st.Progress = progress(now, eveningStart, eveningEnd)
		st.Kelvin = blend(p.DayKelvin, p.NightKelvin, st.Progress)
		st.NextChange = eveningEnd
	case !now.Before(eveningEnd) && now.Before(morningStart):
		st.Phase = PhaseNight
		st.Progress = 1
		st.Kelvin = p.NightKelvin
		st.NextChange = morningStart
	case !now.Before(morningStart) && now.Before(morningEnd):
		st.Phase = PhaseMorning
		st.Progress = 1 - progress(now, morningStart, morningEnd)
		st.Kelvin = blend(p.DayKelvin, p.NightKelvin, st.Progress)
		st.NextChange = morningEnd
	}
	return st
}
