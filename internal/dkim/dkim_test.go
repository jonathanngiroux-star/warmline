package dkim

import (
	"strings"
	"testing"
)

func TestGenerateRSA(t *testing.T) {
	k, err := Generate("s2026", "rsa")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if k.Selector != "s2026" {
		t.Errorf("selector = %q", k.Selector)
	}
	if k.Algorithm != "rsa" {
		t.Errorf("algorithm = %q", k.Algorithm)
	}
	if !strings.HasPrefix(k.DNSRecord("example.com"), "v=DKIM1; k=rsa; p=") {
		t.Errorf("dns record malformed: %q", k.DNSRecord("example.com"))
	}
	if k.DNSHost("example.com") != "s2026._domainkey.example.com" {
		t.Errorf("dns host = %q", k.DNSHost("example.com"))
	}
	if !strings.Contains(k.PrivateKeyPEM, "BEGIN PRIVATE KEY") {
		t.Error("private key not PEM")
	}
}

func TestGenerateEd25519(t *testing.T) {
	k, err := Generate("ed", "ed25519")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.HasPrefix(k.DNSRecord("example.com"), "v=DKIM1; k=ed25519; p=") {
		t.Errorf("dns record malformed: %q", k.DNSRecord("example.com"))
	}
}

func TestGenerateBadAlgorithm(t *testing.T) {
	if _, err := Generate("s", "dsa"); err == nil {
		t.Error("dsa should be rejected")
	}
}

func TestGenerateUniqueKeys(t *testing.T) {
	a, err := Generate("a", "rsa")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate("a", "rsa")
	if err != nil {
		t.Fatal(err)
	}
	if a.PublicKey == b.PublicKey {
		t.Error("two generations produced identical public keys")
	}
}

func TestPlanRotation(t *testing.T) {
	p := PlanRotation("s2025", "s2026")
	if p.OldSelector != "s2025" || p.NewSelector != "s2026" {
		t.Errorf("plan selectors: %+v", p)
	}
	md := p.Markdown()
	if !strings.Contains(md, "s2025 -> s2026") {
		t.Errorf("markdown missing selectors: %q", md)
	}
	// The overlap step is the one that prevents breakage — must be present.
	if !strings.Contains(md, "TTL") {
		t.Error("rotation plan missing the TTL-overlap step")
	}
}
