package color

import "math"

// NormaliseHex scales an sRGB triple so its relative luminance equals target and
// returns it as #rrggbb.
//
// The sky axis uses this to keep the band readable at both ends of the day: a
// 3400 K swatch straight from TempToRGB is nearly black, and on a dark card it
// would read as a hole rather than as evening. Scaling by luminance rather than
// by the maximum channel keeps the hue while changing only the depth.
func NormaliseHex(r, g, b, target float64) string {
	src := RelativeLuminance(clampByte(r*255), clampByte(g*255), clampByte(b*255))
	if src <= 0.0001 {
		return "#000000"
	}
	// sRGB transfer is close to a 2.2 power law, so a luminance ratio raises to
	// 1/2.2 per channel. Three passes are enough to absorb the piecewise segment
	// below 0.04045.
	k := math.Pow(target/src, 1.0/2.2)
	for i := 0; i < 3; i++ {
		rr, gg, bb := clampByte(r*255*k), clampByte(g*255*k), clampByte(b*255*k)
		got := RelativeLuminance(rr, gg, bb)
		if got <= 0.0001 {
			break
		}
		k *= math.Pow(target/got, 1.0/2.2)
	}
	return rgbToHex(clampByte(r*255*k), clampByte(g*255*k), clampByte(b*255*k))
}

// NormaliseChromaHex is NormaliseHex with a chroma multiplier.
//
// Scaling a warm blackbody colour up to a light-surface luminance squeezes every
// channel towards the same grey — at 3400 K the blue channel is already low, so
// raising all three proportionally leaves almost no separation between noon and
// midnight. The dark band does not have this problem because it normalises
// downward, which widens the ratios.
//
// boost pushes each channel away from the grey of the target luminance, in
// encoded sRGB space, so the band keeps the hue travel it has on the dark theme
// at the same luminance. boost = 1 is exactly NormaliseHex.
func NormaliseChromaHex(r, g, b, target, boost float64) string {
	if boost <= 1 {
		return NormaliseHex(r, g, b, target)
	}
	hex := NormaliseHex(r, g, b, target)
	cr, cg, cb := hexToRGB(hex)
	// The grey of this luminance, encoded: the point chroma is measured from.
	grey := 255 * math.Pow(target, 1.0/2.2)
	push := func(c uint8) uint8 {
		return clampByte(grey + (float64(c)-grey)*boost)
	}
	return rgbToHex(push(cr), push(cg), push(cb))
}
