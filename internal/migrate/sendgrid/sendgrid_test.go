package sendgrid

import (
	"encoding/json"
	"testing"

	"github.com/jonathanngiroux-star/warmline/internal/migrate"
)

const fixtureRel = "../../../testdata/fixtures/sendgrid/sample-export.json"

func TestParseFixture(t *testing.T) {
	f, err := ParseFile(fixtureRel)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if f.ExportVersion != "1.0" {
		t.Errorf("export_version = %q, want 1.0", f.ExportVersion)
	}
	if len(f.AuthDomains) != 1 {
		t.Fatalf("auth domains = %d, want 1", len(f.AuthDomains))
	}
	if f.AuthDomains[0].Domain != "mail.example.com" {
		t.Errorf("domain = %q, want mail.example.com", f.AuthDomains[0].Domain)
	}
	if len(f.AuthDomains[0].DNSRecords) != 4 {
		t.Errorf("dns records = %d, want 4", len(f.AuthDomains[0].DNSRecords))
	}
	if len(f.IPPools) != 2 {
		t.Errorf("ip pools = %d, want 2", len(f.IPPools))
	}
	if !f.Webhook.Enabled {
		t.Error("webhook should be enabled in fixture")
	}
	if len(f.Webhook.Events) != 11 {
		t.Errorf("webhook events = %d, want 11", len(f.Webhook.Events))
	}
}

func TestParseFileMissing(t *testing.T) {
	if _, err := ParseFile("/nonexistent.json"); err == nil {
		t.Fatal("ParseFile on missing file should error")
	}
}

func TestBuildReport(t *testing.T) {
	f, err := ParseFile(fixtureRel)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	r, err := f.BuildReport()
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if r.Source != "sendgrid" {
		t.Errorf("source = %q, want sendgrid", r.Source)
	}
	if len(r.Added) != 1 {
		t.Errorf("added = %d, want 1", len(r.Added))
	}
	if len(r.Added) == 1 && r.Added[0].Key != "domain.mail.example.com" {
		t.Errorf("added[0].key = %q, want domain.mail.example.com", r.Added[0].Key)
	}
	if len(r.Unmapped) != 2 {
		t.Fatalf("unmapped = %d, want 2 (dropped, group_resubscribe)", len(r.Unmapped))
	}
	if r.Unmapped[0].Key != "webhook.event.dropped" {
		t.Errorf("unmapped[0] = %q, want webhook.event.dropped", r.Unmapped[0].Key)
	}
	if r.Unmapped[1].Key != "webhook.event.group_resubscribe" {
		t.Errorf("unmapped[1] = %q, want webhook.event.group_resubscribe", r.Unmapped[1].Key)
	}
	wantRisks := []string{"ip_pools", "suppression", "templates.legacy", "webhook.oauth"}
	riskKeys := map[string]bool{}
	for _, risk := range r.Risks {
		riskKeys[risk.Key] = true
	}
	for _, want := range wantRisks {
		if !riskKeys[want] {
			t.Errorf("risks missing %q (have %v)", want, riskKeys)
		}
	}
	if r.ReputationSim.Status != "planned" {
		t.Errorf("reputation_sim.status = %q, want planned", r.ReputationSim.Status)
	}
	if r.ReputationSim.Note == "" {
		t.Error("reputation_sim.note is empty")
	}
	if _, err := json.Marshal(r); err != nil {
		t.Fatalf("report must marshal: %v", err)
	}
}

// Compile-time contract: the sendgrid report must satisfy the shared
// report interface so the CLI can treat sources uniformly.
var _ = migrate.Report{} // report value type; constructors return migrate.Report

// ParseBytes: the wizard and GUI parse embedded fixture bytes, not
// files — the desktop binary runs outside any source checkout.
func TestParseBytes(t *testing.T) {
	e, err := ParseBytes([]byte(`{"export_version":"1"}`))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if e.ExportVersion != "1" {
		t.Errorf("version = %q", e.ExportVersion)
	}
	if _, err := ParseBytes([]byte("{")); err == nil {
		t.Error("malformed JSON must error")
	}
}
