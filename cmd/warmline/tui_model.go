package main

// tui_model.go: the shared logic layer under both the TUI and the GUI.
// Every mutation and read goes through tuiModel — fully testable without
// a TTY or display; the renderers (tui.go, desktop_fyne.go) are thin.

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/jonathanngiroux-star/warmline/internal/dkim"
	"github.com/jonathanngiroux-star/warmline/internal/migrate"
	"github.com/jonathanngiroux-star/warmline/internal/migrate/postmark"
	"github.com/jonathanngiroux-star/warmline/internal/migrate/sendgrid"
	"github.com/jonathanngiroux-star/warmline/internal/simulate"
	"github.com/jonathanngiroux-star/warmline/internal/store"
)

// tuiModel owns the store handle and every operation the TUI/GUI can
// perform. One logic layer, two renderers.
type tuiModel struct {
	db *store.Store
}

func newTUIModel(db *store.Store) *tuiModel { return &tuiModel{db: db} }

// view is a renderable snapshot: a summary line plus display rows.
type view struct {
	summary string
	rows    []string
}

// queueView lists recent queue messages with a status summary.
func (m *tuiModel) queueView() (view, error) {
	msgs, err := m.db.ListMessages(100)
	if err != nil {
		return view{}, err
	}
	bounces, err := m.db.ListBounces(100)
	if err != nil {
		return view{}, err
	}
	complaints, err := m.db.ListComplaints(100)
	if err != nil {
		return view{}, err
	}
	status := map[string]int{}
	for _, msg := range msgs {
		status[msg.Status]++
	}
	v := view{summary: fmt.Sprintf("total %d · queued %d · processing %d · sent %d · bounces %d · complaints %d",
		len(msgs), status["queued"], status["processing"], status["sent"], len(bounces), len(complaints))}
	for _, msg := range msgs {
		v.rows = append(v.rows, fmt.Sprintf("#%-4d %-32s %-10s %s", msg.ID, msg.Recipient, msg.Status, msg.CreatedAt))
	}
	if len(v.rows) == 0 {
		v.rows = append(v.rows, "(queue empty — submit mail to the SMTP port)")
	}
	return v, nil
}

// bounceView lists recent classified bounces, newest first.
func (m *tuiModel) bounceView() (view, error) {
	bs, err := m.db.ListBounces(100)
	if err != nil {
		return view{}, err
	}
	v := view{summary: fmt.Sprintf("%d bounces", len(bs))}
	for _, b := range bs {
		origin := "local"
		if b.MessageID == 0 {
			origin = "esp"
		}
		v.rows = append(v.rows, fmt.Sprintf("%-5s %-6s %-32s %s %s", origin, b.Class, b.Recipient, b.SMTPCode, b.Diagnostic))
	}
	if len(v.rows) == 0 {
		v.rows = append(v.rows, "(no bounces recorded)")
	}
	return v, nil
}

// dkimView lists DKIM selectors, newest first.
func (m *tuiModel) dkimView() (view, error) {
	ks, err := m.db.ListDKIMs()
	if err != nil {
		return view{}, err
	}
	v := view{summary: fmt.Sprintf("%d selectors", len(ks))}
	for _, k := range ks {
		v.rows = append(v.rows, fmt.Sprintf("%-8s %-24s created %s", k.Selector, k.Domain, k.CreatedAt))
	}
	if len(v.rows) == 0 {
		v.rows = append(v.rows, "(no DKIM selectors — generate one)")
	}
	return v, nil
}

// dkimGenerate creates a key, records the selector in the store, and
// returns the DNS record to publish. Private key is written next to the
// db (0600) — never displayed in the UI.
func (m *tuiModel) dkimGenerate(domain, selector, algorithm string) (string, error) {
	key, err := dkim.Generate(selector, algorithm)
	if err != nil {
		return "", err
	}
	if err := m.db.UpsertDKIM(domain, selector, key.PublicKey, nowISO()); err != nil {
		return "", err
	}
	// Persist the private key next to the db file for the signer to read.
	dbPath := m.db.Path()
	if dbPath != "" {
		pemPath := dbPath + "." + selector + ".pem"
		if err := os.WriteFile(pemPath, []byte(key.PrivateKeyPEM), 0o600); err == nil {
			_ = pemPath // best-effort: UI still works read-only
		}
	}
	return key.DNSRecord(domain), nil
}

// migrateDryRun runs the same report the CLI prints, as text.
func (m *tuiModel) migrateDryRun(source, input string) (string, error) {
	switch source {
	case "sendgrid":
		exp, err := sendgrid.ParseFile(input)
		if err != nil {
			return "", err
		}
		r, err := exp.BuildReport()
		if err != nil {
			return "", err
		}
		r.Input = input
		return reportText(r), nil
	case "postmark":
		exp, err := postmark.ParseFile(input)
		if err != nil {
			return "", err
		}
		r, err := exp.BuildReport()
		if err != nil {
			return "", err
		}
		r.Input = input
		return reportText(r), nil
	default:
		return "", fmt.Errorf("unsupported source %q (sendgrid, postmark)", source)
	}
}

// simulate runs the reputation trajectory and returns markdown.
func (m *tuiModel) simulate(planPath string) (string, error) {
	plan, err := simulate.LoadPlan(planPath)
	if err != nil {
		return "", err
	}
	r, err := simulate.Run(plan)
	if err != nil {
		return "", err
	}
	return simulate.Markdown(r), nil
}

// donate returns the donation text (both addresses).
func (m *tuiModel) donate() string {
	return "Warmline is free to self-host. If it saves you a SendGrid week, donate.\n\n" +
		"Ethereum / USDC (ERC-20): " + donateEthereum + "\n" +
		"Bitcoin:                  " + donateBitcoin + "\n"
}

// reportText renders a migrate report as text (shared with the CLI's
// text renderer so both surfaces never drift).
func reportText(r migrate.Report) string {
	var buf bytes.Buffer
	printReportText(r, &buf)
	return buf.String()
}

func nowISO() string { return time.Now().UTC().Format(time.RFC3339) }
