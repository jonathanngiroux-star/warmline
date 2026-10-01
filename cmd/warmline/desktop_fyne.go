//go:build fyne

package main

// desktop_fyne.go: `warmline desktop` launches the Fyne GUI. This file
// only compiles with `-tags fyne` + CGO_ENABLED=1 (Fyne needs OpenGL).
// The GUI reuses tuiModel for every operation — one logic layer, two
// renderers, so tui_model_test.go covers the desktop's behavior too.

import (
	"fmt"
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/jonathanngiroux-star/warmline/internal/store"
)

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

// runDesktop builds and shows the Fyne window. Requires a display.
func runDesktop(m *tuiModel) int {
	a := app.NewWithID("io.warmline.desktop")
	w := a.NewWindow("warmline")
	w.Resize(fyne.NewSize(960, 640))

	// Shared log area for command outputs, disabled (read-only).
	log := widget.NewMultiLineEntry()
	log.Disable()
	log.Wrapping = fyne.TextWrapBreak

	// ---- Queue tab ----
	queueLabel := widget.NewLabel("queue")
	queueList := widget.NewList(
		func() int {
			v, err := m.queueView()
			if err != nil {
				return 1
			}
			return len(v.rows)
		},
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			v, err := m.queueView()
			if err != nil {
				o.(*widget.Label).SetText("error: " + err.Error())
				return
			}
			if i < 0 || int(i) >= len(v.rows) {
				return
			}
			o.(*widget.Label).SetText(v.rows[i])
		},
	)
	refreshQueue := func() {
		v, err := m.queueView()
		if err != nil {
			queueLabel.SetText("queue — error: " + err.Error())
			return
		}
		queueLabel.SetText("queue — " + v.summary)
		queueList.Refresh()
	}
	refreshQueue()

	// ---- Bounces tab ----
	bounceLabel := widget.NewLabel("bounces")
	bounceList := widget.NewList(
		func() int {
			v, err := m.bounceView()
			if err != nil {
				return 1
			}
			return len(v.rows)
		},
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			v, err := m.bounceView()
			if err != nil {
				o.(*widget.Label).SetText("error: " + err.Error())
				return
			}
			if i < 0 || int(i) >= len(v.rows) {
				return
			}
			o.(*widget.Label).SetText(v.rows[i])
		},
	)
	refreshBounces := func() {
		v, err := m.bounceView()
		if err != nil {
			bounceLabel.SetText("bounces — error: " + err.Error())
			return
		}
		bounceLabel.SetText("bounces — " + v.summary)
		bounceList.Refresh()
	}
	refreshBounces()

	// ---- DKIM tab ----
	dkimDomain := widget.NewEntry()
	dkimDomain.SetText("example.com")
	dkimSelector := widget.NewEntry()
	dkimSelector.SetText("s2026")
	dkimAlg := widget.NewSelect([]string{"rsa", "ed25519"}, nil)
	dkimAlg.SetSelected("rsa")
	dkimResult := widget.NewMultiLineEntry()
	dkimResult.Disable()
	dkimGenBtn := widget.NewButton("Generate DKIM key", func() {
		rec, err := m.dkimGenerate(dkimDomain.Text, dkimSelector.Text, dkimAlg.Selected)
		if err != nil {
			dkimResult.SetText("error: " + err.Error())
			return
		}
		dkimResult.SetText("Publish this TXT record:\n" + rec +
			"\n\nPrivate key saved next to the db file. Rotation:\nwarmline dkim rotate --old " + dkimSelector.Text + " --new " + dkimSelector.Text + "b")
	})
	dkimForm := widget.NewForm(
		widget.NewFormItem("domain", dkimDomain),
		widget.NewFormItem("selector", dkimSelector),
		widget.NewFormItem("algorithm", dkimAlg),
		widget.NewFormItem("", dkimGenBtn),
	)

	// ---- Migrate tab ----
	migrateSource := widget.NewSelect([]string{"sendgrid", "postmark"}, nil)
	migrateSource.SetSelected("sendgrid")
	migratePath := widget.NewEntry()
	migratePath.SetText("testdata/fixtures/sendgrid/sample-export.json")
	migrateRunBtn := widget.NewButton("Dry-run migrate", func() {
		out, err := m.migrateDryRun(migrateSource.Selected, migratePath.Text)
		if err != nil {
			log.SetText("error: " + err.Error())
			return
		}
		log.SetText(out)
	})
	migrateForm := widget.NewForm(
		widget.NewFormItem("source", migrateSource),
		widget.NewFormItem("export path", migratePath),
		widget.NewFormItem("", migrateRunBtn),
	)

	// ---- Simulate tab ----
	simPlan := widget.NewEntry()
	simPlan.SetText("testdata/simulate/plan.json")
	simRunBtn := widget.NewButton("Simulate reputation", func() {
		out, err := m.simulate(simPlan.Text)
		if err != nil {
			log.SetText("error: " + err.Error())
			return
		}
		log.SetText(out)
	})
	simForm := widget.NewForm(
		widget.NewFormItem("plan path", simPlan),
		widget.NewFormItem("", simRunBtn),
	)

	// ---- Donate tab (always present, per the donation rule) ----
	donateText := widget.NewLabel(m.donate())
	donateText.Wrapping = fyne.TextWrapWord

	// Footer: donation addresses on every tab.
	footer := widget.NewLabel("free forever · donate: ETH/USDC 0x85ee7E71f762d772599cbF1EC20E651B30657521 · BTC bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg")
	footer.Wrapping = fyne.TextTruncate
	footer.TextStyle = fyne.TextStyle{Italic: true}

	tabs := container.NewAppTabs(
		container.NewTabItem("queue", container.NewBorder(queueLabel, nil, nil, nil, queueList)),
		container.NewTabItem("bounces", container.NewBorder(bounceLabel, nil, nil, nil, bounceList)),
		container.NewTabItem("dkim", container.NewVBox(dkimForm, dkimResult)),
		container.NewTabItem("migrate", container.NewVBox(migrateForm, log)),
		container.NewTabItem("simulate", container.NewVBox(simForm, log)),
		container.NewTabItem("donate", container.NewVBox(donateText)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// Refresh data when the queue/bounce tabs are selected.
	tabs.OnChanged = func(t *container.TabItem) {
		switch t.Text {
		case "queue":
			refreshQueue()
		case "bounces":
			refreshBounces()
		}
	}

	w.SetContent(container.NewBorder(nil, footer, nil, nil, tabs))
	w.ShowAndRun()
	return 0
}
