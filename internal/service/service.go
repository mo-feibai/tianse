// Package service is the only surface the frontend can call.
//
// Everything it returns is plain data: no channels, no pointers into the engine,
// nothing the UI could mutate. The engine keeps running whether or not a window
// is open, so the frontend is a viewer with a few commands, not the owner of the
// state.
package service

import (
	"log"

	"tianse/internal/autostart"
	"tianse/internal/config"
	"tianse/internal/engine"
	"tianse/internal/screen"
)

// Service is bound into the webview by Wails.
type Service struct {
	store  *config.Store
	engine *engine.Engine
}

// New builds the bound service.
func New(store *config.Store, eng *engine.Engine) *Service {
	return &Service{store: store, engine: eng}
}

// Status returns the current snapshot. The frontend polls this once a second;
// it is a struct copy behind a read lock, so polling costs nothing.
func (s *Service) Status() engine.Status {
	return s.engine.Status()
}

// Config returns the persisted settings.
func (s *Service) Config() config.Config {
	return s.store.Snapshot()
}

// SaveConfig writes a whole settings object.
//
// The frontend sends the entire struct rather than a diff: the settings page is
// one form, and a partial update would let a stale tab overwrite a field the user
// changed on another page.
func (s *Service) SaveConfig(cfg config.Config) error {
	if err := s.store.Update(func(c *config.Config) { *c = cfg }); err != nil {
		return err
	}
	// Saving is the user speaking about the settings, including the brightness
	// ones, so it also ends any hand-over taken when they moved the backlight on
	// the monitor itself. Without this the only way back would be a restart.
	s.engine.ConfigChanged()
	log.Printf("service: config saved")
	return nil
}

// Cities returns the built-in location table.
func (s *Service) Cities() []config.City {
	return config.Cities
}

// Displays returns the attached monitors.
func (s *Service) Displays() []screen.Display {
	return s.engine.Displays()
}

// SetPaused freezes or resumes the tool and reports the new state.
func (s *Service) SetPaused(paused bool) bool {
	s.engine.SetPaused(paused)
	return paused
}

// TogglePaused flips the pause state and reports the new one.
func (s *Service) TogglePaused() bool {
	s.engine.SetPaused(!s.engine.Paused())
	return s.engine.Paused()
}

// SetManualTemperature pins the temperature the user dragged to on the sky axis.
// Zero hands control back to the curve.
func (s *Service) SetManualTemperature(kelvin float64) {
	s.engine.SetManualTemperature(kelvin)
}

// Autostart reports whether the Run entry exists in the registry.
//
// The registry is read rather than the config file: a cleaner tool or a manual
// edit can remove the entry, and the settings page must then show the truth
// rather than what the user last chose.
func (s *Service) Autostart() bool {
	return autostart.Enabled()
}

// SetAutostart writes the Run entry and records the same value in the config,
// so the two agree on the next start.
func (s *Service) SetAutostart(enabled bool) error {
	if err := autostart.Set(enabled); err != nil {
		return err
	}
	return s.store.Update(func(c *config.Config) { c.App.Autostart = enabled })
}

// ResetGamma puts the desktop back to its original ramp without changing any
// setting. It exists for the "something looks wrong" case: the user can clear
// the tint, then decide whether to keep using the tool.
func (s *Service) ResetGamma() {
	s.engine.ResetGamma()
}
