// Package simulate is the reputation trajectory engine: a deterministic
// function of a user-declared volume plan. Same plan → same curve, every
// time — that determinism is CI-gated on a golden fixture. This is a
// simulation, not a deliverability guarantee.
package simulate

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Plan is the user-declared sending plan the simulation runs over.
type Plan struct {
	VolumeStart   int     `json:"volume_start"`   // day-1 volume (messages)
	VolumeTarget  int     `json:"volume_target"`  // steady-state volume
	RampDays      int     `json:"ramp_days"`      // days to reach target
	Days          int     `json:"days"`           // total simulated days
	BounceRate    float64 `json:"bounce_rate"`    // % (1.0 = 1%)
	ComplaintRate float64 `json:"complaint_rate"` // % (0.05 = 0.05%)
	IPAge         string  `json:"ip_age"`         // "new" or "aged"
}

// RiskBand is the predicted block/defer risk classification for a day.
type RiskBand string

const (
	RiskLow      RiskBand = "low"
	RiskElevated RiskBand = "elevated"
	RiskHigh     RiskBand = "high"
)

// DayPoint is one day of the trajectory.
type DayPoint struct {
	Day       int      `json:"day"`
	Volume    int      `json:"volume"`
	BlockRisk RiskBand `json:"block_risk"`
	DeferRisk RiskBand `json:"defer_risk"`
}

// Result is the full simulated trajectory.
type Result struct {
	Plan       Plan       `json:"plan"`
	Days       int        `json:"days"`
	Trajectory []DayPoint `json:"trajectory"`
	Disclaimer string     `json:"disclaimer"`
}

const disclaimer = "Simulation from declared inputs only. Not a deliverability guarantee. Warmline does not operate IP pools."

// LoadPlan reads a plan JSON file.
func LoadPlan(path string) (*Plan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	p, err := LoadPlanBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return p, nil
}

// LoadPlanBytes parses a plan from raw JSON bytes (embedded fixtures,
// GUI-loaded plans).
func LoadPlanBytes(raw []byte) (*Plan, error) {
	var p Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate rejects nonsense plans with actionable errors.
func (p *Plan) Validate() error {
	switch {
	case p.VolumeStart <= 0:
		return fmt.Errorf("volume_start must be > 0 (got %d)", p.VolumeStart)
	case p.VolumeTarget < p.VolumeStart:
		return fmt.Errorf("volume_target (%d) must be >= volume_start (%d)", p.VolumeTarget, p.VolumeStart)
	case p.Days <= 0:
		return fmt.Errorf("days must be > 0 (got %d)", p.Days)
	case p.RampDays < 0 || p.RampDays > p.Days:
		return fmt.Errorf("ramp_days must be within [0, days] (got %d, days %d)", p.RampDays, p.Days)
	case p.IPAge != "new" && p.IPAge != "aged":
		return fmt.Errorf("ip_age must be \"new\" or \"aged\" (got %q)", p.IPAge)
	case p.BounceRate < 0 || p.ComplaintRate < 0:
		return fmt.Errorf("rates must be >= 0")
	}
	return nil
}

// Run simulates the plan and returns the trajectory. The caller's plan
// is never mutated; RampDays=0 is legal (day 1 jumps straight to
// target) and is reported as declared in Result.Plan.
func Run(plan *Plan) (*Result, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	ramp := plan.RampDays
	if ramp == 0 {
		ramp = 1 // effective ramp: day 1 hits target immediately
	}
	effective := *plan
	effective.RampDays = ramp
	traj := make([]DayPoint, 0, plan.Days)
	for day := 1; day <= plan.Days; day++ {
		vol := volumeOn(&effective, day)
		traj = append(traj, DayPoint{
			Day:       day,
			Volume:    vol,
			BlockRisk: blockRisk(plan.BounceRate, plan.ComplaintRate, plan.IPAge, day, ramp),
			DeferRisk: deferRisk(plan.BounceRate, plan.ComplaintRate, plan.IPAge, day, ramp),
		})
	}
	return &Result{Plan: *plan, Days: plan.Days, Trajectory: traj, Disclaimer: disclaimer}, nil
}

// volumeOn is the linear warmup ramp: day 1 starts at VolumeStart and
// the ramp hits VolumeTarget on day RampDays, then holds. Deterministic,
// integer volumes only.
func volumeOn(p *Plan, day int) int {
	if day >= p.RampDays {
		return p.VolumeTarget
	}
	// Linear interpolation over ramp days, rounded to a whole message.
	prog := float64(day-1) / float64(p.RampDays-1) // day 1 → 0, day ramp → 1
	if p.RampDays == 1 {
		return p.VolumeTarget
	}
	v := float64(p.VolumeStart) + prog*float64(p.VolumeTarget-p.VolumeStart)
	return int(v + 0.5)
}

// blockRisk classifies predicted hard-block probability for a day.
//
// Thresholds distilled from published ISP engagement guidance (Google/
// Yahoo 2024 sender rules: complaint ceiling 0.3%, bounce ceiling 2%)
// plus an IP-reputation prior: a brand-new IP carries an elevated prior
// that decays as the ramp completes without violations. Bands:
//
//	high: complaint >0.3%, bounce >4%, or a new IP in the first half of
//	      its ramp (no accrued reputation yet)
//	elevated: complaint >0.1%, bounce >2%, or a new IP still ramping
//	low: everything else
func blockRisk(bounce, complaint float64, ipAge string, day, rampDays int) RiskBand {
	if ipAge == "new" && day <= rampDays/2 && rampDays >= 4 {
		return RiskHigh
	}
	if complaint > 0.3 || bounce > 4 {
		return RiskHigh
	}
	if complaint > 0.1 || bounce > 2 || (ipAge == "new" && day <= rampDays) {
		return RiskElevated
	}
	return RiskLow
}

// deferRisk classifies predicted 4xx-deferral probability for a day.
// Deferrals run hotter than hard blocks during warmup: throttling is the
// ISP's first tool, so the elevated band widens (bounce >1%, complaint
// >0.1%, any new-IP ramp day).
func deferRisk(bounce, complaint float64, ipAge string, day, rampDays int) RiskBand {
	if ipAge == "new" && day <= rampDays/2 && rampDays >= 4 {
		return RiskHigh
	}
	if bounce > 2 || complaint > 0.3 {
		return RiskHigh
	}
	if bounce > 1 || complaint > 0.1 || (ipAge == "new" && day <= rampDays) {
		return RiskElevated
	}
	return RiskLow
}

// Markdown renders the trajectory as a markdown table for humans.
func Markdown(r *Result) string {
	var b strings.Builder
	b.WriteString("## Reputation trajectory (simulation)\n\n")
	b.WriteString(fmt.Sprintf("Plan: %d → %d msg/day over %d ramp days, %d days total; bounce %.2f%%, complaint %.3f%%, IP: %s\n\n",
		r.Plan.VolumeStart, r.Plan.VolumeTarget, r.Plan.RampDays, r.Plan.Days, r.Plan.BounceRate, r.Plan.ComplaintRate, r.Plan.IPAge))
	b.WriteString("| Day | Volume | Block risk | Defer risk |\n|---|---|---|---|\n")
	for _, d := range r.Trajectory {
		b.WriteString(fmt.Sprintf("| %d | %d | %s | %s |\n", d.Day, d.Volume, d.BlockRisk, d.DeferRisk))
	}
	b.WriteString("\n" + disclaimer + "\n")
	return b.String()
}
