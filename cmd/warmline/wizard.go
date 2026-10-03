package main

// wizard.go: the guided setup wizard shared by the TUI and the desktop
// GUI. Every step carries the real input fields and runs the real
// operation on the user's values (wizard_interactive.go); the
// renderers (tui.go wizard page, the Wails GUI modal) only draw the
// fields and show the output. The wizard runs once per database (first
// run) and can be re-opened any time from either UI.

// wizardStep is one guided page.
type wizardStep struct {
	ID        string
	Title     string
	Body      string
	Fields    []wizardField
	NextLabel string
}

// wizardSteps returns the fixed step sequence. Order is the wedge:
// understand the tool → migrate → simulate → sign → serve → donate.
func wizardSteps() []wizardStep {
	return []wizardStep{
		{
			ID:    "welcome",
			Title: "Welcome to Warmline",
			Body: "Warmline is a free, self-hosted transactional-email wrapper " +
				"and reputation-simulation harness. It does not send mail for you " +
				"and never will — outbound goes through a relay you supply. " +
				"This wizard walks the core loop once with your own values: " +
				"migrate → simulate → sign → serve.\n\n" +
				"Every step is a real operation on real data, not a slideshow. " +
				"Everything is also a CLI command; the wizard just runs them with you.",
		},
		{
			ID:    "migrate",
			Title: "Step 1 — Dry-run your migration",
			Body: "Point Warmline at your ESP account export and see what actually " +
				"moves, what cannot move, and what will bite you — before you touch " +
				"anything. Warmline never mutates your ESP account; this runs the same " +
				"report as `warmline migrate --dry-run`.\n\n" +
				"CLI equivalent: warmline migrate --from=<source> --dry-run --input <file>",
			Fields: []wizardField{
				{ID: "source", Label: "source ESP", Value: "sendgrid", Kind: "select", Options: []string{"sendgrid", "postmark"},
					Hint: "where you are migrating from"},
				{ID: "input", Label: "export file", Value: "", Kind: "path",
					Hint: "path to your ESP account export JSON"},
			},
			NextLabel: "Run dry-run",
		},
		{
			ID:    "simulate",
			Title: "Step 2 — Simulate your warmup",
			Body: "Declare your sending plan and see the reputation trajectory — " +
				"same plan, same curve, deterministic. This is a simulation from " +
				"your declared numbers only: not a deliverability guarantee, and " +
				"Warmline operates no IP pools.\n\n" +
				"CLI equivalent: warmline simulate --plan plan.json",
			Fields: []wizardField{
				{ID: "volume_start", Label: "day-1 volume (messages)", Value: "500", Kind: "text"},
				{ID: "volume_target", Label: "target volume/day", Value: "50000", Kind: "text"},
				{ID: "ramp_days", Label: "ramp days", Value: "14", Kind: "text"},
				{ID: "days", Label: "total days", Value: "21", Kind: "text"},
				{ID: "bounce_rate", Label: "bounce rate % (1 = 1%)", Value: "1", Kind: "text"},
				{ID: "complaint_rate", Label: "complaint rate % (0.05 = 0.05%)", Value: "0.05", Kind: "text"},
				{ID: "ip_age", Label: "IP age", Value: "new", Kind: "select", Options: []string{"new", "aged"}},
			},
			NextLabel: "Run simulation",
		},
		{
			ID:    "dkim",
			Title: "Step 3 — Sign your mail",
			Body: "Generate a DKIM key pair for YOUR domain and publish the TXT " +
				"record it prints. The private key is saved next to the database " +
				"file (0600) and never shown in any UI. Warmline is not a CA; " +
				"rotation is a checklist (`warmline dkim rotate --old X --new Y`).",
			Fields: []wizardField{
				{ID: "domain", Label: "your domain", Value: "", Kind: "text", Hint: "e.g. example.com"},
				{ID: "selector", Label: "selector", Value: "s2026", Kind: "text"},
				{ID: "algorithm", Label: "algorithm", Value: "rsa", Kind: "select", Options: []string{"rsa", "ed25519"}},
			},
			NextLabel: "Generate key",
		},
		{
			ID:    "serve",
			Title: "Step 4 — Verify your local stack",
			Body: "Warmline will now boot the real SMTP front on your chosen " +
				"address, submit a probe message through a genuine SMTP dialogue, " +
				"and verify it lands in the queue — then clean up. Your real db is " +
				"never touched (throwaway probe db), nothing is sent anywhere (no " +
				"relay is wired).\n\n" +
				"After the probe, run the stack for real:\n" +
				"  warmline serve --smtp <smtp addr> --http <http addr>",
			Fields: []wizardField{
				{ID: "smtp", Label: "SMTP listen address", Value: "127.0.0.1:2525", Kind: "text"},
				{ID: "http", Label: "HTTP UI address", Value: "127.0.0.1:8080", Kind: "text"},
				{ID: "db", Label: "queue database (probe uses a throwaway)", Value: "warmline.db", Kind: "text"},
			},
			NextLabel: "Run probe",
		},
		{
			ID:    "donate",
			Title: "Step 5 — Donate (optional)",
			Body: "Warmline is free to self-host. If it saves you a SendGrid week, donate.\n\n" +
				"Ethereum / USDC (ERC-20): " + donateEthereum + "\n" +
				"Bitcoin:                  " + donateBitcoin + "\n\n" +
				"No feature is gated on donations. Details: docs/donate.md",
		},
	}
}

// wizardSeen reports whether the wizard has been completed on this db.
func (m *tuiModel) wizardSeen() bool {
	return m.db.Setting(settingWizardSeen) != ""
}

// wizardDone marks the wizard complete for this database.
func (m *tuiModel) wizardDone() error {
	return m.db.SetSetting(settingWizardSeen, nowISO())
}

// settingWizardSeen is the settings key for first-run state.
const settingWizardSeen = "wizard_seen"
