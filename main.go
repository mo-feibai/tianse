package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"tianse/internal/config"
	"tianse/internal/diagnose"
	"tianse/internal/engine"
	"tianse/internal/logging"
	"tianse/internal/screen"
	"tianse/internal/service"
	"tianse/internal/trayicon"
	"tianse/internal/version"
)

//go:embed all:frontend/dist
var assets embed.FS

const (
	windowWidth  = 900
	windowHeight = 620
)

func main() {
	// The report has to work on a machine where nothing else works, so it runs
	// before logging, config and the display manager are touched.
	diag := flag.Bool("diagnose", false, "打印环境诊断报告后退出")
	showVersion := flag.Bool("version", false, "打印版本号后退出")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.String())
		return
	}
	if *diag {
		fmt.Print(diagnose.Report())
		return
	}

	if dir, err := config.LogDir(); err == nil {
		if err := logging.Setup(dir); err != nil {
			log.Printf("main: logging disabled: %v", err)
		}
	}
	defer logging.Close()

	store, err := config.Load("")
	if err != nil {
		log.Fatalf("main: load config: %v", err)
	}

	mgr, err := screen.NewManager()
	if err != nil {
		// A machine with no usable display is not something the tool can work
		// around; report it and exit rather than run a control loop that does
		// nothing.
		log.Fatalf("main: no displays: %v", err)
	}
	defer mgr.Close()

	eng := engine.New(store, mgr)
	svc := service.New(store, eng)

	// Declared before New so the second-instance callback can reach the window
	// that does not exist yet at that point.
	var mainWindow application.Window

	app := application.New(application.Options{
		Name:        "天色",
		Description: "随天光而变的护眼工具",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Windows: application.WindowsOptions{
			// The tool lives in the tray. Closing the window must not end the
			// process, and the taskbar button is noise for a window this small.
			DisableQuitOnLastWindowClosed: true,
			// Keep the browser profile beside our own config instead of in
			// %LOCALAPPDATA%: the portable build then carries everything in one
			// folder, and a run with a redirected config dir cannot collide with
			// an installed copy.
			WebviewUserDataPath: webviewDataPath(),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.tianse.app",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if mainWindow != nil {
					mainWindow.Show()
					mainWindow.Focus()
				}
			},
		},
		OnShutdown: func() {
			eng.Stop()
		},
	})

	mainWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "main",
		Title:         "天色",
		Width:         windowWidth,
		Height:        windowHeight,
		DisableResize: true,
		// The title bar is drawn in the page: the native one cannot be tinted to
		// match the jade surface, and it would be the only grey strip in a window
		// that is otherwise entirely colour.
		Frameless:        true,
		BackgroundType:   application.BackgroundTypeSolid,
		BackgroundColour: application.NewRGB(0x14, 0x17, 0x1C),
		URL:              "/",
		InitialPosition:  application.WindowCentered,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: false,
		},
	})

	// Closing the window hides it. The engine keeps running: the point of the
	// product is that it works while the user is doing something else.
	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		mainWindow.Hide()
		e.Cancel()
	})

	tray := app.SystemTray.New()
	tray.SetTooltip("天色")
	tray.SetIcon(trayicon.Render(trayicon.Normal, false))
	tray.SetDarkModeIcon(trayicon.Render(trayicon.Normal, true))
	tray.AttachWindow(mainWindow).WindowOffset(4)

	menu := app.NewMenu()
	pauseItem := menu.Add("暂停")
	// A read-only line rather than a submenu: the point is to answer "is it
	// doing anything right now" without opening the window.
	statusItem := menu.Add("正在读取天色…")
	statusItem.SetEnabled(false)
	menu.Add("打开设置").OnClick(func(ctx *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(ctx *application.Context) {
		eng.Stop()
		app.Quit()
	})

	pauseItem.OnClick(func(ctx *application.Context) {
		paused := svc.TogglePaused()
		refreshTray(tray, menu, pauseItem, paused)
	})

	tray.SetMenu(menu)
	tray.OnClick(func() {
		mainWindow.Show()
		mainWindow.Focus()
	})

	eng.Start()
	go watchTray(tray, pauseItem, statusItem, eng)

	if err := app.Run(); err != nil {
		log.Printf("main: %v", err)
		os.Exit(1)
	}
}

// webviewDataPath resolves the WebView2 profile directory, falling back to the
// platform default when our own directory is unavailable.
func webviewDataPath() string {
	dir, err := config.Dir()
	if err != nil {
		return ""
	}
	path := filepath.Join(dir, "WebView2")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return ""
	}
	return path
}

// watchTray mirrors the engine state into the tray every few seconds.
//
// Ten seconds is chosen against the product, not the CPU: the temperature moves
// by a handful of kelvin a minute, so a faster refresh would produce a number
// that visibly changes for no reason.
func watchTray(tray *application.SystemTray, pauseItem, statusItem *application.MenuItem, eng *engine.Engine) {
	lastState := ""
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		st := eng.Status()
		if state := trayState(st); state != lastState {
			lastState = state
			applyTrayState(tray, pauseItem, statusItem, st)
		}
		<-tick.C
	}
}

// trayState is a compact fingerprint of everything the tray displays, so the
// watcher can skip identical redraws.
func trayState(st engine.Status) string {
	// Brightness belongs in the fingerprint because the status line prints it:
	// leaving it out would freeze the label at whatever percentage it had when
	// the menu was last rebuilt.
	return fmt.Sprintf("%v|%d|%v|%v|%s|%d", st.Paused, int(st.Kelvin), st.Held, st.GammaOK, st.Warning, st.BrightnessPercent)
}

func applyTrayState(tray *application.SystemTray, pauseItem, statusItem *application.MenuItem, st engine.Status) {
	label := "暂停"
	if st.Paused {
		label = "继续"
	}
	pauseItem.SetLabel(label)

	state := trayicon.Normal
	switch {
	case st.Paused:
		state = trayicon.Paused
	case st.Warning != "" || !st.GammaOK:
		state = trayicon.Failed
	}
	tray.SetIcon(trayicon.Render(state, false))
	tray.SetDarkModeIcon(trayicon.Render(state, true))

	statusItem.SetLabel(statusLine(st))
}

// statusLine is the same sentence the status card shows, shortened for a menu.
func statusLine(st engine.Status) string {
	switch {
	case st.Paused:
		return "已暂停"
	case st.Held:
		return "全屏中 · 已让出屏幕"
	case st.Warning != "":
		return st.Warning
	}
	// The brightness figure is a real reading whenever the panel answers DDC,
	// whether or not the engine is driving it (DESIGN 7).
	if st.BrightnessOK && st.BrightnessPercent >= 0 {
		return fmt.Sprintf("%.0f K · 亮度 %d%%", st.Kelvin, st.BrightnessPercent)
	}
	return fmt.Sprintf("%.0f K · %s", st.Kelvin, phaseWord(st.Phase))
}

func phaseWord(phase string) string {
	switch phase {
	case "day":
		return "白天"
	case "evening":
		return "入夜"
	case "night":
		return "夜间"
	case "morning":
		return "破晓"
	case "polar":
		return "极昼极夜"
	}
	return phase
}

// refreshTray keeps the menu label and the icon in step with the pause state.
func refreshTray(tray *application.SystemTray, menu *application.Menu, item *application.MenuItem, paused bool) {
	if paused {
		item.SetLabel("继续")
		tray.SetIcon(trayicon.Render(trayicon.Paused, false))
		tray.SetDarkModeIcon(trayicon.Render(trayicon.Paused, true))
		return
	}
	item.SetLabel("暂停")
	tray.SetIcon(trayicon.Render(trayicon.Normal, false))
	tray.SetDarkModeIcon(trayicon.Render(trayicon.Normal, true))
}
