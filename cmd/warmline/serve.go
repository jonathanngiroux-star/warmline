package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/jonathanngiroux-star/warmline/internal/relay"
	"github.com/jonathanngiroux-star/warmline/internal/serve"
	"github.com/jonathanngiroux-star/warmline/internal/store"
)

// cmdServe runs the local stack: SMTP submission front + SQLite queue +
// HTTP UI (queue page, /donate, webhook ingest). Outbound delivery is
// NOT included — relay credentials are user-supplied and will be wired
// in a later milestone (see docs). Ctrl-C stops cleanly.
//
// Flags fall back to WARMLINE_DB / WARMLINE_SMTP / WARMLINE_HTTP env
// vars (the Docker image sets the bind addresses to 0.0.0.0).
func cmdServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", envDefault("WARMLINE_DB", "warmline.db"), "path to the SQLite queue database")
	smtpAddr := fs.String("smtp", envDefault("WARMLINE_SMTP", "127.0.0.1:2525"), "SMTP submission listen address")
	httpAddr := fs.String("http", envDefault("WARMLINE_HTTP", "127.0.0.1:8080"), "HTTP UI listen address")
	relayAddr := fs.String("relay", envDefault("WARMLINE_RELAY", ""), "optional outbound relay host:port YOU operate or rent (e.g. smtp.yourprovider.com:587)")
	relayUser := fs.String("relay-user", envDefault("WARMLINE_RELAY_USER", ""), "relay username (optional)")
	relayPass := fs.String("relay-pass", envDefault("WARMLINE_RELAY_PASS", ""), "relay password (optional; prefer env var)")
	relayFrom := fs.String("relay-from", envDefault("WARMLINE_RELAY_FROM", ""), "envelope-from override (optional)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "serve: open db: %v\n", err)
		return 1
	}
	defer db.Close()

	fmt.Fprintf(stdout, "warmline serve\n")
	fmt.Fprintf(stdout, "  smtp submission: %s\n", *smtpAddr)
	fmt.Fprintf(stdout, "  http ui:         http://%s/\n", *httpAddr)
	fmt.Fprintf(stdout, "  queue db:        %s\n", *dbPath)
	fmt.Fprintf(stdout, "  donate:          http://%s/donate\n", *httpAddr)
	if *relayAddr != "" {
		fmt.Fprintf(stdout, "  outbound relay:   %s (user-supplied)\n", *relayAddr)
	} else {
		fmt.Fprintf(stdout, "  outbound relay:   none (queue holds mail until you configure --relay)\n")
	}
	fmt.Fprintf(stdout, "Outbound delivery is user-supplied-relay only — Warmline does not send mail itself.\n")

	// Ctrl-C -> clean shutdown.
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		fmt.Fprintln(stdout, "\nshutting down")
		db.Close()
		os.Exit(0)
	}()

	opts := serve.Options{SMTPAddr: *smtpAddr, HTTPAddr: *httpAddr}
	if *relayAddr != "" {
		opts.Relay = relay.Config{Addr: *relayAddr, Username: *relayUser, Password: *relayPass, From: *relayFrom}
	}
	if err := serve.Run(db, opts); err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 1
	}
	return 0
}

// envDefault returns the env var value or fallback if unset.
func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
