package main

import (
	"strings"
	"testing"
)

// The wizard is a shared logic layer over tuiModel: a fixed sequence of
// guided steps, each carrying the real fields for a real operation.
// The renderers (tui.go wizard page, the Wails GUI modal) only display
// it.
func TestWizardStepsComplete(t *testing.T) {
	steps := wizardSteps()
	want := []string{"welcome", "migrate", "simulate", "dkim", "serve", "donate"}
	if len(steps) != len(want) {
		t.Fatalf("wizard has %d steps, want %d: %+v", len(steps), len(want), steps)
	}
	for i, s := range steps {
		if s.ID != want[i] {
			t.Errorf("step %d = %q, want %q", i, s.ID, want[i])
		}
		if len(s.Title) < 3 {
			t.Errorf("step %q has no title", s.ID)
		}
		if len(s.Body) < 20 {
			t.Errorf("step %q has no body copy", s.ID)
		}
		// Steps with fields must have a run button; prose steps must not.
		if len(s.Fields) > 0 && s.NextLabel == "" {
			t.Errorf("step %q has fields but no run button label", s.ID)
		}
		if len(s.Fields) == 0 && s.NextLabel != "" {
			t.Errorf("step %q has a run button but no fields", s.ID)
		}
	}
}

// Every step's body must stay honest: no deliverability guarantees, and
// the donate step carries both addresses byte-for-byte.
func TestWizardCopyRules(t *testing.T) {
	for _, s := range wizardSteps() {
		low := strings.ToLower(s.Body)
		if strings.Contains(low, "guarantee") && !strings.Contains(low, "not a deliverability guarantee") {
			t.Errorf("step %q overpromises: %q", s.ID, s.Body)
		}
		if s.ID == "donate" {
			if !strings.Contains(s.Body, donateEthereum) || !strings.Contains(s.Body, donateBitcoin) {
				t.Errorf("donate step missing addresses: %q", s.Body)
			}
		}
	}
	// Non-donate steps must not carry the addresses (footer does that).
	for _, s := range wizardSteps() {
		if s.ID != "donate" {
			if strings.Contains(s.Body, donateEthereum) {
				t.Errorf("step %q duplicates the footer addresses", s.ID)
			}
		}
	}
}

// First-run detection: unseen flag → wizard shown once; after Done the
// flag persists and the wizard never auto-starts again.
func TestWizardFirstRunPersistence(t *testing.T) {
	m := openModel(t)
	if m.wizardSeen() {
		t.Error("fresh db must not have wizard_seen set")
	}
	if err := m.wizardDone(); err != nil {
		t.Fatalf("wizardDone: %v", err)
	}
	if !m.wizardSeen() {
		t.Error("wizard_seen must persist after Done")
	}
	// And a second model over the same db sees it too (persistence, not
	// in-memory state).
	m2 := newTUIModel(m.db)
	if !m2.wizardSeen() {
		t.Error("wizard_seen must be durable across model instances")
	}
}

// The serve step runs a real probe but must still teach the real
// command for after the wizard.
func TestWizardServeStepTeachesServe(t *testing.T) {
	steps := wizardSteps()
	var serve *wizardStep
	for i := range steps {
		if steps[i].ID == "serve" {
			serve = &steps[i]
		}
	}
	if serve == nil {
		t.Fatal("no serve step")
	}
	if !strings.Contains(serve.Body, "warmline serve") {
		t.Errorf("serve step must teach the serve command: %q", serve.Body)
	}
}
