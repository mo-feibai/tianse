//go:build windows

// Package theme flips the Windows light/dark preference.
//
// Windows has no API for this. The setting lives in the registry under
// HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize, and the
// shell only notices a change when a WM_SETTINGCHANGE broadcast carries the
// "ImmersiveColorSet" payload. Writing the value alone leaves Explorer and every
// running app on the old theme until the next sign-in.
package theme

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	personalizeKey = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	appsValue      = "AppsUseLightTheme"
	systemValue    = "SystemUsesLightTheme"

	hkeyCurrentUser = 0x80000001
	keyQueryValue   = 0x0001
	keySetValue     = 0x0002
	regDWORD        = 4

	wmSettingChange = 0x001A
	hwndBroadcast   = 0xFFFF
	smtoAbortIfHung = 0x0002
	broadcastMs     = 1000
)

var (
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")

	procRegOpenKeyExW       = advapi32.NewProc("RegOpenKeyExW")
	procRegSetValueExW      = advapi32.NewProc("RegSetValueExW")
	procRegQueryValueExW    = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey         = advapi32.NewProc("RegCloseKey")
	procSendMessageTimeoutW = user32.NewProc("SendMessageTimeoutW")
)

// IsDark reports whether Windows is currently using the dark app theme.
func IsDark() (bool, error) {
	v, err := readDWORD(appsValue)
	if err != nil {
		return false, err
	}
	return v == 0, nil
}

// SetDark switches both the app and the system theme, then tells the shell.
//
// The two values are kept in step on purpose: a light taskbar under dark app
// windows looks like a bug, and this product is meant to be invisible except for
// its effect on the screen.
func SetDark(dark bool) error {
	v := uint32(1)
	if dark {
		v = 0
	}
	if err := writeDWORD(appsValue, v); err != nil {
		return err
	}
	if err := writeDWORD(systemValue, v); err != nil {
		return err
	}
	broadcast("ImmersiveColorSet")
	broadcast("WindowsThemeElement")
	return nil
}

// broadcast posts WM_SETTINGCHANGE to every top-level window. SendMessageTimeout
// with SMTO_ABORTIFHUNG is used instead of PostMessage because a hung window
// must not block the engine loop.
func broadcast(setting string) {
	p, err := syscall.UTF16PtrFromString(setting)
	if err != nil {
		return
	}
	var result uintptr
	procSendMessageTimeoutW.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(p)),
		uintptr(smtoAbortIfHung),
		uintptr(broadcastMs),
		uintptr(unsafe.Pointer(&result)),
	)
}

func openKey(access uint32) (syscall.Handle, error) {
	p, err := syscall.UTF16PtrFromString(personalizeKey)
	if err != nil {
		return 0, err
	}
	var h syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(hkeyCurrentUser),
		uintptr(unsafe.Pointer(p)),
		0,
		uintptr(access),
		uintptr(unsafe.Pointer(&h)),
	)
	if r != 0 {
		return 0, fmt.Errorf("RegOpenKeyEx failed: %d", r)
	}
	return h, nil
}

func readDWORD(name string) (uint32, error) {
	h, err := openKey(keyQueryValue)
	if err != nil {
		return 0, err
	}
	defer procRegCloseKey.Call(uintptr(h))

	np, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	var value, typ uint32
	size := uint32(regDWORD)
	r, _, _ := procRegQueryValueExW.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(np)),
		0,
		uintptr(unsafe.Pointer(&typ)),
		uintptr(unsafe.Pointer(&value)),
		uintptr(unsafe.Pointer(&size)),
	)
	if r != 0 {
		return 0, fmt.Errorf("RegQueryValueEx(%s) failed: %d", name, r)
	}
	return value, nil
}

func writeDWORD(name string, value uint32) error {
	h, err := openKey(keySetValue)
	if err != nil {
		return err
	}
	defer procRegCloseKey.Call(uintptr(h))

	np, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	r, _, _ := procRegSetValueExW.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(np)),
		0,
		uintptr(regDWORD),
		uintptr(unsafe.Pointer(&value)),
		uintptr(regDWORD),
	)
	if r != 0 {
		return fmt.Errorf("RegSetValueEx(%s) failed: %d", name, r)
	}
	return nil
}
