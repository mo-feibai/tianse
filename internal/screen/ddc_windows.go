//go:build windows

package screen

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	dxva2 = windows.NewLazySystemDLL("dxva2.dll")

	procGetNumberOfPhysicalMonitors = dxva2.NewProc("GetNumberOfPhysicalMonitorsFromHMONITOR")
	procGetPhysicalMonitors         = dxva2.NewProc("GetPhysicalMonitorsFromHMONITOR")
	procDestroyPhysicalMonitors     = dxva2.NewProc("DestroyPhysicalMonitors")
	procGetMonitorBrightness        = dxva2.NewProc("GetMonitorBrightness")
	procSetMonitorBrightness        = dxva2.NewProc("SetMonitorBrightness")
)

type physicalMonitor struct {
	Handle      windows.Handle
	Description [128]uint16
}

// ddcState caches one physical monitor handle per display. Opening the handle
// costs a DDC round trip, so it is done once and reused until the display set
// changes.
type ddcState struct {
	mu       sync.Mutex
	handles  map[string]*ddcHandle
	disabled map[string]bool
}

type ddcHandle struct {
	hmonitor windows.Handle
	count    uint32
	monitors []physicalMonitor
}

func newDDCState() *ddcState {
	return &ddcState{
		handles:  map[string]*ddcHandle{},
		disabled: map[string]bool{},
	}
}

func (s *ddcState) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, h := range s.handles {
		if h.count > 0 {
			procDestroyPhysicalMonitors.Call(
				uintptr(h.count),
				uintptr(unsafe.Pointer(&h.monitors[0])),
			)
		}
	}
	s.handles = map[string]*ddcHandle{}
}

func (s *ddcState) open(device string) (*ddcHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if h, ok := s.handles[device]; ok {
		return h, nil
	}
	if s.disabled[device] {
		return nil, fmt.Errorf("ddc disabled for %s", device)
	}
	hmon, err := hmonitorForDevice(device)
	if err != nil {
		s.disabled[device] = true
		return nil, err
	}
	var count uint32
	r, _, e := procGetNumberOfPhysicalMonitors.Call(uintptr(hmon), uintptr(unsafe.Pointer(&count)))
	if r == 0 || count == 0 {
		s.disabled[device] = true
		return nil, fmt.Errorf("GetNumberOfPhysicalMonitorsFromHMONITOR(%s): %w", device, e)
	}
	monitors := make([]physicalMonitor, count)
	r, _, e = procGetPhysicalMonitors.Call(
		uintptr(hmon),
		uintptr(count),
		uintptr(unsafe.Pointer(&monitors[0])),
	)
	if r == 0 {
		s.disabled[device] = true
		return nil, fmt.Errorf("GetPhysicalMonitorsFromHMONITOR(%s): %w", device, e)
	}
	h := &ddcHandle{hmonitor: hmon, count: count, monitors: monitors}
	s.handles[device] = h
	return h, nil
}

func (s *ddcState) getBrightness(device string) (min, cur, max uint32, err error) {
	h, err := s.open(device)
	if err != nil {
		return 0, 0, 0, err
	}
	mon := h.monitors[0]
	r, _, e := procGetMonitorBrightness.Call(
		uintptr(mon.Handle),
		uintptr(unsafe.Pointer(&min)),
		uintptr(unsafe.Pointer(&cur)),
		uintptr(unsafe.Pointer(&max)),
	)
	if r == 0 {
		s.mu.Lock()
		s.disabled[device] = true
		s.mu.Unlock()
		return 0, 0, 0, fmt.Errorf("GetMonitorBrightness(%s): %w", device, e)
	}
	return min, cur, max, nil
}

func (s *ddcState) setBrightness(device string, value uint32) error {
	h, err := s.open(device)
	if err != nil {
		return err
	}
	mon := h.monitors[0]
	r, _, e := procSetMonitorBrightness.Call(uintptr(mon.Handle), uintptr(value))
	if r == 0 {
		return fmt.Errorf("SetMonitorBrightness(%s): %w", device, e)
	}
	return nil
}

func (s *ddcState) isDisabled(device string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disabled[device]
}

// hmonitorForDevice resolves a GDI device name to its HMONITOR by walking the
// monitor list with EnumDisplayMonitors.
func hmonitorForDevice(device string) (windows.Handle, error) {
	target := device
	var found windows.Handle

	// The callback must always return 1. Returning 0 means "stop" and makes
	// EnumDisplayMonitors itself report failure, which used to surface as the
	// useless error "EnumDisplayMonitors: The operation completed successfully".
	cb := windows.NewCallback(func(hmon windows.Handle, hdc windows.Handle, rect unsafe.Pointer, data uintptr) uintptr {
		if found != 0 {
			return 1
		}
		var mi monitorInfoEx
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		r, _, _ := procGetMonitorInfoW.Call(uintptr(hmon), uintptr(unsafe.Pointer(&mi)))
		if r != 0 && windows.UTF16ToString(mi.SzDevice[:]) == target {
			found = hmon
		}
		return 1
	})

	if r, _, _ := procEnumDisplayMonitors.Call(0, 0, cb, 0); r == 0 {
		return 0, fmt.Errorf("EnumDisplayMonitors 调用失败")
	}
	if found == 0 {
		return 0, fmt.Errorf("找不到 %s 对应的 HMONITOR", device)
	}
	return found, nil
}

type rect struct {
	Left, Top, Right, Bottom int32
}

type monitorInfoEx struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
	SzDevice  [32]uint16
}
