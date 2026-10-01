package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// runMigrate executes the migrate command in-process and returns
// (exit code, stdout, stderr).
func runMigrate(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errB strings.Builder
	code := run(append([]string{"migrate"}, args...), &out, &errB)
	return code, out.String(), errB.String()
}

var fixturePath = filepath.Join("..", "..", "testdata", "fixtures", "sendgrid", "sample-export.json")

func TestMigrateSendgridDryRunJSON(t *testing.T) {
	code, out, stderr := runMigrate(t,
		"--from=sendgrid",
		"--dry-run",
		"--format=json",
		"--input", fixturePath,
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0. stderr: %s", code, stderr)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	// Diff keys required by the brief.
	for _, key := range []string{"added", "removed", "changed", "unmapped", "risks"} {
		v, ok := report[key]
		if !ok {
			t.Errorf("report missing key %q", key)
			continue
		}
		if _, isSlice := v.([]any); !isSlice {
			t.Errorf("report key %q is not an array: %T", key, v)
		}
	}
	// The fixture has exactly one authenticated domain → one added entry.
	added, _ := report["added"].([]any)
	if len(added) != 1 {
		t.Fatalf("added = %d entries, want 1 (one authenticated domain)\n%s", len(added), out)
	}
	first, _ := added[0].(map[string]any)
	if first["key"] != "domain.mail.example.com" {
		t.Errorf("added[0].key = %v, want domain.mail.example.com", first["key"])
	}

	// v0.1 webhook mapping contract: everything maps to a canonical event
	// except `dropped` and `group_resubscribe`. Extending the mapping table
	// must update this assertion consciously — that is the point.
	unmapped, _ := report["unmapped"].([]any)
	if len(unmapped) != 2 {
		t.Fatalf("unmapped = %d entries, want 2 (dropped, group_resubscribe)\n%s", len(unmapped), out)
	}
	u0, _ := unmapped[0].(map[string]any)
	if u0["key"] != "webhook.event.dropped" {
		t.Errorf("unmapped[0].key = %v, want webhook.event.dropped", u0["key"])
	}
	u1, _ := unmapped[1].(map[string]any)
	if u1["key"] != "webhook.event.group_resubscribe" {
		t.Errorf("unmapped[1].key = %v, want webhook.event.group_resubscribe", u1["key"])
	}

	// Unmappable-but-real source concepts must surface as risks, not
	// silently vanish: IP pools (Warmline simulates, never operates),
	// suppression lists (live behind the ESP API), legacy templates.
	risks, _ := report["risks"].([]any)
	riskKeys := map[string]bool{}
	for _, r := range risks {
		if m, ok := r.(map[string]any); ok {
			riskKeys[m["key"].(string)] = true
		}
	}
	for _, want := range []string{"ip_pools", "suppression", "templates.legacy", "webhook.oauth"} {
		if !riskKeys[want] {
			t.Errorf("risks missing %q (got %v)", want, riskKeys)
		}
	}

	// reputation_sim placeholder must be present and structured.
	sim, ok := report["reputation_sim"].(map[string]any)
	if !ok {
		t.Fatalf("reputation_sim is not an object: %T\n%s", report["reputation_sim"], out)
	}
	if sim["status"] != "planned" {
		t.Errorf("reputation_sim.status = %v, want \"planned\"", sim["status"])
	}
	// A stub must be honest that it is a stub.
	if _, ok := sim["note"]; !ok {
		t.Error("reputation_sim missing note")
	}
}

func TestMigrateUnknownSource(t *testing.T) {
	code, _, stderr := runMigrate(t, "--from=mailgun", "--dry-run", "--input", fixturePath)
	if code == 0 {
		t.Error("unknown --from should not exit 0")
	}
	// Distinguishing detail: the refusal must name the unsupported source —
	// not just any failure (the not-implemented fallback also exits 2).
	if !strings.Contains(stderr, "mailgun") {
		t.Errorf("refusal should name the source 'mailgun': %q", stderr)
	}
}

func TestMigrateMissingInput(t *testing.T) {
	code, _, stderr := runMigrate(t, "--from=sendgrid", "--dry-run", "--input", "/nonexistent/export.json")
	if code == 0 {
		t.Error("missing input file should not exit 0")
	}
	// Distinguishing detail: the error must mention the path — a generic
	// refusal would let a real open failure hide behind the fallback.
	if !strings.Contains(stderr, "/nonexistent/export.json") {
		t.Errorf("error should mention the input path: %q", stderr)
	}
}

func TestMigrateRequiresDryRun(t *testing.T) {
	// Warmline never mutates a real ESP account: migrate without --dry-run
	// must refuse loudly until a real apply path exists (and even then it
	// stays a local config writer, never a live API caller).
	code, out, stderr := runMigrate(t, "--from=sendgrid", "--input", fixturePath)
	if code == 0 {
		t.Error("migrate without --dry-run should not exit 0")
	}
	combined := out + stderr
	if !strings.Contains(combined, "dry-run") {
		t.Errorf("refusal message should mention dry-run: %q", combined)
	}
}
