package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/jonathanngiroux-star/warmline/internal/simulate"
)

// cmdSimulate runs the reputation trajectory simulation over a declared
// plan. Deterministic: same plan → same curve. Simulation, not a
// deliverability guarantee.
func cmdSimulate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("simulate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	planPath := fs.String("plan", "", "path to plan JSON (volume, rates, ip_age)")
	format := fs.String("format", "text", "output format: text (markdown table) or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *planPath == "" {
		fmt.Fprintln(stderr, "simulate: --plan is required (see docs/simulate.md for the schema)")
		return 2
	}
	plan, err := simulate.LoadPlan(*planPath)
	if err != nil {
		fmt.Fprintf(stderr, "simulate: %v\n", err)
		return 1
	}
	result, err := simulate.Run(plan)
	if err != nil {
		fmt.Fprintf(stderr, "simulate: %v\n", err)
		return 1
	}
	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(stderr, "simulate: encode: %v\n", err)
			return 1
		}
	case "text":
		fmt.Fprint(stdout, simulate.Markdown(result))
	default:
		fmt.Fprintf(stderr, "simulate: unknown --format %q (want text or json)\n", *format)
		return 2
	}
	return 0
}
