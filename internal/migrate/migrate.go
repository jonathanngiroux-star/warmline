// Package migrate defines the shared migration-report shape every ESP
// source package produces. Same schema for sendgrid/postmark/mailgun —
// only the mapper differs.
package migrate

// Report is the machine-readable dry-run diff printed by
// `warmline migrate --from=<esp> --dry-run --format=json`.
type Report struct {
	Source        string        `json:"source"`
	DryRun        bool          `json:"dry_run"`
	Input         string        `json:"input"`
	Added         []Entry       `json:"added"`
	Removed       []Entry       `json:"removed"`
	Changed       []Entry       `json:"changed"`
	Unmapped      []Entry       `json:"unmapped"`
	Risks         []Entry       `json:"risks"`
	ReputationSim ReputationSim `json:"reputation_sim"`
}

// Entry is one line of the diff.
type Entry struct {
	Key  string `json:"key"`
	From any    `json:"from,omitempty"`
	To   any    `json:"to,omitempty"`
	Note string `json:"note,omitempty"`
}

// ReputationSim is the placeholder wired into every report until the
// trajectory engine (W5–7) lands. It must always be honest that it is
// not yet a simulation result.
type ReputationSim struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}
