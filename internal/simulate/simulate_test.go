package simulate

import (
	"encoding/json"
	"os"
	"testing"
)

func TestRunPlanGoldenDeterministic(t *testing.T) {
	plan, err := LoadPlan("../../testdata/simulate/plan.json")
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}
	// Determinism: same input → byte-identical curve.
	a, err := Run(plan)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	b, err := Run(plan)
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if string(aj) != string(bj) {
		t.Fatal("simulation is not deterministic: same input produced different output")
	}

	// Golden fixture: committed curve must match computed curve byte-for-byte.
	golden, err := os.ReadFile("../../testdata/simulate/golden-curve.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var goldenResult, computedResult Result
	if err := json.Unmarshal(golden, &goldenResult); err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	if err := json.Unmarshal(aj, &computedResult); err != nil {
		t.Fatalf("parse computed: %v", err)
	}
	if goldenResult.Days != computedResult.Days ||
		len(goldenResult.Trajectory) != len(computedResult.Trajectory) {
		t.Fatalf("golden shape mismatch: %d days vs %d", goldenResult.Days, computedResult.Days)
	}
	for i := range goldenResult.Trajectory {
		if goldenResult.Trajectory[i] != computedResult.Trajectory[i] {
			t.Fatalf("trajectory drift at day %d: golden %+v vs computed %+v",
				i+1, goldenResult.Trajectory[i], computedResult.Trajectory[i])
		}
	}
}

func TestRunPlanCurveShape(t *testing.T) {
	plan := &Plan{
		VolumeStart:   500,
		VolumeTarget:  50000,
		RampDays:      14,
		Days:          21,
		BounceRate:    1.0,
		ComplaintRate: 0.05,
		IPAge:         "new",
	}
	r, err := Run(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Trajectory) != 21 {
		t.Fatalf("trajectory = %d days, want 21", len(r.Trajectory))
	}
	// Day 1 = start volume.
	if r.Trajectory[0].Volume != 500 {
		t.Errorf("day 1 volume = %d, want 500", r.Trajectory[0].Volume)
	}
	// Ramp reaches target by ramp_days and holds.
	if r.Trajectory[13].Volume != 50000 {
		t.Errorf("day 14 (ramp end) volume = %d, want 50000", r.Trajectory[13].Volume)
	}
	if r.Trajectory[20].Volume != 50000 {
		t.Errorf("day 21 volume = %d, want 50000 (hold)", r.Trajectory[20].Volume)
	}
	// Monotonic non-decreasing ramp.
	for i := 1; i < 14; i++ {
		if r.Trajectory[i].Volume < r.Trajectory[i-1].Volume {
			t.Fatalf("ramp not monotonic at day %d: %d < %d", i+1, r.Trajectory[i].Volume, r.Trajectory[i-1].Volume)
		}
	}
	// Low bounce + low complaint but a brand-new IP must still show
	// elevated risk in the first week.
	if r.Trajectory[0].BlockRisk == RiskLow {
		t.Error("new IP day 1 should not be low block risk")
	}
	// …and settle once the ramp is done and metrics are clean.
	last := r.Trajectory[20]
	if last.BlockRisk != RiskLow {
		t.Errorf("clean metrics post-ramp should be low risk, got %q", last.BlockRisk)
	}
}

func TestRiskBands(t *testing.T) {
	tests := []struct {
		name              string
		bounce, complaint float64
		ipAge             string
		day, rampDays     int
		wantBlock         RiskBand
	}{
		{"clean aged", 0.5, 0.02, "aged", 10, 14, RiskLow},
		{"dirty new early", 6.0, 0.4, "new", 3, 14, RiskHigh},
		{"elevated bounce", 3.0, 0.05, "aged", 10, 14, RiskElevated},
		{"high complaint", 1.0, 0.5, "aged", 10, 14, RiskHigh},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := blockRisk(tc.bounce, tc.complaint, tc.ipAge, tc.day, tc.rampDays)
			if got != tc.wantBlock {
				t.Errorf("blockRisk(%v,%v,%q,%d,%d) = %q, want %q",
					tc.bounce, tc.complaint, tc.ipAge, tc.day, tc.rampDays, got, tc.wantBlock)
			}
		})
	}
}

func TestLoadPlanMissing(t *testing.T) {
	if _, err := LoadPlan("/nonexistent.json"); err == nil {
		t.Fatal("missing plan file should error")
	}
}

func TestPlanValidation(t *testing.T) {
	bad := &Plan{VolumeStart: 0, VolumeTarget: 100, Days: 10}
	if err := bad.Validate(); err == nil {
		t.Error("zero start volume should fail validation")
	}
	bad2 := &Plan{VolumeStart: 1000, VolumeTarget: 100, Days: 10}
	if err := bad2.Validate(); err == nil {
		t.Error("target below start should fail validation")
	}
	bad3 := &Plan{VolumeStart: 100, VolumeTarget: 1000, Days: 0}
	if err := bad3.Validate(); err == nil {
		t.Error("zero days should fail validation")
	}
	bad4 := &Plan{VolumeStart: 100, VolumeTarget: 1000, Days: 10, IPAge: "ancient"}
	if err := bad4.Validate(); err == nil {
		t.Error("unknown ip_age should fail validation")
	}
	good := &Plan{VolumeStart: 100, VolumeTarget: 1000, RampDays: 10, Days: 30, BounceRate: 1, ComplaintRate: 0.05, IPAge: "new"}
	if err := good.Validate(); err != nil {
		t.Errorf("valid plan rejected: %v", err)
	}
}

func TestMarkdownOutput(t *testing.T) {
	plan := &Plan{VolumeStart: 100, VolumeTarget: 1000, RampDays: 5, Days: 7, BounceRate: 1.0, ComplaintRate: 0.05, IPAge: "aged"}
	r, err := Run(plan)
	if err != nil {
		t.Fatal(err)
	}
	md := Markdown(r)
	if len(md) == 0 {
		t.Fatal("markdown output empty")
	}
	// Must carry the simulation-not-a-guarantee label every time.
	if !contains(md, "simulation") {
		t.Error("markdown missing the simulation disclaimer")
	}
	if !contains(md, "Day") {
		t.Error("markdown missing table header")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Run must not mutate the caller's plan (RampDays=0 is legal: jump
// straight to target). The result must report what was declared.
func TestRunDoesNotMutatePlan(t *testing.T) {
	plan := &Plan{VolumeStart: 100, VolumeTarget: 1000, RampDays: 0, Days: 5, BounceRate: 1, ComplaintRate: 0.05, IPAge: "aged"}
	r, err := Run(plan)
	if err != nil {
		t.Fatal(err)
	}
	if plan.RampDays != 0 {
		t.Errorf("Run mutated the caller's plan: RampDays = %d, want 0", plan.RampDays)
	}
	if r.Plan.RampDays != 0 {
		t.Errorf("Result.Plan laundered the declared value: %d, want 0 (report what was declared)", r.Plan.RampDays)
	}
	// day 1 jumps straight to target when ramp is 0
	if r.Trajectory[0].Volume != 1000 {
		t.Errorf("ramp 0 should hit target on day 1, got %d", r.Trajectory[0].Volume)
	}
}

// LoadPlanBytes parses a plan from raw bytes (embedded fixtures, GUI).
func TestLoadPlanBytes(t *testing.T) {
	p, err := LoadPlanBytes([]byte(`{"volume_start":50,"volume_target":500,"ramp_days":5,"days":10,"bounce_rate":1,"complaint_rate":0.05,"ip_age":"new"}`))
	if err != nil {
		t.Fatalf("LoadPlanBytes: %v", err)
	}
	if p.Days != 10 {
		t.Errorf("days = %d", p.Days)
	}
	if _, err := LoadPlanBytes([]byte("{")); err == nil {
		t.Error("malformed plan must error")
	}
	if _, err := LoadPlanBytes([]byte(`{"volume_start":0,"volume_target":1,"ramp_days":1,"days":1,"ip_age":"new"}`)); err == nil {
		t.Error("invalid plan values must error")
	}
}
