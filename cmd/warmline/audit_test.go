package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// migratePostmark via the CLI (was 0% coverage): happy path + missing input.
func TestMigratePostmarkCLI(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"migrate", "--from=postmark", "--dry-run", "--format=json",
		"--input", filepath.Join("..", "..", "testdata", "fixtures", "postmark", "sample-export.json")}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errB.String())
	}
	var r map[string]any
	if err := json.Unmarshal([]byte(out.String()), &r); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if r["source"] != "postmark" {
		t.Errorf("source = %v", r["source"])
	}
	if r["input"] == "" {
		t.Error("input path not recorded in report")
	}
	added, _ := r["added"].([]any)
	if len(added) != 2 {
		t.Errorf("added = %d, want 2", len(added))
	}
}

func TestMigratePostmarkCLIMissingInput(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"migrate", "--from=postmark", "--dry-run", "--input", "/nope.json"}, &out, &errB)
	if code == 0 {
		t.Error("missing input must fail")
	}
	if !strings.Contains(errB.String(), "/nope.json") {
		t.Errorf("error must name the path: %q", errB.String())
	}
}

// dkim rotate must render the plan for arbitrary selectors.
func TestDKIMRotateCLI(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"dkim", "rotate", "--old", "s2025", "--new", "s2026"}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errB.String())
	}
	if !strings.Contains(out.String(), "s2025 -> s2026") {
		t.Errorf("plan missing: %q", out.String())
	}
}

// migrate with an unknown --from names the source.
func TestMigrateUnknownFromNamesSource(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"migrate", "--from=mailgun", "--dry-run", "--input", "x.json"}, &out, &errB)
	if code == 0 {
		t.Error("unknown source must fail")
	}
	if !strings.Contains(errB.String(), "mailgun") {
		t.Errorf("must name the source: %q", errB.String())
	}
}
