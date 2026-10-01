package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/jonathanngiroux-star/warmline/internal/migrate/postmark"
)

// migratePostmark prints the Postmark dry-run report (same shape as the
// SendGrid one; gaps listed, no fake 100%).
func migratePostmark(format, input string, stdout, stderr io.Writer) int {
	if input == "" {
		fmt.Fprintln(stderr, "migrate: --input is required (path to your Postmark export JSON)")
		return 2
	}
	exp, err := postmark.ParseFile(input)
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
