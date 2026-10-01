package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/jonathanngiroux-star/warmline/internal/serve"
	"github.com/jonathanngiroux-star/warmline/internal/store"
)

// cmdServe runs the local stack: SMTP submission front + SQLite queue +
// HTTP UI (queue page, /donate, webhook ingest). Outbound delivery is
// NOT included — relay credentials are user-supplied and will be wired
// in a later milestone (see docs). Ctrl-C stops cleanly.
func cmdServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", "warmline.db", "path to the SQLite queue database")
	smtpAddr := fs.String("smtp", "127.0.0.1:2525", "SMTP submission listen address")
	httpAddr := fs.String("http", "127.0.0.1:8080", "HTTP UI listen address")
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

	if err := serve.Run(db, serve.Options{SMTPAddr: *smtpAddr, HTTPAddr: *httpAddr}); err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 1
	}
	return 0
}
