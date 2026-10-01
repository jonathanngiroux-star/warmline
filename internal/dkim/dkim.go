package dkim

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
)

// Key is a generated DKIM keypair material for one selector.
type Key struct {
	Selector  string
	Algorithm string // "rsa" or "ed25519"
	// PublicKey is the value for the DNS TXT record (p= tag), base64.
	PublicKey string
	// PrivateKeyPEM is the signing key, PEM PKCS8. Never leaves the host.
	PrivateKeyPEM string
}

// Generate creates a new DKIM keypair for a selector.
// rsa-2048 for maximum receiver compatibility; ed25519 for small DNS
// records where the receiver supports RFC 8463.
func Generate(selector, algorithm string) (*Key, error) {
	switch algorithm {
	case "rsa":
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("rsa generate: %w", err)
		}
		der, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			return nil, err
		}
		pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
		pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		if err != nil {
			return nil, err
		}
		return &Key{
			Selector:      selector,
			Algorithm:     "rsa",
			PublicKey:     base64.StdEncoding.EncodeToString(pubDER),
			PrivateKeyPEM: pemStr,
		}, nil
	case "ed25519":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		der, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			return nil, err
		}
		pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
		return &Key{
			Selector:      selector,
			Algorithm:     "ed25519",
			PublicKey:     base64.StdEncoding.EncodeToString(pub),
			PrivateKeyPEM: pemStr,
		}, nil
	default:
		return nil, fmt.Errorf("algorithm must be rsa or ed25519 (got %q)", algorithm)
	}
}

// DNSRecord renders the TXT record value for the selector at domain.
func (k *Key) DNSRecord(domain string) string {
	return fmt.Sprintf("v=DKIM1; k=%s; p=%s", k.Algorithm, k.PublicKey)
}

// DNSHost renders the record host name (selector._domainkey.domain).
func (k *Key) DNSHost(domain string) string {
	return fmt.Sprintf("%s._domainkey.%s", k.Selector, domain)
}

// RotationPlan is the documented rotation sequence. Overlap is the whole
// game: publish the new selector, sign with both until TTL expiry, then
// drop the old. Warmline generates config; the operator publishes DNS
// and controls signing — Warmline is not the CA and never sees your
// DNS provider.
type RotationPlan struct {
	OldSelector string
	NewSelector string
	Steps       []string
}

// PlanRotation returns the step-by-step rotation checklist for moving
// from old to new selector.
func PlanRotation(old, new string) *RotationPlan {
	return &RotationPlan{
		OldSelector: old,
		NewSelector: new,
		Steps: []string{
			"1. Generate the new key (warmline dkim generate --selector " + new + ")",
			"2. Publish the new TXT record at " + new + "._domainkey.<domain> alongside the old one",
			"3. Wait for DNS propagation (check with dig TXT " + new + "._domainkey.<domain>)",
			"4. Switch signing to the new selector; keep the old record published",
			"5. Send test mail to a mailbox you control and verify the dkim=pass header names the new selector",
			"6. After >= the old record's TTL (24h minimum), delete the old TXT record",
			"7. Store the new private key where your signer reads it (600 perms, off repo)",
		},
	}
}

// Markdown renders the plan for docs/output.
func (r *RotationPlan) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## DKIM rotation: %s -> %s\n\n", r.OldSelector, r.NewSelector)
	for _, s := range r.Steps {
		fmt.Fprintf(&b, "- %s\n", s)
	}
	b.WriteString("\nWarmline generates keys and config snippets; you publish DNS and control the signer. Warmline is not a CA.\n")
	return b.String()
}
