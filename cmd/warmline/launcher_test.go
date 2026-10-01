package main

import (
	"strings"
	"testing"
)

// Bare `warmline` with no args in a NON-TTY must print the hint and
// exit nonzero — never hang CI/pipes.
func TestBareInvocationNonTTY(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{}, &out, &errB)
	if code == 0 {
		t.Error("bare invocation in non-TTY should not exit 0")
	}
	combined := out.String() + errB.String()
	if !strings.Contains(combined, "interactive terminal") {
		t.Errorf("non-TTY hint missing: %q", combined)
	}
}

// `warmline tui` in a non-TTY must also refuse, not hang.
func TestTUICommandNonTTY(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"tui"}, &out, &errB)
	if code == 0 {
		t.Error("tui in non-TTY should not exit 0")
	}
	if !strings.Contains(errB.String(), "interactive terminal") {
		t.Errorf("tui refusal missing: %q", errB.String())
	}
}

// `warmline desktop` in the default (cgo-free) build explains itself.
func TestDesktopCommandDefaultBuild(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"desktop"}, &out, &errB)
	if code == 0 {
		t.Error("desktop in default build should not exit 0")
	}
	combined := out.String() + errB.String()
	if !strings.Contains(combined, "-tags fyne") {
		t.Errorf("desktop stub must teach the fyne build: %q", combined)
	}
}

// Subcommands still work with zero args passed through.
func TestVersionStillWorks(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"version"}, &out, &errB)
	if code != 0 {
		t.Error("version should still work")
	}
	if !strings.Contains(out.String(), "0.1.0") {
		t.Errorf("version output: %q", out.String())
	}
}
