//go:build windows

package screen

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")
	user32 = windows.NewLazySystemDLL("user32.dll")

	procCreateDCW            = gdi32.NewProc("CreateDCW")
	procDeleteDC             = gdi32.NewProc("DeleteDC")
	procGetDeviceGammaRamp   = gdi32.NewProc("GetDeviceGammaRamp")
	procSetDeviceGammaRamp   = gdi32.NewProc("SetDeviceGammaRamp")
	procEnumDisplayDevicesW  = user32.NewProc("EnumDisplayDevicesW")
	procEnumDisplaySettingsW = user32.NewProc("EnumDisplaySettingsW")
	procEnumDisplayMonitors  = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW      = user32.NewProc("GetMonitorInfoW")
	procGetDeviceCaps        = gdi32.NewProc("GetDeviceCaps")
)

const (
	displayDeviceAttachedToDesktop = 0x00000001
	displayDevicePrimaryDevice     = 0x00000004
	enumCurrentSettings            = 0xFFFFFFFF
	horzRes                        = 8
	vertRes                        = 10
)

type displayDeviceW struct {
	Cb           uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

type devModeW struct {
	DeviceName         [32]uint16
	SpecVersion        uint16
	DriverVersion      uint16
	Size               uint16
	DriverExtra        uint16
	Fields             uint32
	PositionX          int32
	PositionY          int32
	DisplayOrientation uint32
	DisplayFixedOutput uint32
	ColorPlanes        uint16
	BitsPerPixel       uint16
	PelsWidth          uint32
	PelsHeight         uint32
	DisplayFlags       uint32
	DisplayFrequency   uint32
	ICMMethod          uint32
	ICMIntent          uint32
	MediaType          uint32
	DitherType         uint32
	Reserved1          uint32
	Reserved2          uint32
	PanningWidth       uint32
	PanningHeight      uint32
}

// hdcCache keeps one device context per display. Creating a DC per frame would
// leak handles and is measurably slower than reusing them.
type hdcCache struct {
	mu  sync.Mutex
	dcs map[string]windows.Handle
}

func newHDCCache() *hdcCache {
	return &hdcCache{dcs: map[string]windows.Handle{}}
}

func (c *hdcCache) get(device string) (windows.Handle, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if h, ok := c.dcs[device]; ok {
		return h, nil
	}
	name, err := windows.UTF16PtrFromString(device)
	if err != nil {
		return 0, err
	}
	driver, err := windows.UTF16PtrFromString("DISPLAY")
	if err != nil {
		return 0, err
	}
	// CreateDCW("DISPLAY", device, NULL, NULL) is the documented way to obtain
	// a DC bound to one output. Using the primary DC would apply the ramp to
	// the wrong adapter on multi-GPU systems.
	r, _, e := procCreateDCW.Call(
		uintptr(unsafe.Pointer(driver)),
		uintptr(unsafe.Pointer(name)),
		0,
		0,
	)
	if r == 0 {
		return 0, fmt.Errorf("CreateDCW(%s): %w", device, e)
	}
	c.dcs[device] = windows.Handle(r)
	return windows.Handle(r), nil
}

func (c *hdcCache) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for device, h := range c.dcs {
		procDeleteDC.Call(uintptr(h))
		delete(c.dcs, device)
	}
}

// enumerateDisplays lists the outputs attached to the desktop.
func enumerateDisplays() ([]Display, error) {
	var out []Display
	for i := uint32(0); ; i++ {
		var dd displayDeviceW
		dd.Cb = uint32(unsafe.Sizeof(dd))
		r, _, _ := procEnumDisplayDevicesW.Call(
			0,
			uintptr(i),
			uintptr(unsafe.Pointer(&dd)),
			0,
		)
		if r == 0 {
			break
		}
		if dd.StateFlags&displayDeviceAttachedToDesktop == 0 {
			continue
		}
		device := windows.UTF16ToString(dd.DeviceName[:])
		d := Display{
			Device:  device,
			Primary: dd.StateFlags&displayDevicePrimaryDevice != 0,
			// DeviceString on the adapter entry is the graphics card, not the
			// panel: the status card read "NVIDIA GeForce GTX 1650" where the
			// design calls for the display's own name. Enumerating children of
			// the adapter entry yields the monitor ("XMI Redmi Monitor").
			Description: monitorName(device),
		}
		if d.Description == "" {
			d.Description = windows.UTF16ToString(dd.DeviceString[:])
		}
		if w, h, ok := currentMode(device); ok {
			d.Width, d.Height = w, h
		}
		out = append(out, d)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no display attached to the desktop")
	}
	for i := range out {
		out[i].Description = displayLabel(i, out[i].Description)
	}
	return out, nil
}

// displayLabel turns whatever the monitor reports into a name the interface can
// print, falling back to the display's ordinal.
//
// Windows answers "Generic PnP Monitor" for any panel whose EDID name it could
// not read — which is most panels reached over a KVM, a capture card, or a
// virtual adapter, and also this machine's own monitor. That string is a
// placeholder, not a name: printing it in the status card tells the user
// nothing they can act on. An ordinal ("显示器 1") at least maps to the number
// Windows itself shows in Display Settings.
func displayLabel(index int, desc string) string {
	switch desc {
	case "", "Generic PnP Monitor", "Generic Non-PnP Monitor", "Default Monitor":
		return fmt.Sprintf("显示器 %d", index+1)
	}
	return desc
}

// monitorName asks the adapter for its first child device, which is the panel.
func monitorName(adapter string) string {
	name, err := windows.UTF16PtrFromString(adapter)
	if err != nil {
		return ""
	}
	var md displayDeviceW
	md.Cb = uint32(unsafe.Sizeof(md))
	r, _, _ := procEnumDisplayDevicesW.Call(
		uintptr(unsafe.Pointer(name)),
		0,
		uintptr(unsafe.Pointer(&md)),
		0,
	)
	if r == 0 {
		return ""
	}
	return windows.UTF16ToString(md.DeviceString[:])
}

func currentMode(device string) (int, int, bool) {
	var dm devModeW
	dm.Size = uint16(unsafe.Sizeof(dm))
	name, err := windows.UTF16PtrFromString(device)
	if err != nil {
		return 0, 0, false
	}
	r, _, _ := procEnumDisplaySettingsW.Call(
		uintptr(unsafe.Pointer(name)),
		uintptr(enumCurrentSettings),
		uintptr(unsafe.Pointer(&dm)),
	)
	if r != 0 && dm.PelsWidth > 0 {
		return int(dm.PelsWidth), int(dm.PelsHeight), true
	}
	// EnumDisplaySettings answers nothing in a session without a real desktop
	// even though the device is enumerable; the DC still reports its surface.
	dc, _, _ := procCreateDCW.Call(uintptr(unsafe.Pointer(name)), 0, 0, 0)
	if dc == 0 {
		return 0, 0, false
	}
	defer procDeleteDC.Call(dc)
	w, _, _ := procGetDeviceCaps.Call(dc, horzRes)
	h, _, _ := procGetDeviceCaps.Call(dc, vertRes)
	if w == 0 || h == 0 {
		return 0, 0, false
	}
	return int(w), int(h), true
}

func readGamma(h windows.Handle) ([3][256]uint16, error) {
	var ramp [3][256]uint16
	r, _, e := procGetDeviceGammaRamp.Call(uintptr(h), uintptr(unsafe.Pointer(&ramp)))
	if r == 0 {
		return ramp, fmt.Errorf("GetDeviceGammaRamp: %s", rampError(e))
	}
	return ramp, nil
}

func writeGamma(h windows.Handle, ramp *[3][256]uint16) error {
	r, _, e := procSetDeviceGammaRamp.Call(uintptr(h), uintptr(unsafe.Pointer(ramp)))
	if r == 0 {
		return fmt.Errorf("SetDeviceGammaRamp: %s", rampError(e))
	}
	return nil
}

// rampError describes a rejected ramp in words that match what happened.
//
// The gamma calls return 0 on failure but the display driver leaves the
// last-error slot untouched, so the syscall error is ERROR_SUCCESS and printing
// it produces "SetDeviceGammaRamp: The operation completed successfully." — a
// failure reported as a success, once per second, forever. An adapter refuses a
// ramp when the colour shift is larger than it allows; that is the only cause
// seen in practice, so it is what the message says.
func rampError(e error) string {
	if e == nil || errors.Is(e, windows.ERROR_SUCCESS) {
		return "the adapter refused the ramp without setting an error code (the colour shift is larger than this driver allows)"
	}
	return e.Error()
}
