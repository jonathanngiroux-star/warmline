// Command warmline is the single binary: MTA wrapper, migration CLI,
// reputation simulator, and webhook normalizer. One process, one SQLite
// file, no external dependencies.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jonathanngiroux-star/warmline/internal/migrate"
	"github.com/jonathanngiroux-star/warmline/internal/migrate/sendgrid"
)

// version is the single source of truth for the binary version. Overridden
// at build time via -ldflags "-X main.version=...".
var version = "0.1.0"

// Donation addresses — binding, do not change without 4/4 consensus +
// 30-day notice per GOVERNANCE.md.
const (
	donateEthereum = "0x85ee7E71f762d772599cbF1EC20E651B30657521"
	donateBitcoin  = "bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		// Bare `warmline` in a terminal opens the TUI — the operator's
		// front door. In non-TTY contexts launchTUI prints a hint.
		return launchTUI(args, stdout, stderr)
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "tui":
		return launchTUI(rest, stdout, stderr)
	case "desktop":
		return launchDesktop(rest, stdout, stderr)
	case "version":
		return cmdVersion(stdout)
	case "donate":
		return cmdDonate(rest, stdout, stderr)
	case "migrate":
		return cmdMigrate(rest, stdout, stderr)
	case "simulate":
		return cmdSimulate(rest, stdout, stderr)
	case "serve":
		return cmdServe(rest, stdout, stderr)
	case "dkim":
		return cmdDKIM(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", cmd)
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: warmline <command> [flags]

commands:
  (none)    open the TUI (interactive terminal)
  tui       open the TUI (interactive terminal)
  desktop   open the desktop GUI (desktop build: -tags fyne)
  version   print version
  donate    print donation addresses
  migrate   dry-run a migration from an ESP (sendgrid, postmark)
  simulate  reputation trajectory simulation
  serve     run the local MTA wrapper + queue + UI
  dkim      DKIM key generation + rotation plans

Warmline is free to self-host; donations only — see docs/donate.md.
`)
}

func cmdVersion(w io.Writer) int {
	fmt.Fprintf(w, "warmline %s\n", version)
	return 0
}

func cmdDonate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("donate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	switch *format {
	case "text":
		fmt.Fprint(stdout, `Warmline is free to self-host. If it saves you a SendGrid week, donate.

Ethereum / USDC (ERC-20): `+donateEthereum+`
Bitcoin:                  `+donateBitcoin+`

No feature is gated on donations. Details: docs/donate.md
`)
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]string{
			"ethereum":      donateEthereum,
			"ethereum_usdc": "ERC-20 on Ethereum mainnet; USDC is a contract — send to the address above, not the USDC contract address",
			"bitcoin":       donateBitcoin,
			"note":          "Warmline is free to self-host. Donations are optional.",
		}); err != nil {
			fmt.Fprintf(stderr, "encode: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "unknown --format %q (want text or json)\n", *format)
		return 2
	}
	return 0
}

func cmdMigrate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	from := fs.String("from", "", "source ESP: sendgrid (postmark planned)")
	dryRun := fs.Bool("dry-run", false, "report only; never touch the source ESP (required)")
	format := fs.String("format", "text", "output format: text or json")
	input := fs.String("input", "", "path to the ESP export file (JSON)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *from == "" {
		fmt.Fprintln(stderr, "migrate: --from is required (sendgrid)")
		return 2
	}
	if !*dryRun {
		fmt.Fprintln(stderr, "migrate: refusing to run without --dry-run; Warmline never mutates a real ESP account")
		fmt.Fprintln(stderr, "migrate: re-run with --dry-run")
		return 2
	}
	switch *from {
	case "sendgrid":
		return migrateSendgrid(*format, *input, stdout, stderr)
	case "postmark":
		return migratePostmark(*format, *input, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "migrate: unsupported source %q (supported: sendgrid, postmark)\n", *from)
		return 2
	}
}

func migrateSendgrid(format, input string, stdout, stderr io.Writer) int {
	if input == "" {
		fmt.Fprintln(stderr, "migrate: --input is required (path to your SendGrid export JSON)")
		return 2
	}
	exp, err := sendgrid.ParseFile(input)
	if err != nil {
		fmt.Fprintf(stderr, "migrate: %v\n", err)
		return 1
	}
	report, err := exp.BuildReport()
	if err != nil {
		fmt.Fprintf(stderr, "migrate: build report: %v\n", err)
		return 1
	}
	report.Input = input
	switch format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "migrate: encode: %v\n", err)
			return 1
		}
	case "text":
		printReportText(report, stdout)
	default:
		fmt.Fprintf(stderr, "migrate: unknown --format %q (want text or json)\n", format)
		return 2
	}
	return 0
}

func printReportText(r migrate.Report, w io.Writer) {
	fmt.Fprintf(w, "Warmline migrate dry-run (from %s)\n\n", r.Source)
	fmt.Fprintf(w, "added (%d):\n", len(r.Added))
	for _, e := range r.Added {
		fmt.Fprintf(w, "  + %s%s\n", e.Key, noteSuffix(e))
	}
	fmt.Fprintf(w, "removed (%d):\n", len(r.Removed))
	for _, e := range r.Removed {
		fmt.Fprintf(w, "  - %s%s\n", e.Key, noteSuffix(e))
	}
	fmt.Fprintf(w, "changed (%d):\n", len(r.Changed))
	for _, e := range r.Changed {
		fmt.Fprintf(w, "  ~ %s: %v -> %v\n", e.Key, e.From, e.To)
	}
	fmt.Fprintf(w, "unmapped (%d):\n", len(r.Unmapped))
	for _, e := range r.Unmapped {
		fmt.Fprintf(w, "  ? %s%s\n", e.Key, noteSuffix(e))
	}
	fmt.Fprintf(w, "risks (%d):\n", len(r.Risks))
	for _, e := range r.Risks {
		fmt.Fprintf(w, "  ! %s%s\n", e.Key, noteSuffix(e))
	}
	fmt.Fprintf(w, "reputation simulation: %s\n", r.ReputationSim.Status)
	fmt.Fprintf(w, "\nDry-run only: nothing was changed. Warmline never mutates a real ESP account.\n")
}

func noteSuffix(e migrate.Entry) string {
	if e.Note == "" {
		return ""
	}
	return " — " + e.Note
}
