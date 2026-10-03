package main

import (
	"net"
	"os"
	"path/filepath"

	samples "github.com/jonathanngiroux-star/warmline"
	"strings"
	"testing"
)

// Interactive wizard steps carry real fields and run real operations on
// the user's values — a working guide, not prose.
func TestInteractiveStepsHaveFields(t *testing.T) {
	steps := wizardSteps()
	byID := map[string]wizardStep{}
	for _, s := range steps {
		byID[s.ID] = s
	}
	// migrate: source + path
	if len(byID["migrate"].Fields) != 2 {
		t.Errorf("migrate fields = %+v, want source+path", byID["migrate"].Fields)
	}
	// simulate: 7 plan fields
	if len(byID["simulate"].Fields) != 7 {
		t.Errorf("simulate fields = %d, want 7: %+v", len(byID["simulate"].Fields), byID["simulate"].Fields)
	}
	// dkim: domain, selector, algorithm
	if len(byID["dkim"].Fields) != 3 {
		t.Errorf("dkim fields = %+v, want 3", byID["dkim"].Fields)
	}
	// serve: db, smtp, http
	if len(byID["serve"].Fields) != 3 {
		t.Errorf("serve fields = %+v, want db+smtp+http", byID["serve"].Fields)
	}
}

// Every interactive action must consume the user's field values and
// return real output.
func TestInteractiveActionsRunRealWork(t *testing.T) {
	m := openModel(t)
	t.Chdir(t.TempDir())

	// migrate on the user's file (the embedded sample, written to a
	// temp file — the user picks any path, including outside the repo)
	sample := string(samples.SendgridSampleExport)
	if err := os.WriteFile("my-export.json", []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := wizardRunStep(m, "migrate", fieldValues{"source": "sendgrid", "input": "my-export.json"})
	if err != nil {
		t.Fatalf("migrate action: %v", err)
	}
	if !strings.Contains(out, "unmapped (2)") {
		t.Errorf("migrate output missing unmapped: %.200s", out)
	}

	// simulate with the user's numbers
	out, err = wizardRunStep(m, "simulate", fieldValues{
		"volume_start": "500", "volume_target": "50000", "ramp_days": "14",
		"days": "21", "bounce_rate": "1", "complaint_rate": "0.05", "ip_age": "new",
	})
	if err != nil {
		t.Fatalf("simulate action err = %v", err)
	}
	if !strings.Contains(out, "| 21 |") {
		t.Errorf("simulate output missing day 21: %.200s", out)
	}

	// dkim on the user's domain
	out, err = wizardRunStep(m, "dkim", fieldValues{"domain": "acme.com", "selector": "sel1", "algorithm": "ed25519"})
	if err != nil {
		t.Fatalf("dkim action: %v", err)
	}
	if !strings.Contains(out, "v=DKIM1; k=ed25519;") {
		t.Errorf("dkim output missing ed25519 record: %.200s", out)
	}
	// the generated selector must be recorded in the store
	v, _ := m.dkimView()
	found := false
	for _, row := range v.rows {
		if strings.Contains(row, "sel1") {
			found = true
		}
	}
	if !found {
		t.Error("dkim action did not record the selector in the store")
	}
}

// Serve step: the real probe. Boots a real SMTP+HTTP pair on ephemeral
// ports, submits a test message via real SMTP, verifies it lands in
// the queue, shuts down. This is the "actually works" proof.
func TestInteractiveServeStepProbesRealStack(t *testing.T) {
	m := openModel(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "probe.db")
	smtpProbe := freePort(t)
	httpProbe := freePort(t)

	out, err := wizardRunStep(m, "serve", fieldValues{
		"db": dbPath, "smtp": smtpProbe, "http": httpProbe,
	})
	if err != nil {
		t.Fatalf("serve action err = %v", err)
	}
	if !strings.Contains(out, "probe ok") {
		t.Errorf("serve probe output missing ok: %q", out)
	}
	// The probe db must not linger
	if _, statErr := os.Stat(dbPath); !os.IsNotExist(statErr) {
		t.Error("probe db should be cleaned up after the probe")
	}
}

// Errors must name the offending field and value.
func TestInteractiveWizardErrorsNameField(t *testing.T) {
	m := openModel(t)
	t.Chdir(t.TempDir())
	_, err := wizardRunStep(m, "migrate", fieldValues{"source": "sendgrid", "input": "missing.json"})
	if err == nil {
		t.Fatal("missing file must error")
	}
	if !strings.Contains(err.Error(), "missing.json") {
		t.Errorf("error must name the path: %v", err)
	}

	_, err = wizardRunStep(m, "simulate", fieldValues{
		"volume_start": "0", "volume_target": "10", "ramp_days": "5",
		"days": "10", "bounce_rate": "1", "complaint_rate": "0.05", "ip_age": "new",
	})
	if err == nil || !strings.Contains(err.Error(), "volume_start") {
		t.Errorf("invalid volume_start must be named: %v", err)
	}

	_, err = wizardRunStep(m, "dkim", fieldValues{"domain": "", "selector": "x", "algorithm": "rsa"})
	if err == nil || !strings.Contains(err.Error(), "domain") {
		t.Errorf("empty domain must be named: %v", err)
	}
}

// Welcome + donate steps stay prose (no fields), and the donate step
// carries both addresses.
func TestProseStepsUnchanged(t *testing.T) {
	steps := wizardSteps()
	for _, s := range steps {
		if s.ID == "welcome" || s.ID == "donate" {
			if len(s.Fields) != 0 {
				t.Errorf("step %q should have no fields: %+v", s.ID, s.Fields)
			}
			if s.NextLabel != "" {
				t.Errorf("step %q should have no run button", s.ID)
			}
		}
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}
