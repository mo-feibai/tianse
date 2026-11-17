// Package suncalc computes sunrise and sunset for a location on a given day
// using Meeus' solar equations. The result agrees with sunrise-sunset.org to
// within a minute, which is far below what a colour-temperature ramp needs.
package suncalc

import (
	"errors"
	"math"
	"time"
)

// ErrNoSunriseSunset is returned for dates on which the sun never crosses the
// horizon: polar day (no sunset) or polar night (no sunrise).
var ErrNoSunriseSunset = errors.New("suncalc: no sunrise or sunset on this date")

// Location is a geographic position. Longitude is positive east.
type Location struct {
	Latitude  float64
	Longitude float64
}

// Day holds the events for one calendar day, in UTC.
type Day struct {
	Sunrise    time.Time
	Sunset     time.Time
	PolarDay   bool
	PolarNight bool
}

const (
	// Solar zenith used by the official sunrise/sunset definition: the sun's
	// centre 50 arc-minutes below the horizon, plus 34 arc-minutes of
	// refraction.
	officialZenith = 90.833
	obliquity      = 23.4397
)

// JulianFromTime converts an instant to a Julian date.
func JulianFromTime(t time.Time) float64 {
	return float64(t.UTC().UnixNano())/86400e9 + 2440587.5
}

// TimeFromJulian converts a Julian date back to an instant.
func TimeFromJulian(jd float64) time.Time {
	secs := (jd - 2440587.5) * 86400.0
	return time.Unix(int64(math.Round(secs)), 0).UTC()
}

// ForDay returns the sunrise and sunset that fall on the UTC calendar day
// containing date. For a location in a far-eastern timezone the local evening
// may belong to the previous UTC day, so callers that need "the sunset before
// now" should scan several neighbouring days.
func ForDay(date time.Time, loc Location) (Day, error) {
	d := date.UTC()
	y, m, dd := d.Date()
	jd0 := JulianFromTime(time.Date(y, m, dd, 0, 0, 0, 0, time.UTC))

	// Day number relative to J2000, offset by 0.0008 for the leap-second-free
	// TT/UT difference.
	n := math.Round(jd0 - 2451545.0 + 0.0008)

	// Mean solar noon, with longitude folded in. East longitude means the sun
	// transits earlier in UTC, so the offset is subtracted: 120 E puts noon at
	// 04:00 UTC, a third of a day before the 12:00 UTC reference.
	jStar := n - loc.Longitude/360.0

	meanAnomaly := math.Mod(357.5291+0.98560028*jStar, 360)
	M := meanAnomaly * math.Pi / 180
	centre := 1.9148*math.Sin(M) + 0.0200*math.Sin(2*M) + 0.0003*math.Sin(3*M)
	lambda := math.Mod(meanAnomaly+centre+180+102.9372, 360)
	L := lambda * math.Pi / 180

	transit := 2451545.0 + jStar + 0.0053*math.Sin(M) - 0.0069*math.Sin(2*L)

	sinDec := math.Sin(L) * math.Sin(obliquity*math.Pi/180)
	dec := math.Asin(sinDec)

	phi := loc.Latitude * math.Pi / 180
	// The numerator is the sine of the sun's altitude at the event, which for
	// the official definition is -0.833 degrees. Note sin(-0.833) equals
	// cos(90.833): writing sin(-90.833) here would put the sun a full quarter
	// turn off and every location would look polar.
	sinAltitude := math.Cos(officialZenith * math.Pi / 180)
	cosHourAngle := (sinAltitude - math.Sin(phi)*sinDec) /
		(math.Cos(phi) * math.Cos(dec))

	day := Day{}
	if cosHourAngle > 1 {
		// The sun never rises.
		day.PolarNight = true
		return day, ErrNoSunriseSunset
	}
	if cosHourAngle < -1 {
		// The sun never sets.
		day.PolarDay = true
		return day, ErrNoSunriseSunset
	}

	hourAngle := math.Acos(cosHourAngle) * 180 / math.Pi
	day.Sunrise = TimeFromJulian(transit - hourAngle/360)
	day.Sunset = TimeFromJulian(transit + hourAngle/360)
	return day, nil
}

// ForLocalDay returns the events of the calendar day that date falls on in its
// own location. ForDay works in UTC days, which is the wrong day for anywhere
// east of Greenwich after 08:00 local.
func ForLocalDay(date time.Time, loc Location) (Day, error) {
	y, m, d := date.Date()
	return ForDay(time.Date(y, m, d, 12, 0, 0, 0, time.UTC), loc)
}

// Cycle returns the sunset and the sunrise that bracket now: the nearest
// sunset in time and the nearest sunrise in time.
//
// This is the primitive the engine needs, and "the last sunset before now" is
// not it. At 15:00 on a day whose sunset is 17:00 the evening ramp has already
// begun, so the relevant sunset is the one ahead, not yesterday's. Taking the
// nearest event of each kind gives the same answer for every instant, including
// just after midnight, without any special cases at the boundaries.
func Cycle(now time.Time, loc Location) (sunset, sunrise time.Time, err error) {
	var (
		bestSet, bestRise   time.Time
		setDist, riseDist   time.Duration
		foundSet, foundRise bool
	)
	for k := -2; k <= 2; k++ {
		day, dayErr := ForLocalDay(now.AddDate(0, 0, k), loc)
		if dayErr != nil {
			continue
		}
		if d := absDuration(now.Sub(day.Sunset)); !foundSet || d < setDist {
			bestSet, setDist, foundSet = day.Sunset, d, true
		}
		if d := absDuration(now.Sub(day.Sunrise)); !foundRise || d < riseDist {
			bestRise, riseDist, foundRise = day.Sunrise, d, true
		}
	}
	if !foundSet || !foundRise {
		return time.Time{}, time.Time{}, ErrNoSunriseSunset
	}
	return bestSet, bestRise, nil
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// SunsetBefore returns the latest sunset strictly before now, scanning back up
// to three days to absorb timezone and longitude offsets.
func SunsetBefore(now time.Time, loc Location) (time.Time, error) {
	for back := 0; back <= 3; back++ {
		day, err := ForDay(now.AddDate(0, 0, -back), loc)
		if err != nil {
			continue
		}
		if day.Sunset.Before(now) {
			return day.Sunset, nil
		}
	}
	return time.Time{}, ErrNoSunriseSunset
}

// SunriseBefore returns the latest sunrise strictly before now.
func SunriseBefore(now time.Time, loc Location) (time.Time, error) {
	for back := 0; back <= 3; back++ {
		day, err := ForLocalDay(now.AddDate(0, 0, -back), loc)
		if err != nil {
			continue
		}
		if day.Sunrise.Before(now) {
			return day.Sunrise, nil
		}
	}
	return time.Time{}, ErrNoSunriseSunset
}

// SunsetAfter returns the earliest sunset strictly after now.
func SunsetAfter(now time.Time, loc Location) (time.Time, error) {
	for fwd := 0; fwd <= 3; fwd++ {
		day, err := ForLocalDay(now.AddDate(0, 0, fwd), loc)
		if err != nil {
			continue
		}
		if day.Sunset.After(now) {
			return day.Sunset, nil
		}
	}
	return time.Time{}, ErrNoSunriseSunset
}

// SunriseAfter returns the earliest sunrise strictly after now.
func SunriseAfter(now time.Time, loc Location) (time.Time, error) {
	for fwd := 0; fwd <= 3; fwd++ {
		day, err := ForDay(now.AddDate(0, 0, fwd), loc)
		if err != nil {
			continue
		}
		if day.Sunrise.After(now) {
			return day.Sunrise, nil
		}
	}
	return time.Time{}, ErrNoSunriseSunset
}
