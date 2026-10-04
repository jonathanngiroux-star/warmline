package main

// clipboard.go: copy-to-clipboard for the donate addresses. The GUI
// path uses Wails' runtime.ClipboardSetText (bound method); this file
// is the TUI/CLI strategy, tried in order:
//
//  1. OSC-52 escape — works on most modern terminals (kitty, foot,
//     gnome-terminal, tmux passthrough) with zero dependencies; we
//     write it to /dev/tty so it also works when stdout is piped.
//  2. wl-copy (Wayland), xclip / xsel (X11) — for terminals that
//     ignore OSC-52.
//
// The wizard donates step and the donate page both surface a Copy
// button; the button shows the copy result inline.

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// copyAddress copies addr to the system clipboard for the TUI.
func copyAddress(addr string) error {
	// 1. OSC-52 to the controlling terminal (no external tools).
	if tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0); err == nil {
		_, writeErr := tty.WriteString(osc52For(addr))
		tty.Close()
		if writeErr == nil {
			return nil
		}
	}
	// 2. Clipboard tools (Wayland first, then X11).
	return copyToClipboardTools(addr)
}

// osc52For builds the terminal clipboard escape sequence:
// ESC ] 52 ; c ; <base64> BEL
func osc52For(text string) string {
	return "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\x07"
}

// copyToClipboardTools pipes text into the first installed clipboard
// tool. Order: wl-copy (Wayland), xclip, xsel (both X11).
func copyToClipboardTools(text string) error {
	type tool struct {
		name string
		args []string
	}
	tools := []tool{
		{"wl-copy", nil},
		{"xclip", []string{"-selection", "clipboard"}},
		{"xsel", []string{"--clipboard", "--input"}},
	}
	for _, tl := range tools {
		path, err := exec.LookPath(tl.name)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, tl.args...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", tl.name, err)
		}
		return nil
	}
	return fmt.Errorf("no clipboard tool available — install wl-copy (Wayland) or xclip (X11), or copy the address manually")
}

// clipboardHint returns the human instruction when copy is unavailable.
func clipboardHint() string {
	if runtime.GOOS == "windows" {
		return "select the address and press Ctrl+C"
	}
	return "install wl-copy (Wayland) or xclip (X11) to enable one-key copy"
}
