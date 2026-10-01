package main

// tui.go: the terminal UI. tview/tcell is pure Go (no cgo), so the TUI
// ships in the default cgo-free binary. All logic lives on tuiModel;
// this file only renders.

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// runTUI builds and runs the terminal UI. Requires a TTY (checked by
// the launcher, not here).
func runTUI(m *tuiModel) int {
	app := tview.NewApplication()

	// Shared output area for command results (migrate/simulate/donate).
	output := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	output.SetBorder(true).SetTitle(" output ")

	// Tab content builders. Each returns the primary view for a tab.
	newList := func(fetch func() (view, error)) *tview.List {
		l := tview.NewList().ShowSecondaryText(false)
		fill := func() {
			l.Clear()
			v, err := fetch()
			if err != nil {
				l.AddItem("error: "+err.Error(), "", 0, nil)
				return
			}
			l.AddItem("[::b]"+v.summary, "", 0, nil)
			for _, row := range v.rows {
				l.AddItem(row, "", 0, nil)
			}
		}
		fill()
		return l
	}

	// Forms are declared before the lists so button closures can
	// reference them without order issues.
	var dkimForm *tview.Form
	var migrateForm *tview.Form
	var simForm *tview.Form

	queueList := newList(m.queueView)
	bounceList := newList(m.bounceView)
	dkimList := newList(m.dkimView)

	// DKIM actions: generate a selector (prompt domain/selector/alg).
	dkimForm = tview.NewForm().
		AddInputField("domain", "example.com", 20, nil, nil).
		AddInputField("selector", "s2026", 10, nil, nil).
		AddDropDown("algorithm", []string{"rsa", "ed25519"}, 0, nil).
		AddButton("Generate", func() {
			domain := dkimForm.GetFormItem(0).(*tview.InputField).GetText()
			selector := dkimForm.GetFormItem(1).(*tview.InputField).GetText()
			_, alg := dkimForm.GetFormItem(2).(*tview.DropDown).GetCurrentOption()
			rec, err := m.dkimGenerate(domain, selector, alg)
			if err != nil {
				output.SetText("[red]" + err.Error())
			} else {
				output.SetText("[green]Publish this TXT record:\n[white]" + rec +
					"\n\nPrivate key saved next to the db file. Rotation: warmline dkim rotate --old " + selector + " --new " + selector + "b")
			}
		})
	dkimForm.SetBorder(true).SetTitle(" dkim generate ")

	// Migrate tab: source + fixture path -> report text.
	migrateForm = tview.NewForm().
		AddDropDown("source", []string{"sendgrid", "postmark"}, 0, nil).
		AddInputField("export path", "", 40, nil, nil).
		AddButton("Dry-run", func() {
			_, source := migrateForm.GetFormItem(0).(*tview.DropDown).GetCurrentOption()
			path := migrateForm.GetFormItem(1).(*tview.InputField).GetText()
			out, err := m.migrateDryRun(source, path)
			if err != nil {
				output.SetText("[red]" + err.Error())
			} else {
				output.SetText("[white]" + out)
			}
		})
	migrateForm.SetBorder(true).SetTitle(" migrate dry-run ")

	// Simulate tab: plan path -> markdown curve.
	simForm = tview.NewForm().
		AddInputField("plan path", "", 40, nil, nil).
		AddButton("Simulate", func() {
			path := simForm.GetFormItem(0).(*tview.InputField).GetText()
			out, err := m.simulate(path)
			if err != nil {
				output.SetText("[red]" + err.Error())
			} else {
				output.SetText("[white]" + out)
			}
		})
	simForm.SetBorder(true).SetTitle(" reputation simulation ")

	// Donate footer — visible on every tab, per the donation rule.
	donateFooter := tview.NewTextView().SetDynamicColors(true)
	donateFooter.SetText("[gray]free forever · donate: ETH/USDC 0x85ee7E71f762d772599cbF1EC20E651B30657521 · BTC bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg[-]")

	pages := tview.NewPages()
	pages.AddPage("queue", queueList, true, true)
	pages.AddPage("bounces", bounceList, true, false)
	pages.AddPage("dkim", tview.NewFlex().
		AddItem(dkimList, 0, 1, false).
		AddItem(dkimForm, 0, 1, false), true, false)
	pages.AddPage("migrate", tview.NewFlex().
		AddItem(migrateForm, 0, 1, false).
		AddItem(output, 0, 2, false), true, false)
	pages.AddPage("simulate", tview.NewFlex().
		AddItem(simForm, 0, 1, false).
		AddItem(output, 0, 2, false), true, false)
	pages.AddPage("donate", tview.NewTextView().SetText(m.donate()), true, false)

	// Left nav: sections switch pages; every section's list refills on
	// entry so data stays current.
	nav := tview.NewList().ShowSecondaryText(false)
	sections := []struct {
		name   string
		page   string
		refill func()
	}{
		{"queue", "queue", func() {}},
		{"bounces", "bounces", func() {}},
		{"dkim", "dkim", func() {}},
		{"migrate", "migrate", func() {}},
		{"simulate", "simulate", func() {}},
		{"donate", "donate", func() {}},
	}
	for _, sec := range sections {
		sec := sec
		nav.AddItem(sec.name, "", 0, func() {
			pages.SwitchToPage(sec.page)
		})
	}

	root := tview.NewFlex().
		AddItem(nav, 14, 0, true).
		AddItem(pages, 0, 1, false)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'q', 'Q':
				app.Stop()
				return nil
			}
		}
		return event
	})

	if err := app.SetRoot(root, true).EnableMouse(true).Run(); err != nil {
		return 1
	}
	return 0
}
