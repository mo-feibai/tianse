//go:build windows

// Package session reports when the interactive session becomes usable again.
//
// Two events matter to a tool that writes the Windows theme: a sign-in or an
// unlock, and a resume from sleep. In both cases the shell may have spent the
// interval on another desktop (the Winlogon one) or not been running at all, so
// a WM_SETTINGCHANGE broadcast sent then reaches nobody. The desktop comes back
// painted from the shell's own cache, which still holds whatever the theme was
// before we changed it.
//
// Neither event has a polling equivalent that is both cheap and reliable, so
// this package owns a hidden window and a message loop of its own. The window is
// never shown: it exists only to give the notifications somewhere to arrive.
package session

import (
	"log"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	wtsapi32 = syscall.NewLazyDLL("wtsapi32.dll")

	procRegisterClassExW        = user32.NewProc("RegisterClassExW")
	procCreateWindowExW         = user32.NewProc("CreateWindowExW")
	procDestroyWindow           = user32.NewProc("DestroyWindow")
	procDefWindowProcW          = user32.NewProc("DefWindowProcW")
	procGetMessageW             = user32.NewProc("GetMessageW")
	procTranslateMessage        = user32.NewProc("TranslateMessage")
	procDispatchMessageW        = user32.NewProc("DispatchMessageW")
	procPostMessageW            = user32.NewProc("PostMessageW")
	procPostQuitMessage         = user32.NewProc("PostQuitMessage")
	procGetModuleHandleW        = kernel32.NewProc("GetModuleHandleW")
	procWTSRegisterSessionNotif = wtsapi32.NewProc("WTSRegisterSessionNotification")
)

const (
	className = "TianseSessionWatcher"

	wmClose            = 0x0010
	wmDestroy          = 0x0002
	wmWtsSessionChange = 0x02B1
	wmPowerBroadcast   = 0x0218

	// WTS_CONNECTSTATE_CLASS values carried in wParam of WM_WTSSESSION_CHANGE.
	wtsSessionLogon  = 0x0001
	wtsSessionUnlock = 0x0008

	// wParam of WM_POWERBROADCAST. ResumeAutomatic arrives when the machine
	// wakes without user input; ResumeSuspend when it wakes because someone
	// pressed a key. ResumeCritical is the variant with no chance to prepare.
	pbtApmResumeCritical  = 0x0006
	pbtApmResumeSuspend   = 0x0007
	pbtApmResumeAutomatic = 0x0012

	notifyForThisSession = 0

	// csOwndc is not used, but CreateWindowExW needs a class with a wndproc and
	// no style is required for a window that is never shown.
	cwUseDefault = 0x80000000
)

// wndClassEx is WNDCLASSEXW. The field order and the two 32-bit pairs at the
// front are what Windows expects; Go's default layout matches it on amd64.
type wndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

// winMsg is MSG. LPrivate is included even though nothing reads it: GetMessageW
// writes the whole struct, and a short buffer would be overwritten past its end.
type winMsg struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       struct{ X, Y int32 }
	LPrivate uint32
}

var (
	// resumed carries one value per wake-up. It is buffered with capacity one so
	// a notification that arrives while the engine is busy is not lost, and a
	// second one before the engine looks is harmless — both mean the same thing.
	resumed = make(chan struct{}, 1)

	// wndProcCallback is held in a package variable on purpose: syscall keeps
	// the callback alive only as long as the uintptr is reachable.
	wndProcCallback = syscall.NewCallback(wndProc)

	mu     sync.Mutex
	hwnd   uintptr
	closed bool
)

// Notify returns the channel that receives a value whenever the session becomes
// usable again. The channel is shared and never closed.
func Notify() <-chan struct{} { return resumed }

// Start begins watching. It is a no-op if the watcher is already running or has
// been stopped.
func Start() {
	mu.Lock()
	if hwnd != 0 || closed {
		mu.Unlock()
		return
	}
	mu.Unlock()
	go run()
}

// Stop tears the window down and ends the message loop.
func Stop() {
	mu.Lock()
	h := hwnd
	closed = true
	mu.Unlock()
	if h != 0 {
		procPostMessageW.Call(h, wmClose, 0, 0)
	}
}

// run owns the thread the window lives on. Windows delivers a window's messages
// to the thread that created it, so the goroutine must not be rescheduled onto
// another one.
func run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	h, err := createWindow()
	if err != nil {
		// A missing watcher only costs the wake-up re-apply; the engine keeps
		// working from its own clock, so this is worth a line and not a crash.
		log.Printf("session: watcher disabled: %v", err)
		return
	}

	mu.Lock()
	if closed {
		mu.Unlock()
		procDestroyWindow.Call(h)
		return
	}
	hwnd = h
	mu.Unlock()

	defer func() {
		mu.Lock()
		hwnd = 0
		mu.Unlock()
	}()

	if r, _, _ := procWTSRegisterSessionNotif.Call(h, notifyForThisSession); r == 0 {
		log.Printf("session: WTSRegisterSessionNotification refused; only wake-ups will be seen")
	}

	for {
		var m winMsg
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		// GetMessageW returns 0 on WM_QUIT and -1 on error; as an unsigned
		// uintptr both are "not a positive count", so one comparison covers both.
		if int32(r) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func createWindow() (uintptr, error) {
	hInst, _, _ := procGetModuleHandleW.Call(0)
	name, err := syscall.UTF16PtrFromString(className)
	if err != nil {
		return 0, err
	}

	wc := wndClassEx{
		CbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		LpfnWndProc:   wndProcCallback,
		HInstance:     hInst,
		LpszClassName: name,
	}
	if r, _, e := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return 0, e
	}

	// WS_OVERLAPPED and no WS_VISIBLE: a top-level window that is never shown.
	// It has to be top-level rather than message-only, because WM_POWERBROADCAST
	// is broadcast to top-level windows only.
	h, _, e := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(name)),
		0, // WS_OVERLAPPED
		0, 0, 0, 0,
		0, 0, hInst, 0,
	)
	if h == 0 {
		return 0, e
	}
	return h, nil
}

func wndProc(h uintptr, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmWtsSessionChange:
		if wparam == wtsSessionUnlock || wparam == wtsSessionLogon {
			post()
		}
		return 0
	case wmPowerBroadcast:
		switch wparam {
		case pbtApmResumeAutomatic, pbtApmResumeSuspend, pbtApmResumeCritical:
			post()
		}
		// TRUE means "handled"; returning FALSE lets other handlers run, which
		// is what a bystander window should say.
		return 0
	case wmClose:
		procDestroyWindow.Call(h)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(h, msg, wparam, lparam)
	return r
}

// post never blocks: the engine reads this at most once a second, and a queue of
// wake-ups would all mean the same thing anyway.
func post() {
	select {
	case resumed <- struct{}{}:
	default:
	}
}
