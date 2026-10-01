package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/jonathanngiroux-star/warmline/internal/dkim"
)

// cmdDKIM: generate keys / print rotation plans. Warmline generates
// config; the operator publishes DNS. Warmline is not the CA.
func cmdDKIM(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: warmline dkim <generate|rotate> [flags]")
		return 2
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "generate":
		return dkimGenerate(rest, stdout, stderr)
	case "rotate":
		return dkimRotate(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "dkim: unknown subcommand %q (generate|rotate)\n", sub)
		return 2
	}
}

func dkimGenerate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dkim generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	selector := fs.String("selector", "", "DKIM selector (e.g. s2026)")
	algorithm := fs.String("algorithm", "rsa", "rsa or ed25519")
	domain := fs.String("domain", "", "signing domain for the DNS record preview")
	format := fs.String("format", "text", "text or json")
	noPriv := fs.Bool("no-private-key", false, "omit the private key from output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *selector == "" {
		fmt.Fprintln(stderr, "dkim generate: --selector is required")
		return 2
	}
	if *domain == "" {
		fmt.Fprintln(stderr, "dkim generate: --domain is required (for the DNS record preview)")
		return 2
	}
	key, err := dkim.Generate(*selector, *algorithm)
	if err != nil {
		fmt.Fprintf(stderr, "dkim generate: %v\n", err)
		return 1
	}
	switch *format {
	case "json":
		m := map[string]any{
			"selector":   key.Selector,
			"algorithm":  key.Algorithm,
			"dns_host":   key.DNSHost(*domain),
			"dns_record": key.DNSRecord(*domain),
		}
		if !*noPriv {
			m["private_key_pem"] = key.PrivateKeyPEM
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(m); err != nil {
			fmt.Fprintf(stderr, "dkim generate: encode: %v\n", err)
			return 1
		}
	case "text":
		fmt.Fprintf(stdout, "DKIM key generated for selector %q (%s)\n\n", key.Selector, key.Algorithm)
		fmt.Fprintf(stdout, "Publish this TXT record:\n  host:  %s\n  value: %s\n\n", key.DNSHost(*domain), key.DNSRecord(*domain))
		if !*noPriv {
			fmt.Fprintf(stdout, "Private key (PEM, keep off the repo, 600 perms):\n%s\n", key.PrivateKeyPEM)
		} else {
			fmt.Fprintln(stdout, "(private key omitted — re-run without --no-private-key to print it)")
		}
		fmt.Fprintln(stdout, "Warmline generates keys and config; you publish DNS. Warmline is not the CA.")
	default:
		fmt.Fprintf(stderr, "dkim generate: unknown --format %q\n", *format)
		return 2
	}
	return 0
}

func dkimRotate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dkim rotate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	old := fs.String("old", "", "current selector")
	new := fs.String("new", "", "new selector")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *old == "" || *new == "" {
		fmt.Fprintln(stderr, "dkim rotate: --old and --new are required")
		return 2
	}
	plan := dkim.PlanRotation(*old, *new)
	fmt.Fprint(stdout, plan.Markdown())
	return 0
}
