package config

import "testing"

func TestMigratePinsVersionOneFilesToTheStartupBaseline(t *testing.T) {
	// A version-1 file predates the key and its ratios were multiplied by the
	// launch level. Pinning it to "startup" keeps every number meaning what it
	// meant before the upgrade, so the screen does not move on first launch.
	cfg := Default()
	cfg.Version = 1
	cfg.Brightness.Baseline = ""
	migrate(&cfg)
	if cfg.Brightness.Baseline != BaselineStartup {
		t.Errorf("Baseline = %q, want %q", cfg.Brightness.Baseline, BaselineStartup)
	}
	if cfg.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", cfg.Version, CurrentVersion)
	}
}

func TestMigrateLeavesAChosenBaselineAlone(t *testing.T) {
	// A version-2 file that says "panel" is a choice, not a missing key.
	cfg := Default()
	cfg.Version = 2
	cfg.Brightness.Baseline = BaselinePanel
	migrate(&cfg)
	if cfg.Brightness.Baseline != BaselinePanel {
		t.Errorf("Baseline = %q, want the file's %q", cfg.Brightness.Baseline, BaselinePanel)
	}
}

func TestMigrateFillsAVersionTwoFileWithNoKey(t *testing.T) {
	// Hand-written or truncated files can reach version 2 without the key. They
	// were written by someone who had no baseline to choose, so the safer of the
	// two applies: the screen must not get brighter behind their back.
	cfg := Default()
	cfg.Version = 2
	cfg.Brightness.Baseline = ""
	migrate(&cfg)
	if cfg.Brightness.Baseline != BaselineStartup {
		t.Errorf("Baseline = %q, want %q", cfg.Brightness.Baseline, BaselineStartup)
	}
}

func TestNormaliseKeepsAFixedBaselineWithAPercent(t *testing.T) {
	cfg := Default()
	cfg.Brightness.Baseline = BaselineFixed
	cfg.Brightness.BasePercent = 40
	normalise(&cfg)
	if cfg.Brightness.Baseline != BaselineFixed {
		t.Errorf("Baseline = %q, want %q", cfg.Brightness.Baseline, BaselineFixed)
	}
	if cfg.Brightness.BasePercent != 40 {
		t.Errorf("BasePercent = %v, want 40", cfg.Brightness.BasePercent)
	}
}

func TestNormaliseFallsBackWhenFixedHasNoPercent(t *testing.T) {
	// Asking for "fixed" without a value would multiply by zero; normalise has
	// to downgrade to the panel so the screen never blacks out.
	cfg := Default()
	cfg.Brightness.Baseline = BaselineFixed
	cfg.Brightness.BasePercent = 0
	normalise(&cfg)
	if cfg.Brightness.Baseline != BaselinePanel {
		t.Errorf("Baseline = %q, want %q without a fixed percent", cfg.Brightness.Baseline, BaselinePanel)
	}
}

func TestNormaliseRejectsAnUnknownBaseline(t *testing.T) {
	cfg := Default()
	cfg.Brightness.Baseline = "whatever"
	normalise(&cfg)
	if cfg.Brightness.Baseline != BaselinePanel {
		t.Errorf("Baseline = %q, want %q", cfg.Brightness.Baseline, BaselinePanel)
	}
}

func TestMigrateFoldsTheRetiredSurfaceChoiceIntoTheThemeMode(t *testing.T) {
	// Version 2 kept a separate answer for the app's own surface. Whatever the
	// user picked there has to survive as the single switch, or the upgrade would
	// silently repaint their window the other colour.
	cases := map[string]string{
		"auto":   "follow_sky",
		"light":  "always_light",
		"dark":   "always_dark",
		"system": "follow_system",
	}
	for old, want := range cases {
		cfg := Default()
		cfg.Version = 2
		cfg.UI.Theme = old
		migrate(&cfg)
		if cfg.Theme.Mode != want {
			t.Errorf("UI.Theme %q folded to Mode %q, want %q", old, cfg.Theme.Mode, want)
		}
		if cfg.UI.Theme != "" {
			t.Errorf("UI.Theme = %q after migrate, want it cleared", cfg.UI.Theme)
		}
	}
}

func TestMigrateFoldsTheRetiredKeyEvenInAVersionThreeFile(t *testing.T) {
	// The condition looks at the field, not the version number: a hand-edited or
	// partially written file can claim version 3 and still carry the old key.
	cfg := Default()
	cfg.Version = CurrentVersion
	cfg.UI.Theme = "dark"
	migrate(&cfg)
	if cfg.Theme.Mode != "always_dark" {
		t.Errorf("Mode = %q, want always_dark", cfg.Theme.Mode)
	}
}

func TestMigrateLeavesTheThemeModeAloneWhenTheOldKeyIsAbsent(t *testing.T) {
	// A version-3 file has no UI.Theme and its Mode is the user's own choice.
	cfg := Default()
	cfg.Version = CurrentVersion
	cfg.Theme.Mode = "always_light"
	migrate(&cfg)
	if cfg.Theme.Mode != "always_light" {
		t.Errorf("Mode = %q, want the file's always_light", cfg.Theme.Mode)
	}
}
