package postmark

import (
	"encoding/json"
	"testing"

	"github.com/jonathanngiroux-star/warmline/internal/migrate"
)

const fixtureRel = "../../../testdata/fixtures/postmark/sample-export.json"

func TestParseFixture(t *testing.T) {
	f, err := ParseFile(fixtureRel)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if f.ExportVersion != "1.0" {
		t.Errorf("export_version = %q", f.ExportVersion)
	}
	if len(f.Domains) != 2 {
		t.Errorf("domains = %d, want 2", len(f.Domains))
	}
	if !f.Domains[0].DKIM {
		t.Error("first domain should have DKIM")
	}
	if len(f.Webhook.Events) != 6 {
		t.Errorf("webhook events = %d, want 6", len(f.Webhook.Events))
	}
}

func TestBuildReport(t *testing.T) {
	f, err := ParseFile(fixtureRel)
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.BuildReport()
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if r.Source != "postmark" {
		t.Errorf("source = %q", r.Source)
	}
	// Two domains, one DKIM-verified → both added (one with a note about
	// missing DKIM/return-path), one unverified → risk entry.
	if len(r.Added) != 2 {
		t.Errorf("added = %d, want 2", len(r.Added))
	}
	riskKeys := map[string]bool{}
	for _, risk := range r.Risks {
		riskKeys[risk.Key] = true
	}
	for _, want := range []string{"domain.unverified", "suppression", "webhook.event.subscription_change"} {
		if !riskKeys[want] {
			t.Errorf("risks missing %q (have %v)", want, riskKeys)
		}
	}
	// Postmark has no webhook OAuth and no legacy template engine: risks
	// must NOT contain the SendGrid-specific keys.
	if riskKeys["webhook.oauth"] || riskKeys["templates.legacy"] {
		t.Errorf("SendGrid-specific risk keys leaked into postmark report: %v", riskKeys)
	}
	// reputation_sim placeholder identical contract to sendgrid.
	if r.ReputationSim.Status != "planned" {
		t.Errorf("reputation_sim.status = %q", r.ReputationSim.Status)
	}
	if _, err := json.Marshal(r); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var _ migrate.Report = r
}

func TestWebhookEventMapping(t *testing.T) {
	f, err := ParseFile(fixtureRel)
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.BuildReport()
	if err != nil {
		t.Fatal(err)
	}
	// subscription_change has no canonical event → risk entry (not
	// silent). delivery/open/click/bounce/spam_complaint map.
	found := false
	for _, risk := range r.Risks {
		if risk.Key == "webhook.event.subscription_change" {
			found = true
		}
	}
	if !found {
		t.Error("subscription_change must surface as a risk (no canonical event)")
	}
}
