package main

// launcher.go: what bare `warmline`, `warmline tui`, and `warmline
// desktop` do. Bare invocation in a terminal opens the TUI (the
// operator's front door); `desktop` opens the Wails GUI in desktop
// builds and explains itself in default (cgo-free) builds. Non-TTY
// contexts never hang — they print and exit.

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jonathanngiroux-star/warmline/internal/store"
)

// isTerminal reports whether f is an interactive terminal.
var isTerminal = func(f *os.File) bool {
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// newFlagSet builds the shared flag set used by launcher subcommands.
func newFlagSet(name string, output io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)
	return fs
}

// launchTUI runs the terminal UI when interactive; otherwise prints the
// usage hint (tests, pipes, CI — never a hang).
func launchTUI(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("tui", stderr)
	db := fs.String("db", envDefault("WARMLINE_DB", "warmline.db"), "path to the SQLite queue database")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		fmt.Fprintln(stderr, "warmline: the TUI needs an interactive terminal.")
		fmt.Fprintln(stderr, "headless? try: warmline serve, warmline migrate, warmline simulate, warmline dkim")
		fmt.Fprintln(stderr, "")
		usage(stderr)
		return 2
	}
	st, err := store.Open(*db)
	if err != nil {
		fmt.Fprintf(stderr, "tui: %v\n", err)
		return 1
	}
	defer st.Close()
	return runTUI(newTUIModel(st))
}
