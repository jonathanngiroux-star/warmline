package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	samples "github.com/jonathanngiroux-star/warmline"
	"github.com/jonathanngiroux-star/warmline/internal/migrate/sendgrid"
)

// The sample-export affordance: users without an ESP export yet can
// still run the migrate step. It materializes the embedded fixture to
// a temp file and hands back a path the field accepts.
func TestSampleExportTempFile(t *testing.T) {
	t.Chdir(t.TempDir()) // outside any checkout
	path, err := sampleExportPath()
	if err != nil {
		t.Fatalf("sampleExportPath: %v", err)
	}
	if path == "" {
		t.Fatal("path empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	exp, err := sendgrid.ParseBytes(raw)
	if err != nil {
		t.Fatalf("sample fixture must parse: %v", err)
	}
	if _, err := exp.BuildReport(); err != nil {
		t.Fatalf("sample fixture must report: %v", err)
	}
}

// The embedded fixture must be byte-identical to the committed CI
// fixture (no drift).
func TestEmbeddedFixtureMatchesCommitted(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "sendgrid", "sample-export.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(samples.SendgridSampleExport) {
		t.Error("embedded sample fixture drifted from testdata/")
	}
}

// The wizard's migrate step runs on the sample path end to end,
// outside any source checkout.
func TestWizardMigrateOnSample(t *testing.T) {
	m := openModel(t)
	t.Chdir(t.TempDir())
	path, err := sampleExportPath()
	if err != nil {
		t.Fatal(err)
	}
	out, err := wizardRunStep(m, "migrate", fieldValues{"source": "sendgrid", "input": path})
	if err != nil {
		t.Fatalf("migrate on sample: %v", err)
	}
	if !strings.Contains(out, "unmapped (2)") {
		t.Errorf("migrate output missing unmapped section: %.200s", out)
	}
}
