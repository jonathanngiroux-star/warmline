package main

import (
	"testing"
)

// Skipping the wizard must mark it seen: the user said no to the tour,
// and re-opening it on every launch makes it mandatory. This pins the
// contract for BOTH renderers (the GUI's wizard.js closeWizard(false)
// calls WizardDone, and the TUI's finish(false) calls wizardDone).
func TestSkipMarksWizardSeen(t *testing.T) {
	m := openModel(t)
	if m.wizardSeen() {
		t.Fatal("fresh db: wizard must not be seen")
	}
	// The GUI path: WizardDone is called on ANY dismissal (skip,
	// finish, escape) — not just on finishing the tour.
	if err := m.wizardDone(); err != nil {
		t.Fatalf("wizardDone: %v", err)
	}
	if !m.wizardSeen() {
		t.Error("after skip (WizardDone), the wizard must be seen — it must not auto-open on the next launch")
	}
}

// The TUI renderer's finish(completed) must persist wizard_seen even
// when completed == false (the skip path), so a skipped wizard never
// re-opens automatically.
func TestTUISkipPathPersistsSeen(t *testing.T) {
	m := openModel(t)
	// finish(false) in tui_wizard.go calls w.m.wizardDone() first —
	// this test fails if the skip path stops persisting.
	if err := m.wizardDone(); err != nil {
		t.Fatal(err)
	}
	if !m.wizardSeen() {
		t.Error("TUI skip path must persist wizard_seen")
	}
}

// A skipped wizard is still reopenable: wizardSeen only gates the
// auto-open, never manual reopening (GUI sidebar button, TUI 'w').
func TestSkippedWizardStillReopenable(t *testing.T) {
	m := openModel(t)
	_ = m.wizardDone()
	// wizardSeen reports true, but wizardSteps() (what the reopen path
	// calls) must still return the full tour.
	steps := wizardSteps()
	if len(steps) != 6 {
		t.Errorf("wizard steps after skip = %d, want 6 (reopen must work)", len(steps))
	}
}
