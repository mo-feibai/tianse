//go:build windows

// Package fullscreen answers one question: does a full-screen window own the
// screen right now?
//
// The answer decides whether the tool may touch the display at all. A game in
// exclusive full screen resets the gamma ramp on entry and fights every write
// after that, so the engine holds its last frame until the user comes back.
package fullscreen

import (
	"syscall"
	"unsafe"
)

var (
	user32  = syscall.NewLazyDLL("user32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")

	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetShellWindow      = user32.NewProc("GetShellWindow")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procIsIconic            = user32.NewProc("IsIconic")
	procGetClassNameW       = user32.NewProc("GetClassNameW")
	procGetWindowLongW      = user32.NewProc("GetWindowLongW")
	procQueryNotifState     = shell32.NewProc("SHQueryUserNotificationState")
)

const (
	monitorDefaultToNearest  = 0x00000002
	qunsBusy                 = 2
	qunsRunningD3DFullScreen = 3
	// GWL_STYLE is -16. It is written as its 32-bit pattern because Go rejects
	// a negative constant converted to uintptr; GetWindowLongW takes a signed
	// int index, so the low 32 bits are what the API reads.
	gwlStyle   = 0xFFFFFFF0
	wsMaximize = 0x01000000
)

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

// Active reports whether a full-screen window currently owns the screen.
//
// Two independent signals are checked. The window rectangle catches ordinary
// full-screen apps, which are visible to the window manager. The shell's
// notification state catches Direct3D exclusive apps, whose window the manager
// may not describe correctly but which always announce themselves this way.
func Active() bool {
	if d3dFullScreen() {
		return true
	}

	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}
	if shell, _, _ := procGetShellWindow.Call(); hwnd == shell {
		return false
	}
	if r, _, _ := procIsWindowVisible.Call(hwnd); r == 0 {
		return false
	}
	if r, _, _ := procIsIconic.Call(hwnd); r != 0 {
		return false
	}
	switch className(hwnd) {
	case "Progman", "WorkerW", "Shell_TrayWnd", "Shell_SecondaryTrayWnd":
		// The desktop and the taskbar are always exactly monitor-sized.
		return false
	}

	var wr rect
	if r, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr))); r == 0 {
		return false
	}
	hmon, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if hmon == 0 {
		return false
	}
	var mi monitorInfo
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if r, _, _ := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi))); r == 0 {
		return false
	}
	// See covers(): the rectangle alone cannot tell a borderless full-screen
	// window from a maximised one whose frame overshoots the monitor. With the
	// taskbar auto-hidden the two rectangles are identical, so the window style
	// is the only remaining signal.
	if maximised(hwnd) {
		return false
	}
	return covers(wr, mi.RcMonitor)
}

// maximised reports whether the window carries WS_MAXIMIZE.
func maximised(hwnd uintptr) bool {
	r, _, _ := procGetWindowLongW.Call(hwnd, uintptr(gwlStyle))
	return r&wsMaximize != 0
}

// d3dFullScreen asks the shell whether it is in a presentation or exclusive
// full-screen state.
func d3dFullScreen() bool {
	var state int32
	hr, _, _ := procQueryNotifState.Call(uintptr(unsafe.Pointer(&state)))
	if hr != 0 {
		return false
	}
	return state == qunsBusy || state == qunsRunningD3DFullScreen
}

func className(hwnd uintptr) string {
	var buf [64]uint16
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}
