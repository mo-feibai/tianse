// Package clock turns instants into the strings the interface shows.
//
// It exists because of one mistake worth never making twice: the solar
// functions return UTC instants, and calling Format("15:04") on one of those
// prints UTC wall time. In Beijing that put every time on the sky axis eight
// hours behind the taskbar clock — sunset 09:11 instead of 17:11 — while the
// temperature itself was correct, because the curve only compares instants.
// Every human-readable time in the app goes through here.
package clock

import "time"

// HM formats an instant as 24-hour local wall time, e.g. "17:11".
//
// The zero time means "there is no such event" (polar day, polar night, a
// paused engine) and formats as an empty string so the interface can drop the
// line instead of printing 00:00.
func HM(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(time.Local).Format("15:04")
}

// TruncateHour returns the start of the local clock hour containing t.
//
// time.Truncate works on absolute time, so on a half-hour zone such as
// Asia/Kolkata it lands on :30 rather than :00. Rebuilding the time in the local
// zone is the only way to get a wall-clock hour mark.
func TruncateHour(t time.Time) time.Time {
	l := t.In(time.Local)
	return time.Date(l.Year(), l.Month(), l.Day(), l.Hour(), 0, 0, 0, time.Local)
}
