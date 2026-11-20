// Package config loads and persists the user's settings.
//
// The file is YAML at %APPDATA%\Tianse\config.yaml. Every field has a default
// that matches the shipped product, so a truncated or partial file still boots.
// A "version" key gates future migrations.
package config

import (
	"os"
	"path/filepath"
	"sync"
)

// CurrentVersion is the schema version written into new files.
//
// 2 added Brightness.Baseline. Files written by version 1 have no such key, and
// migrate gives them "startup" so the screen does not move on the first launch
// after the upgrade.
//
// 3 folded UI.Theme into Theme.Mode. There used to be two independent answers to
// "is it dark yet" — one for the Windows theme, one for the app's own surface —
// and they disagreed by up to an hour, which is exactly the bug the user
// reported: the desktop went dark at 16:50 and the window was still white at
// 17:17. One switch now drives both.
//
// 4 added Brightness.BasePercent for the "fixed" baseline. "startup" re-reads
// the panel each launch, so a screen left dim overnight became tomorrow's
// baseline and the display crept darker day after day. A fixed panel percentage
// breaks that feedback loop.
const CurrentVersion = 4

// The three baselines Brightness.Baseline can name. They are strings in the file
// because a user reads the file, and an int would need a comment to decode.
const (
	// BaselinePanel multiplies against the panel's own full scale.
	BaselinePanel = "panel"
	// BaselineStartup multiplies against the level read at launch.
	BaselineStartup = "startup"
	// BaselineFixed multiplies against a panel percentage the user sets once.
	// Unlike "startup" it does not drift with last night's dim value.
	BaselineFixed = "fixed"
)

// Location is where the user is. Latitude and longitude are pre-filled from the
// built-in city table, so the app never needs a network call or a Windows
// location permission.
type Location struct {
	City      string  `yaml:"city"`
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
	Timezone  string  `yaml:"timezone"`
}

// Temp is the colour-temperature ramp.
type Temp struct {
	Enabled              bool    `yaml:"enabled"`
	Day                  float64 `yaml:"day"`
	Night                float64 `yaml:"night"`
	StartBeforeSunsetMin int     `yaml:"start_before_sunset_min"`
	SettleAfterSunsetMin int     `yaml:"settle_after_sunset_min"`
}

// Brightness is the DDC/CI brightness ramp. The three ratios are multipliers on
// a baseline, and Baseline decides which baseline they multiply:
//
//	"panel"   — the panel's own full scale, so a ratio of 1.0 asks for 100% of
//	            the panel and 0.45 asks for 45% of it.
//	"startup" — the level the panel happened to be at when the app launched, so
//	            1.0 means "leave it where it was" and the ratios can only dim.
//	"fixed"   — BasePercent, a panel percentage the user sets once. The panel is
//	            read at launch only to seed the UI, never to move the baseline.
//
// The field is a switch rather than three sets of numbers because the formula is
// the same either way; only the value of the baseline changes.
type Brightness struct {
	Enabled  bool   `yaml:"enabled"`
	Baseline string `yaml:"baseline"`
	// BasePercent is the fixed baseline in panel percent (0..100). Zero means
	// the user has not picked a value yet; the settings page fills it from the
	// launch reading when "fixed" is chosen.
	BasePercent   float64 `yaml:"base_percent"`
	DayRatio      float64 `yaml:"day_ratio"`
	NightRatio    float64 `yaml:"night_ratio"`
	FloorRatio    float64 `yaml:"floor_ratio"`
	StepThreshold float64 `yaml:"step_threshold"`
}

// Theme is the light/dark switch, and it drives both surfaces: the Windows
// theme (via the registry) and the app's own palette. They were separate once,
// on the theory that a tool built around daylight should paint itself from the
// sky rather than from Windows — but two rules for one question drift apart, and
// the user sees a white window on a black desktop.
type Theme struct {
	Enabled bool    `yaml:"enabled"`
	Mode    string  `yaml:"mode"`
	DarkAt  float64 `yaml:"dark_at_k"`
	LightAt float64 `yaml:"light_at_k"`
}

// Schedule chooses between solar events and a fixed clock table.
type Schedule struct {
	Mode  string     `yaml:"mode"`
	Fixed FixedHours `yaml:"fixed"`
}

// FixedHours is the fallback schedule used at polar latitudes or by choice.
type FixedHours struct {
	NightStart string `yaml:"night_start"`
	NightEnd   string `yaml:"night_end"`
}

// DisplayOverride is the per-monitor tuning. The key is the GDI device name.
type DisplayOverride struct {
	Sync            bool    `yaml:"sync"`
	TempOffset      float64 `yaml:"temp_offset"`
	BrightnessRatio float64 `yaml:"brightness_ratio"`
}

// UI holds the front-end preferences.
type UI struct {
	// Theme is retired: Theme.Mode answers for the app's surface as well now.
	// The field survives only so an older file can be read once and folded in by
	// migrate — writing it back would recreate the second answer this version
	// exists to remove, hence omitempty.
	Theme       string `yaml:"theme,omitempty"`
	AccentDrift bool   `yaml:"accent_drift"`
	LastPage    string `yaml:"last_page"`
}

// App holds process-level behaviour.
type App struct {
	Autostart  bool   `yaml:"autostart"`
	PauseUntil string `yaml:"pause_until"`
	FirstRun   bool   `yaml:"first_run_done"`
}

// Config is the whole file.
type Config struct {
	Version    int                        `yaml:"version"`
	Location   Location                   `yaml:"location"`
	Temp       Temp                       `yaml:"temp"`
	Brightness Brightness                 `yaml:"brightness"`
	Theme      Theme                      `yaml:"theme"`
	Schedule   Schedule                   `yaml:"schedule"`
	Displays   map[string]DisplayOverride `yaml:"displays"`
	UI         UI                         `yaml:"ui"`
	App        App                        `yaml:"app"`
}

// Default returns the shipped configuration.
//
// Brightness is off by default: DDC/CI is not universally implemented and a
// monitor that silently ignores writes would make the app look broken. Colour
// temperature has no such failure mode, so it is on.
func Default() Config {
	return Config{
		Version: CurrentVersion,
		Location: Location{
			City:      "北京",
			Latitude:  39.9042,
			Longitude: 116.4074,
			Timezone:  "Asia/Shanghai",
		},
		Temp: Temp{
			Enabled:              true,
			Day:                  6500,
			Night:                3400,
			StartBeforeSunsetMin: 120,
			SettleAfterSunsetMin: 30,
		},
		Brightness: Brightness{
			Enabled:       false,
			Baseline:      BaselinePanel,
			BasePercent:   0,
			DayRatio:      1.0,
			NightRatio:    0.45,
			FloorRatio:    0.30,
			StepThreshold: 0.03,
		},
		Theme: Theme{
			Enabled: false,
			Mode:    "follow_sky",
			DarkAt:  4500,
			LightAt: 5200,
		},
		Schedule: Schedule{
			Mode: "sun",
			Fixed: FixedHours{
				NightStart: "19:00",
				NightEnd:   "07:00",
			},
		},
		Displays: map[string]DisplayOverride{},
		UI: UI{
			AccentDrift: true,
			LastPage:    "main",
		},
		App: App{
			Autostart: false,
		},
	}
}

// Store is a mutex-guarded config plus its on-disk location.
type Store struct {
	mu   sync.RWMutex
	cfg  Config
	path string
}

// EnvDir lets a caller point the config somewhere else. It exists for two
// reasons: the portable build keeps its config beside the executable, and a
// test run must not touch the profile of whoever is logged in.
const EnvDir = "TIANSE_CONFIG_DIR"

// Dir returns %APPDATA%\Tianse, creating it if needed.
func Dir() (string, error) {
	if override := os.Getenv(EnvDir); override != "" {
		if err := os.MkdirAll(override, 0o755); err != nil {
			return "", err
		}
		return override, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "Tianse")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// LogDir returns the directory that holds the rotating log.
func LogDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		return "", err
	}
	return logs, nil
}
