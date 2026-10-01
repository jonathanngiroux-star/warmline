package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var out strings.Builder
	code := run([]string{"version"}, &out, nil)
	if code != 0 {
		t.Fatalf("version exit code = %d, want 0", code)
	}
	got := out.String()
	if !strings.Contains(got, "warmline") {
		t.Errorf("version output missing binary name: %q", got)
	}
	if !strings.Contains(got, "0.1.0") {
		t.Errorf("version output missing semver: %q", got)
	}
}

func TestRunDonate(t *testing.T) {
	var out strings.Builder
	code := run([]string{"donate"}, &out, nil)
	if code != 0 {
		t.Fatalf("donate exit code = %d, want 0", code)
	}
	got := out.String()
	for _, want := range []string{
		"0x85ee7E71f762d772599cbF1EC20E651B30657521",
		"bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("donate output missing address %s:\n%s", want, got)
		}
	}
	// Both addresses must appear EXACTLY — no typo'd or lowercased variant.
	if strings.Count(got, "0x85ee7E71f762d772599cbF1EC20E651B30657521") != 1 {
		t.Errorf("donate output should print the ETH address exactly once:\n%s", got)
	}
}

func TestRunDonateJSON(t *testing.T) {
	// Machine-readable donation info: `warmline donate --format=json` is used
	// by the UI footer and docs tooling.
	var out strings.Builder
	code := run([]string{"donate", "--format=json"}, &out, nil)
	if code != 0 {
		t.Fatalf("donate --format=json exit code = %d, want 0", code)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out.String()), &parsed); err != nil {
		t.Fatalf("donate --format=json output is not valid JSON: %v\n%s", err, out.String())
	}
	if parsed["ethereum"] != "0x85ee7E71f762d772599cbF1EC20E651B30657521" {
		t.Errorf("ethereum field = %v, want exact address", parsed["ethereum"])
	}
	if parsed["bitcoin"] != "bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg" {
		t.Errorf("bitcoin field = %v, want exact address", parsed["bitcoin"])
	}
}
