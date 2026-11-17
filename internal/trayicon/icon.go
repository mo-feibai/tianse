// Package trayicon renders the tray glyph at runtime.
//
// The icon is drawn instead of shipped as a file so the three states (normal,
// paused, failed) always agree with each other and the shape can be tuned in
// one place. It is a half-sunk sun over a horizon line: the product's whole idea
// in 16 pixels.
package trayicon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// State selects which glyph to draw.
type State int

const (
	// Normal is the everyday icon.
	Normal State = iota
	// Paused is the same glyph at 40% opacity.
	Paused
	// Failed adds a break to the horizon line.
	Failed
)

const size = 32

// Render returns a PNG for the given state. Dark selects the variant drawn in
// white, for use on a dark taskbar.
func Render(state State, dark bool) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	ink := color.NRGBA{R: 0x14, G: 0x17, B: 0x1C, A: 0xFF}
	if dark {
		ink = color.NRGBA{R: 0xF2, G: 0xF6, B: 0xF8, A: 0xFF}
	}
	if state == Paused {
		ink.A = 0x66
	}

	horizon := 19.0
	const sunRadius = 6.4
	cx, cy := 16.0, horizon

	// Sun disc: only the part above the horizon is filled, so the glyph reads as
	// "setting" rather than "circle with a line through it".
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			if dx*dx+dy*dy <= sunRadius*sunRadius && float64(y)+0.5 <= horizon {
				img.SetNRGBA(x, y, ink)
			}
		}
	}

	// Horizon: two pixels tall, broken in the middle when the write path failed.
	lineTop, lineBottom := int(horizon)-1, int(horizon)+1
	for x := 3; x < size-3; x++ {
		if state == Failed && x >= 13 && x <= 18 {
			continue
		}
		for y := lineTop; y < lineBottom; y++ {
			img.SetNRGBA(x, y, ink)
		}
	}

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	_ = enc.Encode(&buf, img)
	return buf.Bytes()
}
