package main

// tui_wizard.go: the TUI wizard renderer. tview form per wizard step,
// built from the shared wizardSteps() — zero logic here; Run collects
// the field values and calls wizardRunStep, exactly like the GUI.

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// tuiWizard is the wizard overlay: one page per step over the main UI.
type tuiWizard struct {
	app     *tview.Application
	m       *tuiModel
	pages   *tview.Pages
	steps   []wizardStep
	forms   []*tview.Form     // per-step field form
	outputs []*tview.TextView // per-step result area
	onDone  func()
}

// newTUIWizard builds all wizard pages up front. onDone fires when the
// tour is finished or skipped.
func newTUIWizard(app *tview.Application, m *tuiModel, onDone func()) *tuiWizard {
	w := &tuiWizard{
		app:    app,
		m:      m,
		pages:  tview.NewPages(),
		steps:  wizardSteps(),
		onDone: onDone,
	}
	for i := range w.steps {
		page, form, output := w.buildPage(i)
		w.forms = append(w.forms, form)
		w.outputs = append(w.outputs, output)
		w.pages.AddPage(w.steps[i].ID, page, true, i == 0)
	}
	return w
}

// buildPage renders one step: body text, fields form (which also holds
// the navigation buttons — tview Forms give native Tab/Enter traversal,
// so the keyboard path works with zero custom focus code), and output.
// Returns the page plus direct refs to form and output.
func (w *tuiWizard) buildPage(i int) (tview.Primitive, *tview.Form, *tview.TextView) {
	step := w.steps[i]

	body := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	body.SetText(step.Body)
	body.SetBorder(true).SetTitle(" " + step.Title + " ")

	output := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	output.SetBorder(true).SetTitle(" result ")

	// One form per page: fields first, then Run (if any), then
	// navigation. Enter on any input field runs the step directly —
	// the keyboard path never depends on tab counts.
	form := tview.NewForm().
		SetCancelFunc(func() { w.finish(false) })
	for _, f := range step.Fields {
		if f.Kind == "select" {
			form.AddDropDown(f.Label, f.Options, 0, nil)
		} else {
			form.AddInputField(f.Label, f.Value, 40, nil, nil)
			// Enter in the field = run the step (Tab still moves focus).
			idx := form.GetFormItemCount() - 1
			form.GetFormItem(idx).(*tview.InputField).SetDoneFunc(func(key tcell.Key) {
				if key == tcell.KeyEnter {
					w.run(i)
				}
			})
		}
	}
	if len(step.Fields) > 0 {
		form.AddButton(step.NextLabel, func() { w.run(i) })
	}
	if i > 0 {
		form.AddButton("Prev", func() {
			w.pages.SwitchToPage(w.steps[i-1].ID)
			w.app.SetFocus(w.forms[i-1])
		})
	}
	nextLabel := "Next"
	if i == len(w.steps)-1 {
		nextLabel = "Finish"
	}
	form.AddButton(nextLabel, func() {
		if i+1 < len(w.steps) {
			w.pages.SwitchToPage(w.steps[i+1].ID)
			w.app.SetFocus(w.forms[i+1])
		} else {
			w.finish(true)
		}
	})
	form.AddButton("Skip tour", func() { w.finish(false) })
	form.SetBorder(true).SetTitle(" fields ")

	page := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(body, 0, 2, false).
		AddItem(form, 0, 3, true).
		AddItem(output, 0, 3, false)
	return page, form, output
}

// run collects the form's field values and executes the step's real
// operation, rendering the result (or error) in the output area.
func (w *tuiWizard) run(i int) {
	step := w.steps[i]
	form := w.forms[i]
	values := fieldValues{}
	for fi, f := range step.Fields {
		if f.Kind == "select" {
			_, val := form.GetFormItem(fi).(*tview.DropDown).GetCurrentOption()
			values[f.ID] = val
		} else {
			values[f.ID] = form.GetFormItem(fi).(*tview.InputField).GetText()
		}
	}
	out, err := wizardRunStep(w.m, step.ID, values)
	ot := w.outputs[i]
	if err != nil {
		ot.SetText("[red]" + err.Error())
	} else {
		ot.SetText("[white]" + out)
	}
}

// finish closes the wizard; completing the tour marks it seen.
func (w *tuiWizard) finish(completed bool) {
	if completed {
		_ = w.m.wizardDone()
	}
	w.onDone()
}
