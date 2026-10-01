package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSimulateFromPlanJSON(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"simulate", "--plan", "../../testdata/simulate/plan.json", "--format=json"}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errB.String())
	}
	var r map[string]any
	if err := json.Unmarshal([]byte(out.String()), &r); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if r["disclaimer"] == nil || !strings.Contains(out.String(), "Not a deliverability guarantee") {
		t.Error("output must carry the simulation disclaimer")
	}
	traj, ok := r["trajectory"].([]any)
	if !ok || len(traj) != 21 {
		t.Errorf("trajectory = %v, want 21 points", traj)
	}
}

func TestSimulateMarkdown(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"simulate", "--plan", "../../testdata/simulate/plan.json"}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errB.String())
	}
	if !strings.Contains(out.String(), "Day") || !strings.Contains(out.String(), "simulation") {
		t.Errorf("markdown missing table or disclaimer:\n%s", out.String())
	}
}

func TestSimulateMissingPlan(t *testing.T) {
	var out, errB strings.Builder
	code := run([]string{"simulate", "--plan", "/nonexistent.json"}, &out, &errB)
	if code == 0 {
		t.Error("missing plan should fail")
	}
	if !strings.Contains(errB.String(), "/nonexistent.json") {
		t.Errorf("error should name the path: %q", errB.String())
	}
}
