package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonathanngiroux-star/warmline/internal/store"
)

func openModel(t *testing.T) *tuiModel {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "tui.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return newTUIModel(db)
}

func TestModelQueueView(t *testing.T) {
	m := openModel(t)
	if _, err := m.db.Enqueue("a@example.net", "m1", nil); err != nil {
		t.Fatal(err)
	}
	id, err := m.db.Enqueue("b@example.net", "m2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.Dequeue(); err != nil {
		t.Fatal(err)
	}
	if err := m.db.MarkSent(id, "relay-x"); err != nil {
		t.Fatal(err)
	}

	v, err := m.queueView()
	if err != nil {
		t.Fatalf("queueView: %v", err)
	}
	// summary line: total + per-status
	if !strings.Contains(v.summary, "total 2") {
		t.Errorf("summary = %q, want 'total 2'", v.summary)
	}
	if !strings.Contains(v.summary, "sent 1") {
		t.Errorf("summary = %q, want 'sent 1'", v.summary)
	}
	if len(v.rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(v.rows))
	}
	if !strings.Contains(v.rows[0], "b@example.net") {
		t.Errorf("first row should be newest: %q", v.rows[0])
	}
	if !strings.Contains(v.rows[1], "a@example.net") {
		t.Errorf("second row: %q", v.rows[1])
	}
	// each row carries id, recipient, status
	if !strings.Contains(v.rows[0], "sent") {
		t.Errorf("row missing status: %q", v.rows[0])
	}
}

func TestModelBounceView(t *testing.T) {
	m := openModel(t)
	id, _ := m.db.Enqueue("a@example.net", "m", nil)
	if err := m.db.RecordBounce(id, "a@example.net", "550", "no user", "hard"); err != nil {
		t.Fatal(err)
	}
	// ESP-originated (no local row) — message_id 0 path
	if err := m.db.RecordBounce(0, "webhook@example.net", "421", "busy", "soft"); err != nil {
		t.Fatal(err)
	}
	v, err := m.bounceView()
	if err != nil {
		t.Fatal(err)
	}
	if len(v.rows) != 2 {
		t.Fatalf("bounce rows = %d, want 2", len(v.rows))
	}
	if !strings.Contains(v.rows[0], "webhook@example.net") {
		t.Errorf("newest bounce first: %q", v.rows[0])
	}
	if !strings.Contains(v.rows[0], "soft") {
		t.Errorf("row missing class: %q", v.rows[0])
	}
	if !strings.Contains(v.rows[1], "hard") {
		t.Errorf("row missing class: %q", v.rows[1])
	}
}

func TestModelDKIMView(t *testing.T) {
	m := openModel(t)
	if err := m.db.UpsertDKIM("example.com", "s2025", "pub1", "2026-09-01"); err != nil {
		t.Fatal(err)
	}
	if err := m.db.UpsertDKIM("example.com", "s2026", "pub2", "2026-10-01"); err != nil {
		t.Fatal(err)
	}
	v, err := m.dkimView()
	if err != nil {
		t.Fatal(err)
	}
	if len(v.rows) != 2 {
		t.Fatalf("dkim rows = %d, want 2", len(v.rows))
	}
	if !strings.Contains(v.rows[0], "s2026") {
		t.Errorf("newest selector first: %q", v.rows[0])
	}
	if !strings.Contains(v.rows[1], "s2025") {
		t.Errorf("second selector: %q", v.rows[1])
	}
}

func TestModelDKIMGenerate(t *testing.T) {
	m := openModel(t)
	// Generate records the new selector in the store AND returns the DNS
	// record text — one code path for TUI, GUI, and CLI.
	rec, err := m.dkimGenerate("example.com", "s2026", "rsa")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(rec, "v=DKIM1; k=rsa; p=") {
		t.Errorf("record malformed: %q", rec)
	}
	v, _ := m.dkimView()
	if len(v.rows) != 1 || !strings.Contains(v.rows[0], "s2026") {
		t.Errorf("generated selector not listed: %+v", v.rows)
	}
}

func TestModelMigrateDryRun(t *testing.T) {
	m := openModel(t)
	out, err := m.migrateDryRun("sendgrid", "../../testdata/fixtures/sendgrid/sample-export.json")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// reuses the CLI's report text: risks + unmapped must appear
	if !strings.Contains(out, "unmapped (2)") {
		t.Errorf("migrate output missing unmapped: %q", out[:200])
	}
	if !strings.Contains(out, "ip_pools") {
		t.Errorf("migrate output missing risks: %q", out[:200])
	}
}

func TestModelSimulate(t *testing.T) {
	m := openModel(t)
	md, err := m.simulate("../../testdata/simulate/plan.json")
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if !strings.Contains(md, "Not a deliverability guarantee") {
		t.Error("simulate output missing disclaimer")
	}
	if !strings.Contains(md, "| 1 |") {
		t.Errorf("simulate output missing day rows:\n%s", md)
	}
}

func TestModelDonate(t *testing.T) {
	m := openModel(t)
	d := m.donate()
	if !strings.Contains(d, "0x85ee7E71f762d772599cbF1EC20E651B30657521") {
		t.Error("donate missing ETH address")
	}
	if !strings.Contains(d, "bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg") {
		t.Error("donate missing BTC address")
	}
}
