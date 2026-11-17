//go:build windows

package screen

import (
	"fmt"
	"sync"
	"time"
)

// Manager owns the per-display device contexts and the DDC handles, and
// remembers each display's original gamma ramp so it can be restored exactly.
type Manager struct {
	mu      sync.Mutex
	hdc     *hdcCache
	ddc     *ddcState
	origin  map[string][3][256]uint16
	last    map[string][3][256]uint16
	display []Display
}

// NewManager builds a manager and snapshots every display's current ramp.
func NewManager() (*Manager, error) {
	m := &Manager{
		hdc:    newHDCCache(),
		ddc:    newDDCState(),
		origin: map[string][3][256]uint16{},
		last:   map[string][3][256]uint16{},
	}
	if err := m.Refresh(); err != nil {
		return nil, err
	}
	return m, nil
}

// Refresh re-enumerates the displays and snapshots the ramps of any display it
// has not seen before. It is cheap enough to call on every resolution change.
func (m *Manager) Refresh() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	displays, err := enumerateDisplays()
	if err != nil {
		return err
	}
	m.display = displays

	for i := range m.display {
		d := &m.display[i]
		h, err := m.hdc.get(d.Device)
		if err != nil {
			continue
		}
		if _, ok := m.origin[d.Device]; !ok {
			if ramp, err := readGamma(h); err == nil {
				m.origin[d.Device] = ramp
				m.last[d.Device] = ramp
			}
		}
		if min, cur, max, err := m.ddc.getBrightness(d.Device); err == nil && max > 0 {
			d.BrightnessSupported = true
			d.BrightnessMin, d.BrightnessCurrent, d.BrightnessMax = min, cur, max
			d.BrightnessErr = ""
		} else if err != nil {
			d.BrightnessErr = err.Error()
		} else {
			d.BrightnessErr = fmt.Sprintf("max=%d，显示器没有报告亮度范围", max)
		}
	}
	return nil
}

// Displays returns a copy of the last enumeration.
func (m *Manager) Displays() []Display {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Display, len(m.display))
	copy(out, m.display)
	return out
}

// Primary returns the device name of the primary display, or the first display
// if Windows reports none.
func (m *Manager) Primary() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.display {
		if d.Primary {
			return d.Device
		}
	}
	if len(m.display) > 0 {
		return m.display[0].Device
	}
	return ""
}

// Apply writes a ramp to one display. It returns the previous ramp so callers
// can tell whether anything changed.
func (m *Manager) Apply(device string, ramp [3][256]uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	h, err := m.hdc.get(device)
	if err != nil {
		return err
	}
	if prev, ok := m.last[device]; ok {
		diff := rampMaxDiff(prev, ramp)
		if diff < 655 {
			// Less than 1% of full scale: invisible, skip the call.
			return nil
		}
	}
	if err := writeGamma(h, &ramp); err != nil {
		return err
	}
	m.last[device] = ramp
	return nil
}

// ApplyBrightness sets a DDC/CI brightness. value is a fraction of the panel
// maximum in [0,1]. Panels that do not answer are marked disabled so the engine
// stops retrying them.
func (m *Manager) ApplyBrightness(device string, fraction float64) error {
	m.mu.Lock()
	d := m.displayByDevice(device)
	m.mu.Unlock()
	if d == nil || !d.BrightnessSupported {
		return fmt.Errorf("brightness not supported on %s", device)
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	span := float64(d.BrightnessMax-d.BrightnessMin) / 100.0
	if span <= 0 {
		span = 1
	}
	value := d.BrightnessMin + uint32(fraction*100*span+0.5)
	if value > d.BrightnessMax {
		value = d.BrightnessMax
	}
	return m.ddc.setBrightness(device, value)
}

// ReadBrightness returns the current raw DDC brightness as a fraction.
func (m *Manager) ReadBrightness(device string) (float64, bool) {
	min, cur, max, err := m.ddc.getBrightness(device)
	if err != nil || max == min {
		return 0, false
	}
	return float64(cur-min) / float64(max-min), true
}

// ProbeBrightness reports the raw DDC/CI triple, keeping the error so the
// diagnostic report can say why a panel does not answer.
func (m *Manager) ProbeBrightness(device string) (min, cur, max uint32, err error) {
	return m.ddc.getBrightness(device)
}

// ReadRamp returns the ramp the display is currently using. The diagnostic
// report needs it to tell "gamma is unsupported" apart from "gamma is fine but
// the tool never wrote it".
func (m *Manager) ReadRamp(device string) ([3][256]uint16, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, err := m.hdc.get(device)
	if err != nil {
		return [3][256]uint16{}, err
	}
	return readGamma(h)
}

// RestoreOrigin puts every display back to the ramp captured at startup. It is
// called on a clean exit so the desktop is not left tinted.
func (m *Manager) RestoreOrigin() {
	m.mu.Lock()
	devices := make([]string, 0, len(m.origin))
	ramps := map[string][3][256]uint16{}
	for d, r := range m.origin {
		devices = append(devices, d)
		ramps[d] = r
	}
	m.mu.Unlock()

	for _, d := range devices {
		_ = m.Apply(d, ramps[d])
	}
}

// SelfHeal rewrites any display whose ramp no longer matches what we last set.
// Sleep/wake, resolution changes and exclusive-fullscreen games all reset the
// hardware ramp behind our back; a cheap read-and-compare every few seconds
// catches all three without hooking window messages.
func (m *Manager) SelfHeal() int {
	m.mu.Lock()
	type pair struct {
		device string
		ramp   [3][256]uint16
	}
	var todo []pair
	for d, want := range m.last {
		h, err := m.hdc.get(d)
		if err != nil {
			continue
		}
		got, err := readGamma(h)
		if err != nil {
			continue
		}
		if rampMaxDiff(got, want) > 3000 {
			todo = append(todo, pair{device: d, ramp: want})
		}
	}
	m.mu.Unlock()

	for _, t := range todo {
		_ = m.Apply(t.device, t.ramp)
	}
	return len(todo)
}

// Close releases the device contexts and DDC handles.
func (m *Manager) Close() {
	m.ddc.reset()
	m.hdc.close()
}

func (m *Manager) displayByDevice(device string) *Display {
	for i := range m.display {
		if m.display[i].Device == device {
			return &m.display[i]
		}
	}
	return nil
}

func rampMaxDiff(a, b [3][256]uint16) int {
	max := 0
	for c := 0; c < 3; c++ {
		for i := 0; i < 256; i++ {
			d := int(a[c][i]) - int(b[c][i])
			if d < 0 {
				d = -d
			}
			if d > max {
				max = d
			}
		}
	}
	return max
}

// WaitForDDC gives the monitor a moment to settle after a brightness write.
// DDC/CI transactions are serialised by the display and a burst of writes will
// silently drop the later ones.
func WaitForDDC() {
	time.Sleep(60 * time.Millisecond)
}
