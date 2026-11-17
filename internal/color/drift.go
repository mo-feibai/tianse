package color

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Anchor is one colour stop of a drift table. Kelvin descends through the
// tables so that lookup can walk from the cool end towards the warm end.
type Anchor struct {
	Kelvin float64
	Hex    string
}

// Palette is the set of colours the UI needs at one instant. Every field is a
// CSS source colour: it already includes the inverse of the current display
// gain, so the frontend can assign it to a custom property verbatim.
type Palette struct {
	Accent string
	Ink    string
	Xiang  string
}

// Dark anchors. Accent walks from teal (H187) at 6500K to sea green (H159) at
// 3400K, the only hue direction reachable once the blue channel gain collapses.
var (
	darkAccent = []Anchor{
		{6500, "#5AA5B2"},
		{5725, "#4FAFB9"},
		{4950, "#4DBCC4"},
		{4175, "#4BCDD7"},
		{3400, "#49E4FF"},
	}
	darkInk = []Anchor{
		{6500, "#7F919F"},
		{5725, "#8397A5"},
		{4950, "#899EAD"},
		{4175, "#91A7B7"},
		{3400, "#9CB3C4"},
	}
	lightAccent = []Anchor{
		{6500, "#2B7780"},
		{5725, "#2B7680"},
		{4950, "#2B767F"},
		{4175, "#2A757E"},
		{3400, "#2A737C"},
	}
	lightXiang = []Anchor{
		{6500, "#92680F"},
		{5725, "#8E650F"},
		{4950, "#89620E"},
		{4175, "#835E0E"},
		{3400, "#7B580D"},
	}
	lightInk = []Anchor{
		{6500, "#606D74"},
		{5725, "#5F6B72"},
		{4950, "#5D6970"},
		{4175, "#5B666D"},
		{3400, "#576269"},
	}
)

// DarkXiang is the data-highlight colour on the dark surface. It is not
// compensated: even at 3400K it keeps 5.67:1 against the jade surface.
const DarkXiang = "#D9A441"

// DarkPalette returns the dark-surface palette for a colour temperature.
func DarkPalette(kelvin float64) Palette {
	return Palette{
		Accent: interpolate(darkAccent, kelvin),
		Ink:    interpolate(darkInk, kelvin),
		Xiang:  DarkXiang,
	}
}

// LightPalette returns the light-surface palette for a colour temperature.
// The light surface does not drift hue: a green accent on frost reads as a
// different product, not as evening.
func LightPalette(kelvin float64) Palette {
	return Palette{
		Accent: interpolate(lightAccent, kelvin),
		Ink:    interpolate(lightInk, kelvin),
		Xiang:  interpolate(lightXiang, kelvin),
	}
}

// interpolate walks a descending Kelvin table and blends the two neighbouring
// stops in sRGB space. Five stops keep the error under 1.2 Delta-E against the
// exact curve, which is below the just-noticeable difference.
func interpolate(table []Anchor, kelvin float64) string {
	if kelvin >= table[0].Kelvin {
		return table[0].Hex
	}
	last := table[len(table)-1]
	if kelvin <= last.Kelvin {
		return last.Hex
	}
	for i := 0; i < len(table)-1; i++ {
		hi, lo := table[i], table[i+1]
		if kelvin <= hi.Kelvin && kelvin >= lo.Kelvin {
			span := hi.Kelvin - lo.Kelvin
			t := 0.0
			if span > 0 {
				t = (hi.Kelvin - kelvin) / span
			}
			hr, hg, hb := hexToRGB(hi.Hex)
			lr, lg, lb := hexToRGB(lo.Hex)
			return rgbToHex(
				lerp(hr, lr, t),
				lerp(hg, lg, t),
				lerp(hb, lb, t),
			)
		}
	}
	return last.Hex
}

func lerp(a, b uint8, t float64) uint8 {
	v := float64(a) + (float64(b)-float64(a))*t
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return uint8(v + 0.5)
}

func hexToRGB(hex string) (uint8, uint8, uint8) {
	s := strings.TrimPrefix(hex, "#")
	if len(s) != 6 {
		return 0, 0, 0
	}
	r, err := strconv.ParseUint(s[0:2], 16, 8)
	if err != nil {
		return 0, 0, 0
	}
	g, err := strconv.ParseUint(s[2:4], 16, 8)
	if err != nil {
		return 0, 0, 0
	}
	b, err := strconv.ParseUint(s[4:6], 16, 8)
	if err != nil {
		return 0, 0, 0
	}
	return uint8(r), uint8(g), uint8(b)
}

func rgbToHex(r, g, b uint8) string {
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// Preview renders what a CSS source colour looks like once the display gain is
// applied. It exists for tests and for the settings page, not for the hot path.
func Preview(source string, kelvin float64) string {
	r, g, b := hexToRGB(source)
	gr, gg, gb := TempToRGB(kelvin)
	return rgbToHex(
		clampByte(float64(r)*gr),
		clampByte(float64(g)*gg),
		clampByte(float64(b)*gb),
	)
}

// Hue returns the hue angle of a hex colour in degrees.
func Hue(hex string) float64 {
	r, g, b := hexToRGB(hex)
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	d := max - min
	if d == 0 {
		return 0
	}
	var h float64
	switch max {
	case rf:
		h = math.Mod((gf-bf)/d, 6)
	case gf:
		h = (bf-rf)/d + 2
	default:
		h = (rf-gf)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h
}
