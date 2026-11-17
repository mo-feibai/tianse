package clock

import (
	"testing"
	"time"
)

// withLocal runs f with a fixed local zone so the test does not depend on the
// machine it runs on.
func withLocal(t *testing.T, zone *time.Location, f func()) {
	t.Helper()
	prev := time.Local
	time.Local = zone
	defer func() { time.Local = prev }()
	f()
}

func TestHMConvertsUTCToLocal(t *testing.T) {
	// 09:11 UTC is 17:11 in Beijing. Printing "09:11" was the bug.
	utc := time.Date(2026, 11, 17, 9, 11, 0, 0, time.UTC)
	withLocal(t, time.FixedZone("CST", 8*3600), func() {
		if got := HM(utc); got != "17:11" {
			t.Fatalf("HM(%s) = %q, want 17:11", utc, got)
		}
	})
}

func TestHMKeepsLocalInputAlone(t *testing.T) {
	zone := time.FixedZone("CST", 8*3600)
	local := time.Date(2026, 11, 17, 21, 37, 0, 0, zone)
	withLocal(t, zone, func() {
		if got := HM(local); got != "21:37" {
			t.Fatalf("HM(%s) = %q, want 21:37", local, got)
		}
	})
}

func TestHMZeroIsEmpty(t *testing.T) {
	if got := HM(time.Time{}); got != "" {
		t.Fatalf("HM(zero) = %q, want empty", got)
	}
}

func TestTruncateHourUsesWallClock(t *testing.T) {
	// Asia/Kolkata is UTC+5:30, the case absolute truncation gets wrong.
	zone := time.FixedZone("IST", 5*3600+30*60)
	withLocal(t, zone, func() {
		in := time.Date(2026, 11, 17, 21, 37, 0, 0, zone)
		got := TruncateHour(in)
		if got.Hour() != 21 || got.Minute() != 0 {
			t.Fatalf("TruncateHour(%s) = %s, want 21:00 local", in, got)
		}
		if HM(got) != "21:00" {
			t.Fatalf("HM(TruncateHour) = %q, want 21:00", HM(got))
		}
	})
}
