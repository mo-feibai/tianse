package fullscreen

// rect is a rectangle in virtual-screen pixels. It lives in this untagged file
// rather than the Windows one so the rule below can be tested on any platform.
type rect struct {
	Left, Top, Right, Bottom int32
}

// tolerance is how far a window edge may sit from the monitor edge and still
// count as the same edge.
const tolerance = 2

// covers reports whether a window rectangle fills a monitor exactly.
//
// The test is two-sided on purpose. The first version asked only whether the
// window was at least as large as the monitor on every side, which is true for
// every maximised window: Windows draws a maximised window a few pixels
// OUTSIDE the monitor on the left, top and bottom, so its rectangle overshoots
// on all four sides. The result was that the tool stood down — "让出屏幕" —
// whenever any window was maximised, which on a normal desktop is most of the
// time. Requiring each edge to be within tolerance of the monitor edge rejects
// the maximised window (its edges are 7-8 px out) and still accepts borderless
// full screen, whose rectangle is the monitor rectangle.
//
// One case cannot be separated by geometry: with the taskbar set to auto-hide,
// a maximised window's rectangle does equal the monitor rectangle. The caller
// rules that out with the window's WS_MAXIMIZE style.
func covers(wr, mon rect) bool {
	return near(wr.Left, mon.Left) &&
		near(wr.Top, mon.Top) &&
		near(wr.Right, mon.Right) &&
		near(wr.Bottom, mon.Bottom)
}

func near(a, b int32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tolerance
}
