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
const CurrentVersion = 1

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

// Brightness is the DDC/CI brightness ramp, expressed relative to the level the
// user had when the app started.
type Brightness struct {
	Enabled       bool    `yaml:"enabled"`
	DayRatio      float64 `yaml:"day_ratio"`
	NightRatio    float64 `yaml:"night_ratio"`
	FloorRatio    float64 `yaml:"floor_ratio"`
	StepThreshold float64 `yaml:"step_threshold"`
}

// Theme is the Windows light/dark switch.
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
	Theme       string `yaml:"theme"`
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
			Theme:       "auto",
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
