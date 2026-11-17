package service

import (
	"time"

	"tianse/internal/clock"
	"tianse/internal/color"
	"tianse/internal/curve"
	"tianse/internal/suncalc"
)

// AxisStop is one colour stop on the sky axis.
type AxisStop struct {
	// Offset is the position along the axis, 0 at the left end and 1 at the
	// right end. The ends are the shoulders of the curve, not sunset and
	// sunrise: see Axis.SunsetOffset.
	Offset float64
	// Color is the sRGB hex the band shows at that moment.
	Color string
	// Kelvin is the temperature at that moment. The frontend needs the number,
	// not just the colour, because dragging the axis previews the reading.
	Kelvin float64
}

// AxisTick is an hour mark on the band.
type AxisTick struct {
	Offset float64
	Label  string
}

// Axis is everything the sky axis needs to draw itself.
//
// The gradient is computed here rather than in TypeScript so that the band and
// the screen tint are literally the same function called twice: whatever the
// user sees on the axis is what the display is being given.
type Axis struct {
	Stops []AxisStop
	// Ticks are the two-hour marks, placed by clock rather than evenly, so the
	// band shows how long the night actually is.
	Ticks []AxisTick
	// Now is the current position along the axis, 0 at the left end and 1 at
	// the right end.
	Now float64
	// NowText is the clock time at Now, for the middle label under the band.
	NowText string
	// Start and End are display strings for the two ends of the axis.
	Start string
	End   string
	// SunsetOffset and SunriseOffset mark where the sun actually crosses the
	// horizon inside the band. The axis spans the shoulders of the curve —
	// two hours before sunset to two hours after sunrise — so the sun's own
	// crossings are interior marks, not the ends of the band. Without them the
	// band would show a ramp with no labelled beginning.
	SunsetOffset  float64
	SunriseOffset float64
	// SunsetText and SunriseText are the clock times of those two crossings.
	SunsetText  string
	SunriseText string
	// Polar is set on days with no sunset or sunrise; the axis is then a single
	// flat band and Now is meaningless.
	Polar bool
	// PolarColor is the flat band colour when Polar is set.
	PolarColor string
	// Empty is set when the axis cannot be built at all (no location, bad date).
	Empty bool
}

const (
	axisStops = 130
	// axisBandLumDark is the band's relative luminance on the dark surface. The
	// band is a surface, not a light source: at 0.075 it sits about 2.1:1 above
	// the sunk card it rests on, so the gradient reads as colour without becoming
	// the brightest thing in the window. 0.30 looked like a strip of cloth laid on
	// the panel and 0.13 was still the first thing the eye went to — DESIGN 5.2.
	axisBandLumDark = 0.075
	// axisBandLumLight is the same rule against the light card (#F0F4F4, relative
	// luminance 0.898). The direction flips — on paper the band has to be the
	// darker object — but 2.1:1 (which would be 0.40) leaves the warm end a muddy
	// grey-brown, so the light band sits at 0.42 with chroma pushed back up:
	// normalising upward squeezes the channels together and eats the hue travel.
	axisBandLumLight = 0.42
	// axisBandChromaLight restores that travel. 1.3 measured on the axis: the band
	// runs cool grey to amber instead of grey to grey-brown, and noon is still the
	// palest point rather than a hole.
	axisBandChromaLight = 1.3
)

// Axis returns the sky band for the current day.
func (s *Service) Axis() Axis {
	cfg := s.store.Snapshot()
	loc := suncalc.Location{Latitude: cfg.Location.Latitude, Longitude: cfg.Location.Longitude}
	params := curve.Params{
		DayKelvin:    cfg.Temp.Day,
		NightKelvin:  cfg.Temp.Night,
		BeforeSunset: time.Duration(cfg.Temp.StartBeforeSunsetMin) * time.Minute,
		AfterSunset:  time.Duration(cfg.Temp.SettleAfterSunsetMin) * time.Minute,
	}

	now := time.Now()
	lum, chroma := axisBandLumDark, 1.0
	if !s.engine.Status().SurfaceDark {
		lum, chroma = axisBandLumLight, axisBandChromaLight
	}
	sunset, err := suncalc.SunsetBefore(now, loc)
	if err != nil {
		return s.polarAxis(params, now, lum, chroma)
	}
	sunrise, err := suncalc.SunriseAfter(now, loc)
	if err != nil {
		return s.polarAxis(params, now, lum, chroma)
	}
	if !sunrise.After(sunset) {
		return s.polarAxis(params, now, lum, chroma)
	}

	// The band spans the shoulders of the curve rather than the night itself.
	// Sunset to sunrise leaves about three quarters of the width flat at the
	// night temperature, because the ramp finishes 30 minutes after sunset — the
	// band would then be a night indicator, not a picture of the sky changing.
	start := sunset.Add(-params.BeforeSunset)
	end := sunrise.Add(params.BeforeSunset)
	span := end.Sub(start)

	out := Axis{
		Start: clock.HM(start),
		End:   clock.HM(end),
	}
	out.SunsetOffset = float64(sunset.Sub(start)) / float64(span)
	out.SunriseOffset = float64(sunrise.Sub(start)) / float64(span)
	out.SunsetText = clock.HM(sunset)
	out.SunriseText = clock.HM(sunrise)

	out.Stops = make([]AxisStop, 0, axisStops)
	for i := 0; i < axisStops; i++ {
		frac := float64(i) / float64(axisStops-1)
		t := start.Add(time.Duration(frac * float64(span)))
		st := curve.Evaluate(t, loc, params)
		out.Stops = append(out.Stops, AxisStop{
			Offset: frac,
			Color:  bandColor(st.Kelvin, lum, chroma),
			Kelvin: st.Kelvin,
		})
	}

	out.Ticks = buildTicks(start, end, span)

	switch {
	case now.Before(start):
		out.Now = 0
	case now.After(end):
		out.Now = 1
	default:
		out.Now = float64(now.Sub(start)) / float64(span)
	}
	// The label follows the marker, not the wall clock: outside the band the
	// marker sits on an edge and the label has to say which edge it is.
	out.NowText = clock.HM(start.Add(time.Duration(out.Now * float64(span))))
	return out
}

// buildTicks places a mark on every even hour inside the band.
func buildTicks(start, end time.Time, span time.Duration) []AxisTick {
	var out []AxisTick
	t := clock.TruncateHour(start).Add(time.Hour)
	for t.Before(end) {
		if t.After(start) {
			out = append(out, AxisTick{
				Offset: float64(t.Sub(start)) / float64(span),
				Label:  clock.HM(t),
			})
		}
		t = t.Add(time.Hour)
	}
	// Two-hour spacing: drop every other mark so the band is not ruled paper.
	sparse := make([]AxisTick, 0, len(out)/2+1)
	for i := 0; i < len(out); i += 2 {
		sparse = append(sparse, out[i])
	}
	return sparse
}

// polarAxis handles the days where the sun never crosses the horizon. Inside the
// polar circles the whole curve concept collapses to a single temperature, so the
// band becomes one flat colour and the UI says so.
func (s *Service) polarAxis(params curve.Params, now time.Time, lum, chroma float64) Axis {
	st := curve.Evaluate(now, suncalc.Location{}, params)
	kelvin := params.DayKelvin
	if st.Phase == curve.PhaseNight {
		kelvin = params.NightKelvin
	}
	return Axis{Polar: true, PolarColor: bandColor(kelvin, lum, chroma), Now: 0.5, NowText: clock.HM(now)}
}

// bandColor turns a colour temperature into a swatch at a fixed perceived
// brightness. Without the brightness normalisation the night end of the band
// would be so dark it disappears into the card.
func bandColor(kelvin, lum, chroma float64) string {
	r, g, b := color.TempToRGB(kelvin)
	return color.NormaliseChromaHex(r, g, b, lum, chroma)
}
