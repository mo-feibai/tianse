//go:build windows

// Package diagnose prints a one-shot report of everything the tool depends on.
//
// The failure modes of this product are environmental rather than logical: a
// machine can compile and run it and still have no writable gamma ramp, no
// DDC/CI, no interactive desktop, or a Night Light setting that will fight it.
// From outside, all of those look the same — "it does not work". The report
// turns each one into a named yes or no.
package diagnose

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"tianse/internal/config"
	"tianse/internal/nightlight"
	"tianse/internal/screen"
	"tianse/internal/theme"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	ntdll    = syscall.NewLazyDLL("ntdll.dll")

	procGetShellWindow     = user32.NewProc("GetShellWindow")
	procOpenInputDesktop   = user32.NewProc("OpenInputDesktop")
	procCloseDesktop       = user32.NewProc("CloseDesktop")
	procRtlGetVersion      = ntdll.NewProc("RtlGetVersion")
	procCreateSnapshot     = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW    = kernel32.NewProc("Process32FirstW")
	procProcess32NextW     = kernel32.NewProc("Process32NextW")
	procCloseHandle        = kernel32.NewProc("CloseHandle")
	procGetConsoleSession  = kernel32.NewProc("WTSGetActiveConsoleSessionId")
	procProcessIDToSession = kernel32.NewProc("ProcessIdToSessionId")
	procRegOpenKeyExW      = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW   = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey        = advapi32.NewProc("RegCloseKey")
)

const (
	hkeyLocalMachine = 0x80000002
	keyRead          = 0x00020019
	regBinary        = 3
	desktopReadObjs  = 0x0001
	desktopSwitch    = 0x0100
)

// osVersionInfoExW mirrors RTL_OSVERSIONINFOW with the extended fields.
type osVersionInfoExW struct {
	Size        uint32
	Major       uint32
	Minor       uint32
	Build       uint32
	Platform    uint32
	CSDVersion  [128]uint16
	ServicePack uint16
	SuiteMask   uint16
	ProductType byte
	Reserved    byte
}

// Report renders the whole report as text.
func Report() string {
	var b strings.Builder
	line := func(format string, args ...any) {
		b.WriteString(fmt.Sprintf(format, args...))
		b.WriteByte('\n')
	}

	line("天色 诊断报告")
	line("生成时间  %s", time.Now().Format("2006-01-02 15:04:05"))
	line("")

	line("[系统]")
	for _, l := range systemLines() {
		line("  %s", l)
	}
	line("")

	line("[配置]")
	for _, l := range configLines() {
		line("  %s", l)
	}
	line("")

	line("[显示器]")
	displayLines(line)
	line("")

	line("[主题]")
	for _, l := range themeLines() {
		line("  %s", l)
	}
	line("")

	line("[WebView2]")
	line("  %s", webviewVersion())
	line("")

	line("[结论]")
	for _, l := range conclusions() {
		line("  %s", l)
	}

	return b.String()
}

func systemLines() []string {
	var out []string

	var vi osVersionInfoExW
	vi.Size = uint32(unsafe.Sizeof(vi))
	// GetVersionExW lies without a manifest (reports 6.2 for Windows 10);
	// RtlGetVersion returns the real build.
	// RtlGetVersion returns an NTSTATUS, where 0 is success.
	if ret, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&vi))); ret == 0 {
		out = append(out, fmt.Sprintf("Windows %d.%d build %d", vi.Major, vi.Minor, vi.Build))
	} else {
		out = append(out, "Windows 版本 读取失败")
	}

	console := uint32(0xFFFFFFFF)
	if ret, _, _ := procGetConsoleSession.Call(); uint32(ret) != 0xFFFFFFFF {
		console = uint32(ret)
	}
	var session uint32
	procProcessIDToSession.Call(uintptr(os.Getpid()), uintptr(unsafe.Pointer(&session)))
	out = append(out, fmt.Sprintf("当前会话 %d（活动控制台会话 %d）", session, console))

	// GetShellWindow is not enough: it can return a stale handle on a session
	// whose Explorer has died. The process table is the honest answer.
	shell, _, _ := procGetShellWindow.Call()
	switch {
	case hasProcess("explorer.exe"):
		out = append(out, "交互式桌面 是 — 资源管理器在运行")
	case shell != 0:
		out = append(out, "交互式桌面 否 — GetShellWindow 有句柄但资源管理器未运行")
	default:
		out = append(out, "交互式桌面 否 — 资源管理器未运行")
	}

	desk, _, _ := procOpenInputDesktop.Call(0, 0, uintptr(desktopReadObjs|desktopSwitch))
	if desk == 0 {
		out = append(out, "输入桌面 打不开 — 无桌面合成，界面与托盘都会失败")
	} else {
		procCloseDesktop.Call(desk)
		out = append(out, "输入桌面 可打开")
	}

	// GetShellWindow and OpenInputDesktop both succeed on a session whose
	// Explorer and dwm have died, so they are not enough on their own. The
	// window manager is what WebView2 actually needs.
	if hasProcess("dwm.exe") {
		composed = true
		out = append(out, "桌面合成 dwm 在运行")
	} else {
		out = append(out, "桌面合成 dwm 未运行 — WebView2 与托盘都会失败，只能命令行使用")
	}
	return out
}

// composed is set by systemLines and read by conclusions.
var composed bool

// hasProcess reports whether a process with this image name is running.
func hasProcess(name string) bool {
	snap, _, _ := procCreateSnapshot.Call(0x00000002, 0)
	if snap == 0 || snap == uintptr(0xFFFFFFFFFFFFFFFF) {
		return false
	}
	defer procCloseHandle.Call(snap)

	var entry processEntry32W
	entry.Size = uint32(unsafe.Sizeof(entry))
	ok, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		if strings.EqualFold(syscall.UTF16ToString(entry.ExeFile[:]), name) {
			return true
		}
		ok, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	}
	return false
}

// processEntry32W is PROCESSENTRY32W; only the trailing image name matters here.
type processEntry32W struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

func configLines() []string {
	dir, err := config.Dir()
	if err != nil {
		return []string{"目录 无法确定: " + err.Error()}
	}
	out := []string{"目录 " + dir}

	// Writing is what actually matters: the app rewrites config.yaml on every
	// change, so a read-only directory fails at the first toggle.
	probe := filepath.Join(dir, ".diagnose-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		out = append(out, "可写 否 — "+err.Error())
	} else {
		_ = os.Remove(probe)
		out = append(out, "可写 是")
	}

	if path, pathErr := config.DefaultPath(); pathErr == nil {
		if _, statErr := os.Stat(path); statErr == nil {
			out = append(out, "配置文件 "+path+"（已存在）")
		} else {
			out = append(out, "配置文件 "+path+"（尚未创建）")
		}
	}
	return out
}

// displayLines writes the display table and remembers the gamma verdict for
// the conclusion section.
func displayLines(line func(string, ...any)) {
	mgr, err := screen.NewManager()
	if err != nil {
		line("  枚举失败: %v", err)
		gammaVerdict = "无法测试（没有显示器）"
		return
	}
	defer mgr.Close()

	list := mgr.Displays()
	if len(list) == 0 {
		line("  没有检测到显示器")
		gammaVerdict = "无法测试（没有显示器）"
		return
	}

	writable := 0
	for i, d := range list {
		role := "副屏"
		if d.Primary {
			role = "主屏"
		}
		desc := d.Description
		if desc == "" {
			desc = "(无描述)"
		}
		line("  %d. %s  %s  %s  %d×%d", i+1, d.Device, desc, role, d.Width, d.Height)
		if d.BrightnessSupported {
			line("     DDC/CI 可用  最小 %d  当前 %d  最大 %d",
				d.BrightnessMin, d.BrightnessCurrent, d.BrightnessMax)
		} else {
			reason := d.BrightnessErr
			if reason == "" {
				reason = "原因未知"
			}
			line("     DDC/CI 不可用 — 亮度只能走 Gamma 回落（%s）", reason)
		}
		if ok := gammaProbe(mgr, d, line); ok {
			writable++
		}
	}
	gammaVerdict = fmt.Sprintf("%d/%d 块屏可写", writable, len(list))
	gammaWritable = writable
	gammaTotal = len(list)
}

var (
	gammaVerdict  string
	gammaWritable int
	gammaTotal    int
)

// gammaProbe proves writability without changing what the user sees: the ramp
// that is read back is written straight back. A rejection here is the single
// most important line in the report.
func gammaProbe(mgr *screen.Manager, d screen.Display, line func(string, ...any)) bool {
	ramp, err := mgr.ReadRamp(d.Device)
	if err != nil {
		line("     Gamma 读取失败: %v", err)
		return false
	}
	shape := "已调过色"
	if isLinear(ramp) {
		shape = "出厂线性曲线"
	}
	if err := mgr.Apply(d.Device, ramp); err != nil {
		line("     Gamma 读: 成功（%s）  写: 失败 — %v", shape, err)
		return false
	}
	line("     Gamma 读: 成功（%s）  写: 成功", shape)
	return true
}

func isLinear(ramp [3][256]uint16) bool {
	for c := 0; c < 3; c++ {
		for i := 0; i < 256; i++ {
			if int(ramp[c][i]) != i*257 {
				return false
			}
		}
	}
	return true
}

func themeLines() []string {
	var out []string
	if dark, err := theme.IsDark(); err == nil {
		mode := "浅色"
		if dark {
			mode = "深色"
		}
		out = append(out, "应用主题 "+mode)
	} else {
		out = append(out, "应用主题 读取失败: "+err.Error())
	}

	// Night Light would tint the screen on top of our own ramp, and it wins:
	// the two effects do not add, the OS one is applied later. The state lives
	// in HKCU CloudStore, so it is read through the same package the engine
	// uses — a second parser here could disagree with the running program.
	switch on, err := nightlight.Active(); {
	case errors.Is(err, nightlight.ErrNotConfigured):
		out = append(out, "夜间模式 从未配置 — 不会与天色叠加")
	case err != nil:
		out = append(out, "夜间模式 状态读取失败: "+err.Error())
	case on:
		out = append(out, "夜间模式 开启 — 会与天色的色温叠加，建议二选一")
	default:
		out = append(out, "夜间模式 关闭")
	}
	return out
}

func webviewVersion() string {
	if v, ok := regString("SOFTWARE\\WOW6432Node\\Microsoft\\EdgeUpdate\\Clients\\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}", "pv"); ok && v != "" {
		return v
	}
	if v, ok := regString("SOFTWARE\\Microsoft\\EdgeUpdate\\Clients\\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}", "pv"); ok && v != "" {
		return v
	}
	return "未安装 — 界面无法启动"
}

func regString(path, value string) (string, bool) {
	key, ok := openKey(path)
	if !ok {
		return "", false
	}
	defer procRegCloseKey.Call(uintptr(key))

	vp, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return "", false
	}
	var typ, size uint32
	ret, _, _ := procRegQueryValueExW.Call(uintptr(key), uintptr(unsafe.Pointer(vp)), 0,
		uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&size)))
	if ret != 0 || size == 0 {
		return "", false
	}
	buf := make([]uint16, size/2+1)
	ret, _, _ = procRegQueryValueExW.Call(uintptr(key), uintptr(unsafe.Pointer(vp)), 0,
		uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ret != 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

func openKey(path string) (syscall.Handle, bool) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	var key syscall.Handle
	ret, _, _ := procRegOpenKeyExW.Call(uintptr(hkeyLocalMachine), uintptr(unsafe.Pointer(p)),
		0, keyRead, uintptr(unsafe.Pointer(&key)))
	return key, ret == 0
}

// conclusions restates the probes as the one question the user cares about:
// will the temperature actually change on this machine.
func conclusions() []string {
	var out []string
	switch {
	case gammaTotal == 0:
		out = append(out, "色温 无法判定 — "+gammaVerdict)
	case gammaWritable == 0:
		out = append(out, "色温 无法生效 — 所有显示器的 Gamma 写入均被拒绝")
	case gammaWritable < gammaTotal:
		out = append(out, "色温 部分生效 — "+gammaVerdict)
	default:
		out = append(out, "色温 可以生效 — "+gammaVerdict)
	}
	if composed {
		out = append(out, "界面 可以启动")
		out = append(out, "托盘 可以显示")
	} else {
		out = append(out, "界面 无法启动 — 没有桌面合成，WebView2 创建失败")
		out = append(out, "托盘 无法显示 — Shell_NotifyIcon 被拒绝")
	}
	return out
}
