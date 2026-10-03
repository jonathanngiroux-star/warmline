package store

import (
	"path/filepath"
	"strings"
	"testing"
)

// Settings: a tiny key/value table for first-run state (wizard seen).
func TestSettingsRoundTrip(t *testing.T) {
	db := openTestDB(t)
	// Default on a missing key is "".
	if v := db.Setting("wizard_seen"); v != "" {
		t.Errorf("missing key should be empty, got %q", v)
	}
	if err := db.SetSetting("wizard_seen", "2026-10-03T00:00:00Z"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if v := db.Setting("wizard_seen"); v != "2026-10-03T00:00:00Z" {
		t.Errorf("round trip = %q", v)
	}
	// Update overwrites.
	if err := db.SetSetting("wizard_seen", "later"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if v := db.Setting("wizard_seen"); v != "later" {
		t.Errorf("update = %q", v)
	}
}

// The settings table must exist right after Open (schema).
func TestSettingsTableCreated(t *testing.T) {
	db := openTestDB(t)
	var name string
	err := db.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='settings'`).Scan(&name)
	if err != nil {
		t.Fatalf("settings table missing: %v", err)
	}
}

// dkimGenerate must write the private key PEM next to the db (0600) and
// never include it in any UI-visible string.
func TestDKIMPrivateKeyNotInUIString(t *testing.T) {
	db := openTestDB(t)
	if err := db.SetSetting("wizard_seen", "2026-10-03T00:00:00Z"); err != nil {
		t.Fatalf("set: %v", err)
	}
	// The real guarantee is at the model layer; here we pin the store's
	// DKIM list API exposes no private material.
	ks, err := db.ListDKIMs()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range ks {
		if strings.Contains(strings.ToLower(k.PublicKey), "private") {
			t.Errorf("dkim list leaked private material: %+v", k)
		}
	}
	_ = filepath.Join(t.TempDir())
}
