package samples

import "testing"

// The embedded fixtures must be byte-identical to the files CI gates on
// — they are the wizard's demo data and must never drift from the
// committed contract.
func TestEmbeddedFixturesNonEmpty(t *testing.T) {
	if len(SendgridSampleExport) < 100 {
		t.Errorf("sendgrid sample export embedded too small: %d bytes", len(SendgridSampleExport))
	}
	if len(SimulateSamplePlan) < 50 {
		t.Errorf("simulate sample plan embedded too small: %d bytes", len(SimulateSamplePlan))
	}
}
