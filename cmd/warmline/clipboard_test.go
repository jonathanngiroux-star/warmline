package main

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// osc52For must produce the exact escape sequence terminals expect:
// ESC ] 52 ; c ; <base64 of text> BEL
func TestOSC52Sequence(t *testing.T) {
	seq := osc52For("0x85ee7E71f762d772599cbF1EC20E651B30657521")
	want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte("0x85ee7E71f762d772599cbF1EC20E651B30657521")) + "\x07"
	if seq != want {
		t.Errorf("osc52 = %q, want %q", seq, want)
	}
}

// copyToClipboardTools must pipe the text to the first available tool
// (wl-copy, xclip, xsel) and report success; a missing tool set must
// return a helpful error, not a panic.
func TestCopyToClipboardToolsFallback(t *testing.T) {
	t.Run("no tools installed -> actionable error", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		err := copyToClipboardTools("addr")
		if err == nil {
			t.Fatal("expected error when no clipboard tool exists")
		}
		if !strings.Contains(err.Error(), "wl-copy") {
			t.Errorf("error should tell the user what to install: %v", err)
		}
	})

	t.Run("wl-copy wins and receives the text on stdin", func(t *testing.T) {
		dir := t.TempDir()
		// fake wl-copy: dumps stdin to a file so the test can verify
		script := filepath.Join(dir, "wl-copy")
		out := filepath.Join(dir, "got.txt")
		if err := os.WriteFile(script, []byte("#!/bin/sh\n/bin/cat > "+out+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)
		if err := copyToClipboardTools("0xABC"); err != nil {
			t.Fatalf("wl-copy path failed: %v", err)
		}
		got, _ := os.ReadFile(out)
		if string(got) != "0xABC" {
			t.Errorf("wl-copy received %q, want 0xABC", got)
		}
	})

	t.Run("xclip is called with clipboard selection flags", func(t *testing.T) {
		dir := t.TempDir()
		script := filepath.Join(dir, "xclip")
		args := filepath.Join(dir, "args.txt")
		if err := os.WriteFile(script, []byte("#!/bin/sh\n/bin/printf '%s\\n' \"$@\" > "+args+"\n/bin/cat > /dev/null\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)
		if err := copyToClipboardTools("0xABC"); err != nil {
			t.Fatalf("xclip path failed: %v", err)
		}
		got, _ := os.ReadFile(args)
		if !strings.Contains(string(got), "-selection") || !strings.Contains(string(got), "clipboard") {
			t.Errorf("xclip args = %q, want -selection clipboard", got)
		}
	})
}

// copyAddress runs the full TUI copy strategy; with no tty and no tools
// it must still return a clear error naming an install hint.
func TestCopyAddressNoBackend(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := copyAddress(donateEthereum)
	if err == nil {
		t.Fatal("expected an error without any clipboard backend")
	}
	if !strings.Contains(err.Error(), "wl-copy") && !strings.Contains(err.Error(), "clipboard") {
		t.Errorf("error should hint at a fix: %v", err)
	}
	_ = exec.Command // keep exec imported for the tool path
}
