// Command gen-icon draws the application icon.
//
// It is a generator rather than a checked-in asset so the icon and the tray
// glyph cannot drift apart: both draw the same idea — a sun half sunk below a
// horizon — from the same palette.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// Palette, taken from DESIGN 2.1.
var (
	bg      = color.NRGBA{R: 0x14, G: 0x17, B: 0x1C, A: 0xFF}
	sun     = color.NRGBA{R: 0xD9, G: 0xA4, B: 0x41, A: 0xFF}
	horizon = color.NRGBA{R: 0xE8, G: 0xEE, B: 0xF0, A: 0xFF}
)

// ss is the supersampling factor. Every edge in the glyph is a circle or a
// rectangle, so plain 4x4 sampling is indistinguishable from an analytic
// coverage integral at these sizes.
const ss = 4

// draw renders one square icon at the given edge length.
func draw(px int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, px, px))
	n := float64(px)

	// Geometry in unit coordinates, then scaled. The corner radius is large
	// enough to read as a rounded tile at 256 and still survive at 16.
	const (
		radiusFrac = 0.225
		sunCx      = 0.5
		sunCy      = 0.545
		sunRFrac   = 0.265
		horizonY   = 0.545
		lineHalf   = 0.026
		lineFrom   = 0.175
		lineTo     = 0.825
	)

	radius := radiusFrac * n
	sunR := sunRFrac * n

	insideTile := func(x, y float64) bool {
		// Rounded rectangle: clamp the sample into the inner rectangle, then
		// compare distance to the corner radius.
		cx := math.Min(math.Max(x, radius), n-radius)
		cy := math.Min(math.Max(y, radius), n-radius)
		dx, dy := x-cx, y-cy
		return dx*dx+dy*dy <= radius*radius
	}

	for py := 0; py < px; py++ {
		for pxi := 0; pxi < px; pxi++ {
			var bgHits, sunHits, lineHits int
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := float64(pxi) + (float64(sx)+0.5)/ss
					y := float64(py) + (float64(sy)+0.5)/ss
					if !insideTile(x, y) {
						continue
					}
					bgHits++

					// Sun: the disc, clipped to the part above the horizon.
					dx := x - sunCx*n
					dy := y - sunCy*n
					if dx*dx+dy*dy <= sunR*sunR && y <= horizonY*n {
						sunHits++
						continue
					}

					// Horizon: a bar spanning most of the tile.
					if math.Abs(y-horizonY*n) <= lineHalf*n && x >= lineFrom*n && x <= lineTo*n {
						lineHits++
					}
				}
			}

			total := float64(ss * ss)
			if bgHits == 0 {
				continue
			}

			// Composite sun over horizon over background, weighted by coverage.
			out := blend(bg, sun, float64(sunHits)/float64(bgHits))
			out = blend(out, horizon, float64(lineHits)/float64(bgHits))
			out.A = uint8(math.Round(float64(bgHits) / total * 255))
			img.SetNRGBA(pxi, py, out)
		}
	}
	return img
}

// blend mixes src over dst with the given alpha fraction, ignoring src's own
// alpha (every palette colour here is opaque).
func blend(dst, src color.NRGBA, a float64) color.NRGBA {
	mix := func(d, s uint8) uint8 {
		return uint8(math.Round(float64(d)*(1-a) + float64(s)*a))
	}
	return color.NRGBA{R: mix(dst.R, src.R), G: mix(dst.G, src.G), B: mix(dst.B, src.B), A: dst.A}
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// writeICO packs the sizes into a single .ico. Every entry is stored as PNG,
// which Windows has accepted since Vista and which keeps the file small.
func writeICO(path string, sizes []int) error {
	type entry struct {
		size int
		data []byte
	}
	entries := make([]entry, 0, len(sizes))
	for _, s := range sizes {
		entries = append(entries, entry{size: s, data: encodePNG(draw(s))})
	}

	var buf bytes.Buffer
	// ICONDIR
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(len(entries)))

	offset := 6 + 16*len(entries)
	for _, e := range entries {
		dim := byte(e.size)
		if e.size >= 256 {
			dim = 0 // 0 means 256
		}
		buf.WriteByte(dim)
		buf.WriteByte(dim)
		buf.WriteByte(0)                                    // palette size
		buf.WriteByte(0)                                    // reserved
		binary.Write(&buf, binary.LittleEndian, uint16(1))  // planes
		binary.Write(&buf, binary.LittleEndian, uint16(32)) // bits per pixel
		binary.Write(&buf, binary.LittleEndian, uint32(len(e.data)))
		binary.Write(&buf, binary.LittleEndian, uint32(offset))
		offset += len(e.data)
	}
	for _, e := range entries {
		buf.Write(e.data)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	// appicon.png is what the Wails task uses to regenerate icon.ico; keeping
	// it in step means either path produces the same icon.
	pngPath := filepath.Join(root, "build", "appicon.png")
	if err := os.MkdirAll(filepath.Dir(pngPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(pngPath, encodePNG(draw(512)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	icoPath := filepath.Join(root, "build", "windows", "icon.ico")
	if err := os.MkdirAll(filepath.Dir(icoPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := writeICO(icoPath, []int{16, 24, 32, 48, 64, 128, 256}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("wrote", pngPath)
	fmt.Println("wrote", icoPath)
}
