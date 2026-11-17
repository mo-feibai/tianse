// Package autostart keeps the app in the per-user Run key.
//
// The user's choice is stored twice: in the config file, which the settings page
// reads, and in the registry, which is what actually starts the program. They
// can disagree — a cleaner tool or a manual registry edit changes one without
// the other — so the config value is treated as a record of intent and the
// registry as the truth.
package autostart

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueID = "Tianse"
)

// Enabled reports whether the Run entry exists.
func Enabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(valueID)
	return err == nil
}

// Set adds or removes the Run entry.
//
// The command is the running executable, quoted, because the install path may
// contain spaces. The working directory is left to the registry default: the
// app resolves its own paths from %APPDATA%, never from the cwd.
func Set(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}
	defer k.Close()

	if !enabled {
		if err := k.DeleteValue(valueID); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("delete Run value: %w", err)
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	return k.SetStringValue(valueID, `"`+exe+`"`)
}
