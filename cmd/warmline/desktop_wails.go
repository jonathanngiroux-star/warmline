//go:build wails

package main

// desktop_wails.go: `warmline desktop` launches the Wails v2 desktop
// GUI. This file only compiles with `-tags wails` (webkit2gtk on Linux,
// WebView2 on Windows). The GUI is a thin renderer over tuiModel — one
// logic layer, three thin renderers (CLI, TUI, GUI); the frontend is
// vanilla JS embedded in the binary: no npm tree, no bundler.

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/jonathanngiroux-star/warmline/internal/store"
)

//go:embed frontend/dist
var desktopAssets embed.FS

// launchDesktop parses flags and hands off to the Wails app.
func launchDesktop(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("desktop", stderr)
	db := fs.String("db", envDefault("WARMLINE_DB", "warmline.db"), "path to the SQLite queue database")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	st, err := store.Open(*db)
	if err != nil {
		fmt.Fprintf(stderr, "desktop: %v\n", err)
		return 1
	}
	defer st.Close()
	return runDesktop(newTUIModel(st))
}

// DesktopApp is the struct bound to the frontend. Every method is a
// thin wrapper over tuiModel — no logic lives here.
type DesktopApp struct {
	ctx context.Context
	m   *tuiModel
}

// viewData is the JSON the frontend renders for list surfaces.
type viewData struct {
	Summary string   `json:"summary"`
	Rows    []string `json:"rows"`
}

// Queue returns the queue view (summary + rows).
func (a *DesktopApp) Queue() (viewData, error) {
	v, err := a.m.queueView()
	return toViewData(v, err)
}

// Bounces returns the classified-bounce view.
func (a *DesktopApp) Bounces() (viewData, error) {
	v, err := a.m.bounceView()
	return toViewData(v, err)
}

// DKIMs returns the recorded selector view.
func (a *DesktopApp) DKIMs() (viewData, error) {
	v, err := a.m.dkimView()
	return toViewData(v, err)
}

func toViewData(v view, err error) (viewData, error) {
	if err != nil {
		return viewData{}, err
	}
	return viewData{Summary: v.summary, Rows: v.rows}, nil
}

// Migrate runs a dry-run migrate report. source: sendgrid|postmark.
func (a *DesktopApp) Migrate(source, input string) (string, error) {
	return a.m.migrateDryRun(source, input)
}

// Simulate runs the reputation trajectory over a plan file.
func (a *DesktopApp) Simulate(planPath string) (string, error) {
	return a.m.simulate(planPath)
}

// DKIMGenerate creates a keypair and returns the DNS record to publish.
// The private key never crosses this boundary.
func (a *DesktopApp) DKIMGenerate(domain, selector, algorithm string) (string, error) {
	return a.m.dkimGenerate(domain, selector, normalizeAlgorithm(algorithm))
}

// normalizeAlgorithm maps the frontend's choice onto the dkim package
// values; anything unexpected falls back to rsa (the CLI default).
func normalizeAlgorithm(algorithm string) string {
	if algorithm == "ed25519" {
		return "ed25519"
	}
	return "rsa"
}

// Donate returns the donate text (both addresses).
func (a *DesktopApp) Donate() string { return a.m.donate() }

// WizardStepDTO is one wizard page for the frontend.
type WizardStepDTO struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Body      string        `json:"body"`
	Fields    []wizardField `json:"fields"`
	NextLabel string        `json:"nextLabel"`
}

// WizardSteps returns the guided-setup sequence.
func (a *DesktopApp) WizardSteps() []WizardStepDTO {
	steps := wizardSteps()
	out := make([]WizardStepDTO, 0, len(steps))
	for _, s := range steps {
		out = append(out, WizardStepDTO{
			ID: s.ID, Title: s.Title, Body: s.Body,
			Fields: s.Fields, NextLabel: s.NextLabel,
		})
	}
	return out
}

// WizardRunStep runs a step's real operation on the user's values.
func (a *DesktopApp) WizardRunStep(stepID string, values map[string]string) (string, error) {
	return wizardRunStep(a.m, stepID, values)
}

// SampleExportPath materializes the embedded sample export for users
// without an ESP export yet ("try the sample first" button).
func (a *DesktopApp) SampleExportPath() (string, error) {
	return sampleExportPath()
}

// WizardSeen reports first-run state; WizardDone marks it complete.
func (a *DesktopApp) WizardSeen() bool  { return a.m.wizardSeen() }
func (a *DesktopApp) WizardDone() error { return a.m.wizardDone() }

// Version returns the binary version string.
func (a *DesktopApp) Version() string { return version }

// CopyAddress copies text to the system clipboard (the donate copy
// buttons). Uses Wails' clipboard runtime; if it fails the frontend
// falls back to navigator.clipboard / a manual-select hint.
func (a *DesktopApp) CopyAddress(text string) (string, error) {
	if err := wruntime.ClipboardSetText(a.ctx, text); err != nil {
		return "", err
	}
	return "copied", nil
}

// PickExportPath opens a native file dialog for the ESP export JSON.
func (a *DesktopApp) PickExportPath() (string, error) {
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Choose your ESP export JSON",
		Filters: []wruntime.FileFilter{
			{DisplayName: "ESP export (*.json)", Pattern: "*.json"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
}

// PickPlanPath opens a native file dialog for the simulate plan JSON.
func (a *DesktopApp) PickPlanPath() (string, error) {
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Choose a plan JSON",
		Filters: []wruntime.FileFilter{
			{DisplayName: "Plan (*.json)", Pattern: "*.json"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
}

// runDesktop builds and shows the Wails window. Requires a display.
func runDesktop(m *tuiModel) int {
	app := &DesktopApp{m: m}
	err := wails.Run(&options.App{
		Title:     "warmline",
		Width:     960,
		Height:    640,
		MinWidth:  640,
		MinHeight: 420,
		AssetServer: &assetserver.Options{
			Assets: desktopAssets,
		},
		OnStartup: func(ctx context.Context) {
			app.ctx = ctx
			app.startTestBridge()
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "desktop: %v\n", err)
		return 1
	}
	return 0
}
