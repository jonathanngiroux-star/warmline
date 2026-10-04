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

	// Tab content builders. Each returns the primary view for a tab
	// plus its refill fn (lists must re-read on every entry so data
	// stays current after actions).
	newList := func(fetch func() (view, error)) (*tview.List, func()) {
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
		return l, fill
	}

	// Forms are declared before the lists so button closures can
	// reference them without order issues.
	var dkimForm *tview.Form
	var migrateForm *tview.Form
	var simForm *tview.Form

	queueList, refillQueue := newList(m.queueView)
	bounceList, refillBounces := newList(m.bounceView)
	dkimList, refillDKIMs := newList(m.dkimView)

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
			refillDKIMs()
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
	// Active page tracking: the nav sets it; the e/b copy hotkeys check
	// it (focus alone is wrong — after clicking "donate" in the nav,
	// focus stays on the nav list, and the hotkeys must still work).
	activePage := "queue"
	donateText := tview.NewTextView().SetText(m.donate()).SetDynamicColors(true)
	donateStatus := tview.NewTextView().SetDynamicColors(true)
	setDonateHint := func() {
		donateStatus.SetText("[gray]e = copy ETH/USDC address · b = copy Bitcoin address · q = quit[-]")
	}
	setDonateHint()
	donatePage := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(donateText, 0, 1, false).
		AddItem(donateStatus, 1, 0, false)
	pages.AddPage("donate", donatePage, true, false)

	// copyDonate writes the address to the clipboard and reports the
	// result on the donate status line.
	copyDonate := func(addr string) {
		if err := copyAddress(addr); err != nil {
			donateStatus.SetText("[red]" + err.Error())
			return
		}
		donateStatus.SetText("[green]copied:[white] " + addr)
	}

	// Left nav: sections switch pages; every section's list refills on
	// entry so data stays current. The wizard is the last entry.
	nav := tview.NewList().ShowSecondaryText(false)
	sections := []struct {
		name   string
		page   string
		refill func()
	}{
		{"queue", "queue", refillQueue},
		{"bounces", "bounces", refillBounces},
		{"dkim", "dkim", refillDKIMs},
		{"migrate", "migrate", func() {}},
		{"simulate", "simulate", func() {}},
		{"donate", "donate", func() {}},
	}

	// The wizard overlays the whole UI; finishing restores the main root.
	root := tview.NewFlex().
		AddItem(nav, 14, 0, true).
		AddItem(pages, 0, 1, false)

	openWizard := func() {
		wiz := newTUIWizard(app, m, func() { app.SetRoot(root, true).EnableMouse(true) })
		app.SetRoot(wiz.pages, true)
	}
	nav.AddItem("wizard", "", 0, openWizard)
	for _, sec := range sections {
		sec := sec
		nav.AddItem(sec.name, "", 0, func() {
			pages.SwitchToPage(sec.page)
			activePage = sec.page
			sec.refill()
		})
	}

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'q', 'Q':
				app.Stop()
				return nil
			case 'w', 'W':
				openWizard()
				return nil
			case 'e', 'E':
				// only on the donate page
				if activePage == "donate" {
					copyDonate(donateEthereum)
					return nil
				}
			case 'b', 'B':
				if activePage == "donate" {
					copyDonate(donateBitcoin)
					return nil
				}
			}
		}
		return event
	})

	// First run on a fresh db: open the wizard automatically (the same
	// behavior the desktop GUI has).
	if !m.wizardSeen() {
		go func() {
			app.QueueUpdateDraw(func() {
				wiz := newTUIWizard(app, m, func() { app.SetRoot(root, true).EnableMouse(true) })
				app.SetRoot(wiz.pages, true)
			})
		}()
	}

	if err := app.SetRoot(root, true).EnableMouse(true).Run(); err != nil {
		return 1
	}
	return 0
}
