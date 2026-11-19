package engine

import "testing"

func TestThemeStepHoldsOnTheFirstDisagreement(t *testing.T) {
	// The desktop says light while we last wrote dark. One tick of that is not
	// a decision: Explorer may be mid-repaint, or a broadcast may have landed
	// before our registry write was visible.
	action, strikes := themeStep(true, false, true, 0, false)
	if action != themeHold {
		t.Fatalf("first disagreement must hold, got %v", action)
	}
	if strikes != 1 {
		t.Fatalf("strike count must be 1, got %d", strikes)
	}
}

func TestThemeStepYieldsAfterThreeStrikes(t *testing.T) {
	action, strikes := themeStep(true, false, true, themeStrikesBeforeYield-1, false)
	if action != themeYield {
		t.Fatalf("the third consecutive disagreement must yield, got %v", action)
	}
	if strikes != 0 {
		t.Fatalf("yielding must reset the count, got %d", strikes)
	}
}

func TestThemeStepForgetsStrikesWhenTheRegistryAgrees(t *testing.T) {
	// Two strikes, then the desktop matches again: the run is over.
	action, strikes := themeStep(true, true, true, 2, false)
	if action != themeHold {
		t.Fatalf("agreement must hold, got %v", action)
	}
	if strikes != 0 {
		t.Fatalf("agreement must clear the strikes, got %d", strikes)
	}
}

func TestThemeStepWritesWhenTheSkyAsksForTheOtherTheme(t *testing.T) {
	// Registry matches what we wrote, but the sun has since moved past the
	// threshold: this is the ordinary switch, not a conflict.
	action, strikes := themeStep(false, false, true, 0, false)
	if action != themeWrite {
		t.Fatalf("a threshold crossing must write, got %v", action)
	}
	if strikes != 0 {
		t.Fatalf("strike count must stay 0, got %d", strikes)
	}
}

func TestThemeStepForcedRewriteIgnoresDisagreement(t *testing.T) {
	// A resume repaints from the shell's cache, so the registry disagreeing
	// with us is the expected state rather than evidence about the user.
	action, strikes := themeStep(true, false, true, 2, true)
	if action != themeWrite {
		t.Fatalf("a forced rewrite must write, got %v", action)
	}
	if strikes != 0 {
		t.Fatalf("a forced rewrite must clear the strikes, got %d", strikes)
	}
}

func TestThemeStepForcedRewriteWritesEvenWhenAlreadyCorrect(t *testing.T) {
	// The value is right but the screen may have been painted from a stale
	// cache while we were away; writing again is what re-broadcasts it.
	action, _ := themeStep(true, true, true, 0, true)
	if action != themeWrite {
		t.Fatalf("a forced rewrite must write even when the value matches, got %v", action)
	}
}
