package main

// wizard_interactive.go: the setup wizard's interactive schema — every
// step carries the real input fields and runs the real operation on
// the user's values. Shared by the TUI and Wails GUI renderers.

import (
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"strings"

	samples "github.com/jonathanngiroux-star/warmline"
	"github.com/jonathanngiroux-star/warmline/internal/mta"
	"github.com/jonathanngiroux-star/warmline/internal/simulate"
	"github.com/jonathanngiroux-star/warmline/internal/store"
)

// wizardField is one user input on a wizard page.
type wizardField struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Value   string   `json:"value"`
	Hint    string   `json:"hint"`
	Kind    string   `json:"kind"`
	Options []string `json:"options,omitempty"`
}

// fieldValues is what a renderer hands back when the user presses Run.
type fieldValues map[string]string

// wizardRunStep executes a step's real operation on the user's values.
func wizardRunStep(m *tuiModel, stepID string, v fieldValues) (string, error) {
	switch stepID {
	case "welcome", "donate":
		return "", fmt.Errorf("wizard step %q has no runnable action", stepID)
	case "migrate":
		return wizardMigrate(m, v)
	case "simulate":
		return wizardSimulate(m, v)
	case "dkim":
		return wizardDKIM(m, v)
	case "serve":
		return wizardServe(m, v)
	default:
		return "", fmt.Errorf("unknown wizard step %q", stepID)
	}
}

// wizardMigrate runs a dry-run over the USER's export file with the
// user's chosen source. Errors name the offending path.
func wizardMigrate(m *tuiModel, v fieldValues) (string, error) {
	source := strings.TrimSpace(v["source"])
	input := strings.TrimSpace(v["input"])
	if input == "" {
		return "", fmt.Errorf("export path is required — pick your ESP export JSON")
	}
	out, err := m.migrateDryRun(source, input)
	if err != nil {
		return "", fmt.Errorf("%w — check the export path %q", err, input)
	}
	return out, nil
}

// wizardSimulate runs the reputation trajectory on the USER's declared
// numbers. Every plan field is a wizard field; errors name the field.
func wizardSimulate(m *tuiModel, v fieldValues) (string, error) {
	var errs []string
	getInt := func(key string) int {
		n, err := strconv.Atoi(strings.TrimSpace(v[key]))
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s must be a whole number (got %q)", key, v[key]))
		}
		return n
	}
	getFloat := func(key string) float64 {
		f, err := strconv.ParseFloat(strings.TrimSpace(v[key]), 64)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s must be a number (got %q)", key, v[key]))
		}
		return f
	}
	plan := simulate.Plan{
		VolumeStart:   getInt("volume_start"),
		VolumeTarget:  getInt("volume_target"),
		RampDays:      getInt("ramp_days"),
		Days:          getInt("days"),
		BounceRate:    getFloat("bounce_rate"),
		ComplaintRate: getFloat("complaint_rate"),
		IPAge:         strings.TrimSpace(v["ip_age"]),
	}
	if len(errs) > 0 {
		return "", fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	if err := plan.Validate(); err != nil {
		return "", err
	}
	r, err := simulate.Run(&plan)
	if err != nil {
		return "", err
	}
	return simulate.Markdown(r), nil
}

// wizardDKIM generates a real keypair for the USER's domain and
// selector; the record is recorded in the store and returned as the
// DNS record to publish.
func wizardDKIM(m *tuiModel, v fieldValues) (string, error) {
	domain := strings.TrimSpace(v["domain"])
	selector := strings.TrimSpace(v["selector"])
	algorithm := strings.TrimSpace(v["algorithm"])
	if domain == "" {
		return "", fmt.Errorf("domain is required (e.g. example.com)")
	}
	if selector == "" {
		return "", fmt.Errorf("selector is required (e.g. s2026)")
	}
	rec, err := m.dkimGenerate(domain, selector, algorithm)
	if err != nil {
		return "", fmt.Errorf("%w — check domain %q and selector %q", err, domain, selector)
	}
	return "Publish this TXT record:\n" + rec +
		"\n\nPrivate key saved next to the db file (never shown).", nil
}

// wizardServe probes the REAL local stack with the user's values: it
// boots the actual SMTP front (the same mta.Server `warmline serve`
// uses) on the user's chosen address, submits a test message through a
// real SMTP dialogue, verifies the message landed in the queue, then
// shuts the listener down. A throwaway probe db — the user's real db is
// never touched. It never sends mail anywhere: the probe message stays
// in the local queue (no relay wired).
func wizardServe(m *tuiModel, v fieldValues) (string, error) {
	smtpAddr := strings.TrimSpace(v["smtp"])
	httpAddr := strings.TrimSpace(v["http"])
	if smtpAddr == "" {
		return "", fmt.Errorf("smtp listen address is required (e.g. 127.0.0.1:2525)")
	}
	if httpAddr == "" {
		return "", fmt.Errorf("http ui address is required (e.g. 127.0.0.1:8080)")
	}

	// Throwaway probe db — never the user's real queue db.
	probeDB, err := os.CreateTemp("", "warmline-wizard-probe-*.db")
	if err != nil {
		return "", fmt.Errorf("probe db: %w", err)
	}
	probePath := probeDB.Name()
	probeDB.Close()
	defer os.Remove(probePath)

	st, err := store.Open(probePath)
	if err != nil {
		return "", fmt.Errorf("probe db open: %w", err)
	}
	defer st.Close()

	// The real SMTP front, same wiring serve.Run uses.
	srv := mta.New()
	srv.SetHandler(queueStoreHandler{db: st})
	if err := srv.Listen(smtpAddr); err != nil {
		return "", fmt.Errorf("listen %s: %w — is the port free? try another smtp address", smtpAddr, err)
	}
	go srv.Serve()
	defer srv.Close()

	// Real SMTP submit (RFC 5321 dialogue, net/smtp client).
	msg := "Subject: warmline wizard probe\r\n\r\nThis message proves your local stack works.\r\n"
	if err := smtp.SendMail(smtpAddr, nil, "probe@warmline.local", []string{"probe@warmline.local"}, []byte(msg)); err != nil {
		return "", fmt.Errorf("smtp submit to %s: %w", smtpAddr, err)
	}

	// Verify the message landed in the queue.
	msgs, err := st.ListMessages(10)
	if err != nil {
		return "", fmt.Errorf("queue read: %w", err)
	}
	found := false
	for _, mm := range msgs {
		if mm.Recipient == "probe@warmline.local" && mm.Status == "queued" {
			found = true
		}
	}
	if !found {
		return "", fmt.Errorf("probe message did not land in the queue — submit silently failed")
	}

	return fmt.Sprintf("probe ok\n\n  smtp submission: %s — accepted a message into the queue\n"+
		"  probe db: %s (removed)\n  http ui: http://%s/ — starts when you run `warmline serve`\n\n"+
		"Your stack works. Run it for real:\n  warmline serve --smtp %s --http %s\n"+
		"(mail waits in the queue until you wire --relay; Warmline never sends mail itself)",
		smtpAddr, probePath, httpAddr, smtpAddr, httpAddr), nil
}

// queueStoreHandler adapts store to mta.Handler (same wiring serve.Run
// uses; duplicated here because serve keeps it private).
type queueStoreHandler struct{ db *store.Store }

func (h queueStoreHandler) Submit(mm mta.Message) error {
	for _, to := range mm.To {
		if _, err := h.db.Enqueue(to, mm.Data, map[string]string{"from": mm.From}); err != nil {
			return err
		}
	}
	return nil
}

// sampleExportPath materializes the embedded sample export to a temp
// file and returns its path, so the migrate step works for users who
// have no ESP export yet ("try it on the sample first").
func sampleExportPath() (string, error) {
	f, err := os.CreateTemp("", "warmline-sample-export-*.json")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(samples.SendgridSampleExport); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}
