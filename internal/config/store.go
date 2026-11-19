package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load reads the config file, applying defaults for anything missing. A missing
// file is not an error: it yields the defaults and writes them back so the user
// has something to edit.
func Load(path string) (*Store, error) {
	if path == "" {
		var err error
		if path, err = DefaultPath(); err != nil {
			return nil, err
		}
	}
	cfg := Default()
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		// Decode into the defaulted struct so absent keys keep their defaults.
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	case os.IsNotExist(err):
		// Fresh install.
	default:
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	migrate(&cfg)
	normalise(&cfg)

	s := &Store{cfg: cfg, path: path}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		if err := s.Save(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// migrate upgrades older schema versions in place.
func migrate(cfg *Config) {
	// An empty Baseline means no choice was ever recorded: either a version-1
	// file, which predates the key, or a hand-edited one. Version 1 multiplied
	// every ratio by the launch level, so pinning those files to "startup" keeps
	// each number meaning exactly what it meant before the upgrade and the screen
	// does not move on the first launch. A file that already names a baseline is
	// a decision and is left alone.
	if cfg.Brightness.Baseline == "" {
		cfg.Brightness.Baseline = BaselineStartup
	}
	// Version 2 and earlier had a second answer to "is it dark yet" in
	// UI.Theme. Fold it into Theme.Mode so the window and the Windows theme keep
	// the behaviour the user last chose, then clear the field: from here on there
	// is exactly one answer, and leaving the old key behind would let a later
	// write recreate the disagreement.
	//
	// The condition looks at the field, not the version number. A file can be
	// version 3 and still carry the key — hand-edited, or written by a build that
	// predates the fold — and a version check would walk past it.
	switch cfg.UI.Theme {
	case "light":
		cfg.Theme.Mode = "always_light"
	case "dark":
		cfg.Theme.Mode = "always_dark"
	case "system":
		cfg.Theme.Mode = "follow_system"
	case "auto":
		// "auto" meant follow the sky, which is what follow_sky already says.
		cfg.Theme.Mode = "follow_sky"
	}
	cfg.UI.Theme = ""
	if cfg.Version < CurrentVersion {
		cfg.Version = CurrentVersion
	}
}

// normalise repairs values that would make the engine misbehave: temperatures
// outside the physical range, ratios outside [0,1], an unknown enum value.
func normalise(cfg *Config) {
	if cfg.Temp.Day < 1900 || cfg.Temp.Day > 6500 {
		cfg.Temp.Day = 6500
	}
	if cfg.Temp.Night < 1900 || cfg.Temp.Night > cfg.Temp.Day {
		cfg.Temp.Night = 3400
	}
	if cfg.Temp.StartBeforeSunsetMin < 0 || cfg.Temp.StartBeforeSunsetMin > 480 {
		cfg.Temp.StartBeforeSunsetMin = 120
	}
	if cfg.Temp.SettleAfterSunsetMin < 0 || cfg.Temp.SettleAfterSunsetMin > 240 {
		cfg.Temp.SettleAfterSunsetMin = 30
	}

	switch cfg.Brightness.Baseline {
	case BaselinePanel, BaselineStartup:
	default:
		cfg.Brightness.Baseline = BaselinePanel
	}

	cfg.Brightness.DayRatio = clamp01(cfg.Brightness.DayRatio, 1.0)
	cfg.Brightness.NightRatio = clamp01(cfg.Brightness.NightRatio, 0.45)
	cfg.Brightness.FloorRatio = clamp01(cfg.Brightness.FloorRatio, 0.30)
	if cfg.Brightness.StepThreshold <= 0 || cfg.Brightness.StepThreshold > 0.2 {
		cfg.Brightness.StepThreshold = 0.03
	}
	if cfg.Brightness.NightRatio > cfg.Brightness.DayRatio {
		cfg.Brightness.NightRatio = cfg.Brightness.DayRatio
	}

	switch cfg.Theme.Mode {
	case "follow_sky", "follow_system", "always_light", "always_dark":
	default:
		cfg.Theme.Mode = "follow_sky"
	}
	if cfg.Theme.DarkAt > cfg.Theme.LightAt {
		cfg.Theme.DarkAt, cfg.Theme.LightAt = 4500, 5200
	}

	if cfg.Schedule.Mode != "sun" && cfg.Schedule.Mode != "fixed" {
		cfg.Schedule.Mode = "sun"
	}
	if cfg.Displays == nil {
		cfg.Displays = map[string]DisplayOverride{}
	}
}

func clamp01(v, fallback float64) float64 {
	if v <= 0 || v > 1 {
		if v == 0 {
			return fallback
		}
		return fallback
	}
	return v
}

// Snapshot returns a copy safe to hand to the UI.
func (s *Store) Snapshot() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.cfg
	out.Displays = make(map[string]DisplayOverride, len(s.cfg.Displays))
	for k, v := range s.cfg.Displays {
		out.Displays[k] = v
	}
	return out
}

// Update applies a mutation and writes the file. Writes are atomic: a crash
// mid-save leaves the previous file intact.
func (s *Store) Update(fn func(*Config)) error {
	s.mu.Lock()
	fn(&s.cfg)
	normalise(&s.cfg)
	cfg := s.cfg
	path := s.path
	s.mu.Unlock()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Save writes the current config.
func (s *Store) Save() error {
	return s.Update(func(*Config) {})
}

// Path returns the file location.
func (s *Store) Path() string { return s.path }

// DefaultPath resolves %APPDATA%\Tianse\config.yaml.
func DefaultPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}
