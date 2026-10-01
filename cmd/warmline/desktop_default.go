//go:build !fyne

package main

// desktop_default.go: `warmline desktop` in the default (cgo-free)
// build. The Fyne desktop GUI needs cgo (GLFW/OpenGL); the default
// binary deliberately omits it so Docker, scratch containers, and CI
// all stay pure-Go. This stub explains the situation instead of
// crashing or silently doing nothing.

import (
	"fmt"
	"io"
)

func launchDesktop(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("desktop", stderr)
	db := fs.String("db", envDefault("WARMLINE_DB", "warmline.db"), "path to the SQLite queue database")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	_ = db
	fmt.Fprintln(stderr, "warmline: this binary was built without the desktop GUI.")
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "The Fyne desktop GUI needs cgo (OpenGL); the default warmline binary is")
	fmt.Fprintln(stderr, "cgo-free so it runs everywhere (Docker, scratch, CI). Two ways to get the GUI:")
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "  1. Build it yourself:   go build -tags fyne -o warmline-desktop ./cmd/warmline")
	fmt.Fprintln(stderr, "     then run:            warmline-desktop desktop")
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "  2. Download the desktop release for your OS from the releases page.")
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "The terminal UI ships in every binary: just run `warmline` (or `warmline tui`).")
	return 1
}
