// Package engine is the control loop: it reads the clock, decides what colour
// temperature and brightness the screen should have right now, and pushes that
// decision to every display once a second.
//
// Nothing in here touches the UI. The frontend polls Status and renders it, so
// the engine can keep running with the window closed and the tray hidden.
package engine

import (
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"tianse/internal/clock"
	"tianse/internal/color"
	"tianse/internal/config"
	"tianse/internal/curve"
	"tianse/internal/fullscreen"
	"tianse/internal/nightlight"
	"tianse/internal/screen"
	"tianse/internal/suncalc"
	"tianse/internal/theme"
)

// Status is the snapshot the frontend renders. Times are formatted here rather
// than in TypeScript so the UI never has to parse or localise anything.
type Status struct {
	Kelvin      float64
	Phase       string
	Progress    float64
	Sunset      string
	Sunrise     string
	NextChange  string
	NextChangeK float64
	Paused      bool
	Polar       bool

	BrightnessOn      bool
	BrightnessOK      bool
	BrightnessPercent int
	// BrightnessBase is the level the panel happened to be at when the app
	// launched, in panel percent; -1 when the panel has no controllable
	// backlight. BrightnessRatio is the multiplier the curve is applying to the
	// current baseline, in percent, or 0 while the feature is off.
	//
	// Base is not always the number being multiplied — see BrightnessBaseline —
	// but it is always the launch reading, because that is what the settings
	// page needs to convert between the two baselines without asking the engine
	// to remember anything.
	BrightnessBase  int
	BrightnessRatio int
	// BrightnessBaseline is which baseline the ratios multiply: config
	// BaselinePanel or config BaselineStartup.
	BrightnessBaseline string
	// BrightnessManual is true when the user moved the backlight by hand and the
	// engine has stepped off that display for this run. The reading card says so
	// instead of showing a number the engine is no longer steering.
	BrightnessManual bool

	ThemeOn   bool
	ThemeDark bool
	ThemeMode string

	// Manual is set while the user is holding a temperature picked on the sky
	// axis. The curve keeps running underneath so that leaving manual mode does
	// not jump.
	Manual bool

	SurfaceDark bool
	Accent      string
	Ink         string
	Xiang       string

	City        string
	DisplayName string
	DisplayNum  int
	Warning     string
	GammaOK     bool
	// Held is set while a full-screen app owns the screen and the engine is
	// holding its last frame rather than writing.
	Held bool
}

// Engine owns the loop.
type Engine struct {
	mu   sync.RWMutex
	st   *config.Store
	mgr  *screen.Manager
	stat Status

	paused bool
	stop   chan struct{}
	done   chan struct{}

	baseline   map[string]float64 // DDC fraction at startup, per device
	lastBright map[string]float64 // last fraction written, for the 3% gate
	lastDark   bool
	themeKnown bool

	// themeStrikes counts consecutive ticks where the registry holds something
	// other than what we last wrote. One such tick is noise — Explorer
	// repainting, a broadcast that landed on the wrong desktop, a resume that
	// restored the shell's cached theme — and only a run of them means the user
	// is driving the switch by hand.
	themeStrikes int

	// themeForce asks for one unconditional rewrite of the preference on the
	// next frame allowed to write. Set when the session comes back: the shell
	// spent the interval on the lock screen or asleep, so it repainted from its
	// own cache and our earlier write is no longer what is on screen.
	themeForce bool

	// driftAt is when the backlight was last read to see whether the user moved
	// it by hand, and lastWriteAt is when the engine last wrote a level. The
	// check only runs while both are at least half a minute old: a write inside
	// that window makes any difference ours, not the user's.
	driftAt     time.Time
	lastWriteAt time.Time

	// manual marks displays the user took over by moving the backlight by hand.
	// The engine stops writing to them for the rest of the run: fighting a hand
	// on the monitor's own buttons is a fight the software always loses, and
	// losing it means the screen drifts back to a level the user did not ask
	// for, once a minute, forever. Cleared when the user touches a brightness
	// setting (ConfigChanged).
	manual map[string]bool

	tickCount int

	// brightPercent is the number the reading card shows. While the engine is
	// driving the backlight it is the target; the rest of the time it is what
	// the panel reports, refreshed at most every five seconds because one DDC
	// read costs tens of milliseconds.
	brightPercent int
	brightReadAt  time.Time

	// manualKelvin is non-zero while the user is holding a temperature chosen
	// on the sky axis.
	manualKelvin float64

	// held is true while a full-screen app owns the screen. The engine stops
	// writing instead of fighting the app: an exclusive Direct3D surface resets
	// the ramp on entry and drops every write after that, so pushing frames
	// there only burns CPU and flickers on exit.
	held bool

	// nightLightWarned keeps the conflict notice from being rebuilt every tick.
	nightLightWarned bool

	// gammaFailed records whether the last write attempt was rejected, so the
	// status card can say so instead of showing a temperature that never reached
	// the screen.
	gammaFailed bool

	// gammaWarned remembers which displays are currently refusing the ramp, so a
	// permanent refusal is logged once instead of once per second. The log is
	// the only place the full error text survives, and a message repeated 3600
	// times an hour buries every other line in the file.
	gammaWarned map[string]bool

	// scaleNoted remembers which displays had to be written below the requested
	// strength because their driver caps the colour shift. Also logged once.
	scaleNoted map[string]bool

	// cappedScale is the smallest fraction any display accepted this tick, or 0
	// when every display took the ramp at full strength. The status card turns a
	// value below 1 into "as warm as this panel goes".
	cappedScale float64

	// resumeAt is when the last full-screen hold ended. For two seconds after
	// that the ramp is blended from identity back to the live curve, so leaving
	// a game does not slam the screen from neutral to 3400 K in one frame.
	resumeAt time.Time
}

// New builds an engine over an already-open config and an already-probed screen
// manager.
func New(st *config.Store, mgr *screen.Manager) *Engine {
	e := &Engine{
		st:          st,
		mgr:         mgr,
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
		baseline:    map[string]float64{},
		lastBright:  map[string]float64{},
		manual:      map[string]bool{},
		gammaWarned: map[string]bool{},
		scaleNoted:  map[string]bool{},
	}
	for _, d := range mgr.Displays() {
		if d.BrightnessSupported && d.BrightnessMax > d.BrightnessMin {
			f := float64(d.BrightnessCurrent-d.BrightnessMin) / float64(d.BrightnessMax-d.BrightnessMin)
			e.baseline[d.Device] = f
		}
	}
	// Seed the reading card from the value probed at startup so the first frame
	// already shows a real number instead of a placeholder.
	if d := primaryDisplay(mgr.Displays()); d != nil && d.BrightnessMax > d.BrightnessMin {
		e.brightPercent = int(math.Round(
			float64(d.BrightnessCurrent-d.BrightnessMin) / float64(d.BrightnessMax-d.BrightnessMin) * 100))
	}
	return e
}

// Start runs the loop in a goroutine. The first frame is applied immediately so
// the screen is already correct before the window paints.
func (e *Engine) Start() {
	e.tick(time.Now())
	go func() {
		defer close(e.done)
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-e.stop:
				return
			case now := <-t.C:
				e.tick(now)
			}
		}
	}()
}

// Stop halts the loop and restores the desktop. It is safe to call twice.
func (e *Engine) Stop() {
	select {
	case <-e.stop:
	default:
		close(e.stop)
	}
	select {
	case <-e.done:
	case <-time.After(2 * time.Second):
	}
	e.mgr.RestoreOrigin()
}

// Status returns the last computed snapshot.
func (e *Engine) Status() Status {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stat
}

// Displays returns the current display list for the settings page.
func (e *Engine) Displays() []screen.Display { return e.mgr.Displays() }

// SetPaused freezes or resumes the whole tool.
//
// Pausing restores the original gamma and puts the backlight back to the level
// it had at launch — the one thing pausing can honestly restore, since "off"
// means "as if this app were not running". The Windows light/dark theme is
// deliberately left alone: a
// theme flip repaints every window, and doing that twice in a second (once on
// pause, once on resume) is more intrusive than the setting is worth.
func (e *Engine) SetPaused(paused bool) {
	e.mu.Lock()
	e.paused = paused
	e.mu.Unlock()

	if paused {
		e.mgr.RestoreOrigin()
		for dev, base := range e.baseline {
			if e.manual[dev] {
				continue
			}
			if err := e.mgr.ApplyBrightness(dev, base); err != nil {
				log.Printf("engine: restore brightness %s: %v", dev, err)
			}
			e.lastBright[dev] = base
		}
	}
	e.tick(time.Now())
}

// ResetGamma puts every display back to the ramp captured at startup without
// touching the pause state. The next tick will reapply the current frame, so this
// is a momentary clear, useful when the user wants to compare with and without.
func (e *Engine) ResetGamma() {
	e.mgr.RestoreOrigin()
}

// SetManualTemperature pins the screen to a temperature the user picked on the
// sky axis. Zero clears it.
//
// This is not a setting: it lives in memory only, so restarting the tool returns
// to the curve. A pinned temperature that survived a reboot would be a trap —
// the user would forget why the screen stopped following the day.
func (e *Engine) SetManualTemperature(kelvin float64) {
	e.mu.Lock()
	if kelvin < 1000 {
		kelvin = 0
	}
	if kelvin > 6500 {
		kelvin = 6500
	}
	e.manualKelvin = kelvin
	e.mu.Unlock()
	e.tick(time.Now())
}

// ClearManualTemperature returns control to the curve.
func (e *Engine) ClearManualTemperature() {
	e.SetManualTemperature(0)
}

// Paused reports the current pause state.
func (e *Engine) Paused() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.paused
}

// tick recomputes and applies one frame.
func (e *Engine) tick(now time.Time) {
	cfg := e.st.Snapshot()
	loc := suncalc.Location{Latitude: cfg.Location.Latitude, Longitude: cfg.Location.Longitude}

	st := e.evaluate(now, cfg, loc)

	e.mu.RLock()
	paused := e.paused
	e.mu.RUnlock()

	// A full-screen window gets the screen untouched. Checking every tick is
	// cheap (two window queries); polling faster would not help, because the
	// state only changes when the user switches apps.
	held := !paused && fullscreen.Active()
	if held != e.held {
		if held {
			log.Printf("engine: full-screen app detected, holding the last frame")
			e.mgr.RestoreOrigin()
		} else {
			log.Printf("engine: full-screen app gone, resuming")
			e.resumeAt = now
		}
		e.held = held
	}

	if !paused && !held {
		e.applyTemperature(cfg, st)
		e.applyBrightness(cfg, st, now)
		e.applyTheme(cfg, st)
	}

	e.tickCount++
	if e.tickCount%5 == 0 && !paused && !held {
		if n := e.mgr.SelfHeal(); n > 0 {
			log.Printf("engine: self-heal rewrote %d ramp(s)", n)
		}
	}

	st.Paused = paused
	st.Held = held
	st.GammaOK = !paused && !held && !e.gammaFailed
	st.BrightnessPercent = e.brightnessPercent(cfg, st, now, !paused && !held)
	st.BrightnessBase = e.brightnessBase()
	st.BrightnessRatio = e.brightnessRatioPercent(cfg, st)
	st.BrightnessBaseline = cfg.Brightness.Baseline
	st.BrightnessManual = e.manualPrimary()
	st.Warning = e.warning(cfg, st)
	e.publish(st)
}

// evaluate turns the clock into the state both the screen and the UI read from.
func (e *Engine) evaluate(now time.Time, cfg config.Config, loc suncalc.Location) Status {
	p := curve.Params{
		DayKelvin:    cfg.Temp.Day,
		NightKelvin:  cfg.Temp.Night,
		BeforeSunset: time.Duration(cfg.Temp.StartBeforeSunsetMin) * time.Minute,
		AfterSunset:  time.Duration(cfg.Temp.SettleAfterSunsetMin) * time.Minute,
	}

	var cs curve.State
	if cfg.Schedule.Mode == "fixed" {
		start, end := parseClock(cfg.Schedule.Fixed.NightStart, 19*time.Hour), parseClock(cfg.Schedule.Fixed.NightEnd, 7*time.Hour)
		cs = curve.FixedEvaluate(now, p, start, end)
	} else {
		cs = curve.Evaluate(now, loc, p)
	}

	if !cfg.Temp.Enabled {
		cs.Kelvin = cfg.Temp.Day
		cs.Progress = 0
	}

	// A manual temperature overrides the curve for as long as it is set. The
	// curve value above is still computed, so clearing manual mode resumes
	// wherever the day has got to instead of snapping back to a stale frame.
	e.mu.RLock()
	manual := e.manualKelvin
	e.mu.RUnlock()
	if manual > 0 {
		cs.Kelvin = manual
	}

	st := Status{
		Kelvin:       math.Round(cs.Kelvin),
		Phase:        string(cs.Phase),
		Progress:     cs.Progress,
		Polar:        cs.Polar,
		Sunset:       clockString(cs.Sunset),
		Sunrise:      clockString(cs.Sunrise),
		NextChange:   clockString(cs.NextChange),
		City:         cfg.Location.City,
		ThemeOn:      cfg.Theme.Enabled,
		ThemeMode:    cfg.Theme.Mode,
		BrightnessOn: cfg.Brightness.Enabled,
	}

	st.SurfaceDark = surfaceDark(now, cs, cfg)
	pal := color.DarkPalette(st.Kelvin)
	if !st.SurfaceDark {
		pal = color.LightPalette(st.Kelvin)
	}
	st.Accent, st.Ink, st.Xiang = pal.Accent, pal.Ink, pal.Xiang

	if !cfg.UI.AccentDrift {
		base := color.DarkPalette(color.DayKelvin)
		if !st.SurfaceDark {
			base = color.LightPalette(color.DayKelvin)
		}
		st.Accent = base.Accent
	}

	displays := e.mgr.Displays()
	st.DisplayNum = len(displays)
	if d := primaryDisplay(displays); d != nil {
		// screen already falls back to "显示器 N"; the device path is the last
		// resort for a display that vanished between enumeration and this call.
		st.DisplayName = d.Description
		if st.DisplayName == "" {
			st.DisplayName = d.Device
		}
		st.BrightnessOK = d.BrightnessSupported
	}
	st.Manual = manual > 0
	return st
}

// applyTemperature writes one gamma ramp per display.
//
// The per-display offset exists because two panels of different generations sit
// side by side and the same ramp reads differently on each.
func (e *Engine) applyTemperature(cfg config.Config, st Status) {
	e.cappedScale = 0
	displays := e.mgr.Displays()
	for _, d := range displays {
		kelvin := st.Kelvin
		if o, ok := cfg.Displays[d.Device]; ok {
			kelvin += o.TempOffset
		}
		if kelvin < 1000 {
			kelvin = 1000
		}
		scale := 1.0
		if cfg.Brightness.Enabled && !d.BrightnessSupported {
			scale = brightnessRatio(cfg, st.Progress, d.Device)
		}
		ramp := color.BuildRamp(kelvin, scale)
		if blend := e.resumeBlend(time.Now()); blend < 1 {
			ramp = blendRamp(color.IdentityRamp(), ramp, blend)
		}
		if err := e.mgr.Apply(d.Device, [3][256]uint16(ramp)); err != nil {
			// Once per display per failure streak. The adapter refuses the same
			// ramp forever once it is past its limit, so repeating the line every
			// second only fills the log file.
			if !e.gammaWarned[d.Device] {
				log.Printf("engine: gamma %s: %v", d.Device, err)
				e.gammaWarned[d.Device] = true
			}
			e.gammaFailed = true
			continue
		}
		if e.gammaWarned[d.Device] {
			log.Printf("engine: gamma %s: accepted again", d.Device)
			delete(e.gammaWarned, d.Device)
		}
		e.gammaFailed = false

		// The adapter may have taken less than the requested strength. Record it
		// so the status card can say the screen is as warm as this panel allows
		// instead of pretending the setting was honoured.
		if s := e.mgr.Scale(d.Device); s < 1 {
			if !e.scaleNoted[d.Device] {
				log.Printf("engine: gamma %s: panel caps the colour shift, writing at %.0f%% strength", d.Device, s*100)
				e.scaleNoted[d.Device] = true
			}
			if e.cappedScale == 0 || s < e.cappedScale {
				e.cappedScale = s
			}
		} else {
			delete(e.scaleNoted, d.Device)
			e.cappedScale = 0
		}
	}
}

// resumeBlend returns how far the post-full-screen fade has progressed: 0 right
// after the hold ends, 1 once two seconds have passed.
func (e *Engine) resumeBlend(now time.Time) float64 {
	if e.resumeAt.IsZero() {
		return 1
	}
	const dur = 2 * time.Second
	elapsed := now.Sub(e.resumeAt)
	if elapsed >= dur {
		e.resumeAt = time.Time{}
		return 1
	}
	if elapsed <= 0 {
		return 0
	}
	return float64(elapsed) / float64(dur)
}

// blendRamp interpolates two ramps per channel entry.
func blendRamp(from, to color.Ramp, t float64) color.Ramp {
	if t <= 0 {
		return from
	}
	if t >= 1 {
		return to
	}
	var out color.Ramp
	for c := 0; c < 3; c++ {
		for i := 0; i < 256; i++ {
			a := float64(from[c][i])
			b := float64(to[c][i])
			out[c][i] = uint16(a + (b-a)*t + 0.5)
		}
	}
	return out
}

// applyBrightness drives the backlight over DDC/CI.
//
// Writes are gated at 3% of the panel range because a DDC transaction costs tens
// of milliseconds and the display serialises them; writing every second would
// queue up behind the panel and stutter.
func (e *Engine) applyBrightness(cfg config.Config, st Status, now time.Time) {
	if !cfg.Brightness.Enabled {
		return
	}
	e.detectDrift(now)
	for _, d := range e.mgr.Displays() {
		if !d.BrightnessSupported {
			continue
		}
		if e.manual[d.Device] {
			continue
		}
		want := e.baselineOf(cfg, d.Device) * brightnessRatio(cfg, st.Progress, d.Device)
		if want > 1 {
			want = 1
		}
		if prev, ok := e.lastBright[d.Device]; ok && math.Abs(want-prev) < cfg.Brightness.StepThreshold {
			continue
		}
		if err := e.mgr.ApplyBrightness(d.Device, want); err != nil {
			log.Printf("engine: brightness %s: %v", d.Device, err)
			continue
		}
		e.lastBright[d.Device] = want
		e.lastWriteAt = now
		screen.WaitForDDC()
	}
}

const (
	// driftInterval is how long the backlight has to sit untouched before a
	// reading means anything. Anything shorter and the difference is our own
	// last write still settling in the panel.
	driftInterval = 30 * time.Second
	// driftSlack is how far the panel may differ from what we wrote before the
	// user is credited with the change. 5% of the panel range is one notch on a
	// monitor's on-screen menu, well above DDC read-back noise.
	driftSlack = 0.05
)

// detectDrift notices when the user moves the backlight by hand and hands that
// display over to them.
//
// Without it the curve drags the panel straight back to the level it computed,
// and the reading card keeps reporting a number that is no longer on screen. The
// check only runs while the engine has been idle for a full interval: during a
// curve step the difference is ours, not the user's.
func (e *Engine) detectDrift(now time.Time) {
	if !e.driftAt.IsZero() && now.Sub(e.driftAt) < driftInterval {
		return
	}
	if now.Sub(e.lastWriteAt) < driftInterval {
		return
	}
	e.driftAt = now
	for _, d := range e.mgr.Displays() {
		if !d.BrightnessSupported {
			continue
		}
		last, ok := e.lastBright[d.Device]
		if !ok || last <= 0 {
			continue
		}
		read, ok := e.mgr.ReadBrightness(d.Device)
		if !ok || !brightnessMoved(read, last) {
			continue
		}
		e.manual[d.Device] = true
		e.lastBright[d.Device] = read
		log.Printf("engine: brightness %s moved by hand (%.0f%% to %.0f%%), handing this display back",
			d.Device, last*100, read*100)
	}
}

// brightnessMoved reports whether a panel reading is far enough from what we
// last wrote to be somebody else's doing.
func brightnessMoved(read, last float64) bool {
	return math.Abs(read-last) > driftSlack
}

// baselineOf returns the level the ratios multiply on one display.
//
// "panel" means the panel's own full scale, so a ratio of 1.0 asks for 100% of
// the panel and the screen can get brighter than it was at launch. "startup"
// means the level read at launch, so 1.0 means "leave it where it was" and the
// ratios can only dim. A display with no launch reading falls back to the panel:
// it is the only baseline that cannot make the screen dimmer by accident.
func (e *Engine) baselineOf(cfg config.Config, device string) float64 {
	if cfg.Brightness.Baseline == config.BaselineStartup {
		if b, ok := e.baseline[device]; ok && b > 0 {
			return b
		}
	}
	return 1.0
}

// manualPrimary reports whether the primary display has been handed back to the
// user. The reading card has one slot, so it follows the same display the
// percentage does.
func (e *Engine) manualPrimary() bool {
	d := primaryDisplay(e.mgr.Displays())
	if d == nil {
		return false
	}
	return e.manual[d.Device]
}

// ConfigChanged tells the engine the user just edited the settings.
//
// Moving a brightness slider is an explicit instruction about the backlight, so
// it cancels any hand-over taken earlier: the user is asking the engine to drive
// again. A restart clears it too, because the map lives only in memory.
func (e *Engine) ConfigChanged() {
	e.mu.Lock()
	e.manual = map[string]bool{}
	e.mu.Unlock()
}

// brightnessPercent returns the number the reading card shows on the right.
//
// There are two honest answers and the card has one slot, so the rule is: say
// what the engine is aiming at while it is driving, otherwise say what the
// panel actually is. Reading the panel costs a DDC transaction, so it is done
// at most every five seconds and only when the engine is not writing anyway.
func (e *Engine) brightnessPercent(cfg config.Config, st Status, now time.Time, driving bool) int {
	displays := e.mgr.Displays()
	d := primaryDisplay(displays)
	if d == nil || !d.BrightnessSupported || d.BrightnessMax <= d.BrightnessMin {
		// No controllable backlight: the percentage would be a Gamma-derived
		// guess, which is exactly the kind of number this product refuses to
		// invent. The card shows a dash instead.
		return -1
	}
	if cfg.Brightness.Enabled && driving && !e.manual[d.Device] {
		want := e.baselineOf(cfg, d.Device) * brightnessRatio(cfg, st.Progress, d.Device)
		if want > 1 {
			want = 1
		}
		return int(math.Round(want * 100))
	}
	if now.Sub(e.brightReadAt) >= 5*time.Second {
		e.brightReadAt = now
		if f, ok := e.mgr.ReadBrightness(d.Device); ok {
			e.brightPercent = int(math.Round(f * 100))
		}
	}
	return e.brightPercent
}

// brightnessBase is the panel level the curve multiplies: the reading taken at
// startup, in panel percent.
func (e *Engine) brightnessBase() int {
	d := primaryDisplay(e.mgr.Displays())
	if d == nil || !d.BrightnessSupported || d.BrightnessMax <= d.BrightnessMin {
		return -1
	}
	base, ok := e.baseline[d.Device]
	if !ok || base <= 0 {
		return -1
	}
	return int(math.Round(base * 100))
}

// brightnessRatioPercent is the multiplier the curve is applying right now, in
// percent. Zero while the feature is off: there is no multiplier then.
func (e *Engine) brightnessRatioPercent(cfg config.Config, st Status) int {
	if !cfg.Brightness.Enabled {
		return 0
	}
	d := primaryDisplay(e.mgr.Displays())
	if d == nil || !d.BrightnessSupported {
		return 0
	}
	return int(math.Round(brightnessRatio(cfg, st.Progress, d.Device) * 100))
}

// themeAction is what one frame should do about the Windows light/dark
// preference.
type themeAction int

const (
	// themeHold leaves the registry alone. It is the answer while the evidence
	// is one tick old.
	themeHold themeAction = iota
	// themeWrite sets the preference to the value the sky asks for.
	themeWrite
	// themeYield hands the switch back to the user.
	themeYield
)

// themeStrikesBeforeYield is how many consecutive frames may disagree with the
// registry before the engine concludes that the user is switching by hand.
//
// The number is three because the disagreement has innocent causes that last one
// or two ticks: a resume repaints from the shell's cache, and a broadcast can
// land before the registry write is visible to the next reader. One strike would
// turn any of those into a permanent, silent downgrade to follow_system.
const themeStrikesBeforeYield = 3

// applyTheme flips the Windows light/dark preference with hysteresis.
//
// The gap between DarkAt and LightAt is what stops a temperature hovering at the
// threshold from repainting the whole desktop every few seconds.
func (e *Engine) applyTheme(cfg config.Config, st Status) {
	if !cfg.Theme.Enabled {
		e.themeForce = false
		return
	}
	if cfg.Theme.Mode == "always_light" || cfg.Theme.Mode == "always_dark" {
		want := cfg.Theme.Mode == "always_dark"
		if err := theme.SetDark(want); err != nil {
			log.Printf("engine: theme: %v", err)
		}
		e.lastDark, e.themeKnown = want, true
		st.ThemeDark = want
		return
	}
	if cfg.Theme.Mode == "follow_system" {
		if dark, err := theme.IsDark(); err == nil {
			st.ThemeDark = dark
		}
		return
	}

	// follow_sky: hysteresis around the two thresholds.
	want := e.lastDark
	if !e.themeKnown {
		if dark, err := theme.IsDark(); err == nil {
			want = dark
		}
		e.themeKnown = true
	}
	if st.Kelvin <= cfg.Theme.DarkAt {
		want = true
	} else if st.Kelvin >= cfg.Theme.LightAt {
		want = false
	}

	if current, err := theme.IsDark(); err == nil && current != e.lastDark && e.themeKnown {
		// Someone changed the theme by hand while we were running. Yield to them.
		log.Printf("engine: external theme change detected, switching to follow_system")
		_ = e.st.Update(func(c *config.Config) { c.Theme.Mode = "follow_system" })
		st.ThemeMode = "follow_system"
		st.ThemeDark = current
		e.lastDark = current
		return
	}

	if want != e.lastDark || !e.themeKnown {
		if err := theme.SetDark(want); err != nil {
			log.Printf("engine: theme: %v", err)
			return
		}
		e.lastDark = want
	}
	st.ThemeDark = e.lastDark
}

// warning returns the one sentence worth interrupting the user with, or "".
//
// Only the first applicable warning is shown: the status card has room for one
// line, and a stack of them reads as a broken product rather than a helpful one.
func (e *Engine) warning(cfg config.Config, st Status) string {
	// Windows Night Light and our own ramp both warm the screen, and the OS one
	// is applied last, so the result is the sum of two tints. Nothing in the
	// public API says how strong the OS tint is, so this cannot be corrected —
	// only reported.
	if cfg.Temp.Enabled {
		if on, err := nightlight.Active(); err == nil && on {
			if !e.nightLightWarned {
				log.Printf("engine: Windows Night Light is on; the two tints will stack")
				e.nightLightWarned = true
			}
			return "系统夜间模式已开启，两层色温会叠加，建议二选一"
		}
	}
	e.nightLightWarned = false

	if cfg.Brightness.Enabled && !st.BrightnessOK {
		return "此屏不支持 DDC/CI，亮度只能靠色温衰减"
	}
	if !st.GammaOK && !st.Paused && !st.Held {
		return "色温写入被系统拒绝，详见日志"
	}
	// The user moved the backlight on the monitor itself. Saying so is the only
	// way they learn why the curve stopped moving the screen — and the sentence
	// has to name the way back, or it reads as a failure.
	if st.BrightnessManual && !st.Paused && !st.Held {
		return "亮度已手动接管，改设置即恢复"
	}
	// 192 px of 12 px type fits about sixteen characters on one line, and the
	// card is 176 px tall: a warning longer than that is clipped, so this says
	// the one thing that matters — the screen is as warm as the panel allows.
	if e.cappedScale > 0 && e.cappedScale < 1 && !st.Paused && !st.Held {
		return fmt.Sprintf("面板上限 %.0f%%，色温已收缩", e.cappedScale*100)
	}
	return ""
}

// publish stores the snapshot for the UI to poll.
func (e *Engine) publish(st Status) {
	e.mu.Lock()
	e.stat = st
	e.mu.Unlock()
}

// brightnessRatio returns the multiplier for one display at a given point in the
// daily ramp, already clamped to the safety floor.
func brightnessRatio(cfg config.Config, progress float64, device string) float64 {
	r := cfg.Brightness.DayRatio + (cfg.Brightness.NightRatio-cfg.Brightness.DayRatio)*progress
	if o, ok := cfg.Displays[device]; ok && o.BrightnessRatio > 0 {
		r *= o.BrightnessRatio
	}
	if r < cfg.Brightness.FloorRatio {
		r = cfg.Brightness.FloorRatio
	}
	if r > 1 {
		r = 1
	}
	return r
}

// surfaceDark decides whether the app itself should be painted on the dark or
// the light surface. It follows the sky, not Windows: the tool is meant to match
// the light in the room, and Windows may be following a schedule the user set
// years ago for a different purpose.
func surfaceDark(now time.Time, cs curve.State, cfg config.Config) bool {
	switch cfg.UI.Theme {
	case "dark":
		return true
	case "light":
		return false
	case "system":
		dark, err := theme.IsDark()
		if err == nil {
			return dark
		}
	}
	if cs.Sunset.IsZero() || cs.Sunrise.IsZero() {
		return cs.Progress > 0.5
	}
	if now.After(cs.Sunset.Add(30*time.Minute)) || now.Before(cs.Sunrise.Add(-30*time.Minute)) {
		return true
	}
	return false
}

func primaryDisplay(ds []screen.Display) *screen.Display {
	for i := range ds {
		if ds[i].Primary {
			return &ds[i]
		}
	}
	if len(ds) > 0 {
		return &ds[0]
	}
	return nil
}

// clockString is clock.HM under a local name: the engine builds Status from
// curve.State, whose times are UTC instants from suncalc.
func clockString(t time.Time) string {
	return clock.HM(t)
}

// parseClock accepts "19:00" and falls back to def when the string is unusable.
func parseClock(s string, def time.Duration) time.Duration {
	parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(parts) != 2 {
		return def
	}
	h, err1 := time.ParseDuration(parts[0] + "h")
	m, err2 := time.ParseDuration(parts[1] + "m")
	if err1 != nil || err2 != nil {
		return def
	}
	return h + m
}
