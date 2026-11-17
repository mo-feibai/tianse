// Package screen applies colour temperature and brightness to the physical
// displays attached to this machine.
//
// Two mechanisms are used. Gamma ramps are the universal fallback: they are a
// pure GDI operation that every adapter honours. DDC/CI is the better one for
// brightness because it changes the backlight instead of crushing the signal,
// but it is slow (tens of milliseconds per call) and some panels ignore it.
package screen

// Display describes one physical output.
type Display struct {
	// Device is the GDI device name, for example "\\.\DISPLAY1".
	Device string
	// Description is the human-readable name reported by the monitor, if any.
	Description string
	// Primary marks the display Windows treats as the main one.
	Primary bool
	// Width and Height are the current mode dimensions in pixels.
	Width, Height int
	// BrightnessSupported reports whether DDC/CI brightness worked at probe
	// time.
	BrightnessSupported bool
	// BrightnessMin, BrightnessCurrent and BrightnessMax are raw DDC values.
	BrightnessMin, BrightnessCurrent, BrightnessMax uint32
	// BrightnessErr carries why the probe failed, so the diagnostic report can
	// tell "the panel ignores DDC/CI" apart from "the handle never opened".
	BrightnessErr string
}

// Target is the desired state of one display.
type Target struct {
	Device string
	Ramp   [3][256]uint16
	// Brightness is a fraction of the display's own maximum. A negative value
	// means "leave brightness alone".
	Brightness float64
}

// Result reports what actually happened, so the engine can tell the UI whether
// brightness is under control or only the gamma is.
type Result struct {
	Device       string
	GammaOK      bool
	BrightnessOK bool
	Err          error
}
