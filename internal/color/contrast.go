package color

import "math"

// RelativeLuminance is the WCAG 2.1 relative luminance of an sRGB colour.
func RelativeLuminance(r, g, b uint8) float64 {
	lin := func(c uint8) float64 {
		s := float64(c) / 255.0
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// ContrastRatio is the WCAG contrast ratio between two hex colours.
func ContrastRatio(fg, bg string) float64 {
	fr, fg2, fb := hexToRGB(fg)
	br, bg2, bb := hexToRGB(bg)
	l1 := RelativeLuminance(fr, fg2, fb)
	l2 := RelativeLuminance(br, bg2, bb)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}
