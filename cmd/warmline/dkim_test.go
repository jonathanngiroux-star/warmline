package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDKIMGenerateJSON(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"dkim", "generate", "--selector", "s2026", "--algorithm", "rsa", "--domain", "example.com", "--format=json", "--no-private-key"}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errB.String())
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out.String()), &m); err != nil {
		t.Fatalf("not json: %v\n%s", err, out.String())
	}
	if m["dns_host"] != "s2026._domainkey.example.com" {
		t.Errorf("dns_host = %v", m["dns_host"])
	}
	rec, _ := m["dns_record"].(string)
	if !strings.HasPrefix(rec, "v=DKIM1; k=rsa; p=") {
		t.Errorf("dns_record = %q", rec)
	}
	// Private key must NOT appear unless explicitly requested.
	if _, has := m["private_key_pem"]; has {
		t.Error("private key must be omitted without an explicit flag")
	}
}

func TestDKIMGeneratePrivateKeyOnlyOnRequest(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"dkim", "generate", "--selector", "s2026", "--algorithm", "ed25519", "--domain", "example.com", "--format=json"}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errB.String())
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out.String()), &m); err != nil {
		t.Fatalf("not json: %v", err)
	}
	pem, ok := m["private_key_pem"].(string)
	if !ok || !strings.Contains(pem, "BEGIN PRIVATE KEY") {
		t.Error("private_key_pem missing in full output")
	}
}

func TestDKIMGenerateText(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"dkim", "generate", "--selector", "s2026", "--algorithm", "rsa", "--domain", "example.com", "--no-private-key"}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errB.String())
	}
	if !strings.Contains(out.String(), "s2026._domainkey.example.com") {
		t.Errorf("text output missing dns host:\n%s", out.String())
	}
}

func TestDKIMRotate(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"dkim", "rotate", "--old", "s2025", "--new", "s2026"}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errB.String())
	}
	if !strings.Contains(out.String(), "s2025 -> s2026") {
		t.Errorf("rotate output missing plan:\n%s", out.String())
	}
}

func TestDKIMBadAlgorithm(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"dkim", "generate", "--selector", "s", "--algorithm", "dsa", "--domain", "example.com"}, &out, &errB)
	if code == 0 {
		t.Error("dsa should fail")
	}
}
