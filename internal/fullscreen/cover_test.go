package fullscreen

import "testing"

func TestCoversAcceptsBorderlessFullScreen(t *testing.T) {
	mon := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1080}
	wr := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1080}
	if !covers(wr, mon) {
		t.Fatal("an exact match must count as full screen")
	}
	// A one-pixel rounding difference is still the same window.
	if !covers(rect{Left: 1, Top: 0, Right: 1919, Bottom: 1080}, mon) {
		t.Fatal("a one-pixel inset must still count as full screen")
	}
}

func TestCoversRejectsMaximisedWindow(t *testing.T) {
	// What Windows actually reports for a maximised window: the resize border
	// sits outside the monitor, and the bottom edge overshoots too.
	mon := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1080}
	wr := rect{Left: -8, Top: -8, Right: 1928, Bottom: 1088}
	if covers(wr, mon) {
		t.Fatal("a maximised window must not count as full screen")
	}
}

func TestCoversRejectsWindowOnAnotherMonitor(t *testing.T) {
	mon := rect{Left: 1920, Top: 0, Right: 3840, Bottom: 1080}
	wr := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1080}
	if covers(wr, mon) {
		t.Fatal("a window filling a different monitor must not count")
	}
}

func TestCoversRejectsWindowThatLeavesTheTaskbarVisible(t *testing.T) {
	mon := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1080}
	wr := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1040}
	if covers(wr, mon) {
		t.Fatal("a window 40 px short of the bottom must not count")
	}
}
