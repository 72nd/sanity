package ui

import (
	"context"
	"io"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/theme"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/fine/widgets/grid"
	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/ui/presenter"
)

// importDialog is the review dialog of "Import type…". The view holds no
// logic: the presenter parses, checks and stores.
type importDialog struct {
	sh     *shell.Shell
	p      *presenter.Types
	onDone func(id string, andEdit bool)

	snippet    *widgets.Entry
	samples    *widgets.Label
	privacy    *fyne.Container
	findings   *fyne.Container
	extra      *fyne.Container
	rules      *grid.Grid[presenter.ImportRuleRow]
	rulesBox   fyne.CanvasObject
	example    *widgets.Label
	dry        *grid.Grid[presenter.DryRunRow]
	dryBox     fyne.CanvasObject
	dryHead    fyne.CanvasObject
	collision  *widgets.RadioGroup
	collRow    fyne.CanvasObject
	checkBtn   *widgets.Button
	canSave    bool
	results    fyne.CanvasObject
	content    fyne.CanvasObject
	dlg        widgets.Dialog
	minWidth   float32
	privSerial int
}

// sizeLayout gives its child a fixed minimum height. The width is left to the
// dialog (widgets.DialogOptions.MinWidth), so the content follows its width.
type sizeLayout struct{ h float32 }

func (l sizeLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, l.h) }
func (l sizeLayout) Layout(o []fyne.CanvasObject, s fyne.Size) {
	for _, c := range o {
		c.Move(fyne.NewPos(0, 0))
		c.Resize(s)
	}
}

func statusOf(l analysis.Level) (theme.Status, string) {
	switch l {
	case analysis.LevelError:
		return theme.StatusDanger, i18n.T("types.import.level.error")
	case analysis.LevelWarning:
		return theme.StatusWarning, i18n.T("types.import.level.warning")
	}
	return theme.StatusSuccess, i18n.T("types.import.level.ok")
}

// newImportDialog builds the dialog content; show opens it.
func newImportDialog(sh *shell.Shell, p *presenter.Types, onDone func(id string, andEdit bool)) *importDialog {
	d := &importDialog{sh: sh, p: p, onDone: onDone}
	p.ImportReset()

	d.snippet = widgets.NewTextArea()
	d.snippet.SetMinRowsVisible(8)
	d.snippet.SetPlaceHolder(i18n.T("types.import.snippet.placeholder"))
	d.snippet.OnChanged = func(s string) {
		p.SetSnippet(s)
		d.reset()
		d.refreshPrivacy()
	}
	paste := widgets.NewButton(i18n.T("types.import.paste"), widgets.ButtonSecondary, func() {
		d.snippet.SetText(sh.App.Clipboard().Content())
	})
	addSample := widgets.NewButton(i18n.T("types.import.sample.add"), widgets.ButtonSecondary, d.pickSample)
	clearSamples := widgets.NewButton(i18n.T("types.import.sample.clear"), widgets.ButtonSecondary, func() {
		for len(p.Samples()) > 0 {
			p.RemoveSample(0)
		}
		d.reset()
		d.refreshSamples()
		d.refreshPrivacy()
	})
	d.checkBtn = widgets.NewButton(i18n.T("types.import.check"), widgets.ButtonPrimary, d.check)
	d.samples = widgets.NewWrapLabel("", widgets.LabelMuted)

	d.privacy = widgets.VBox().(*fyne.Container)
	d.findings = widgets.VBox().(*fyne.Container)
	d.extra = widgets.VBox().(*fyne.Container)

	d.rules = grid.New(d.ruleColumns())
	d.rules.AccessibleLabel = i18n.T("types.rules")
	d.rules.Key = func(r presenter.ImportRuleRow) string { return string(rune('a' + r.Index)) }
	d.rules.StretchLast = true
	d.rulesBox = tableScroll(func() int { return d.rules.Len() }, d.rules)
	d.example = widgets.NewWrapLabel("", widgets.LabelBody)
	d.dry = grid.New(d.dryColumns())
	d.dry.AccessibleLabel = i18n.T("types.import.dry")
	d.dry.Key = func(r presenter.DryRunRow) string { return r.Sample }
	d.dry.StretchLast = true
	d.dryBox = tableScroll(func() int { return d.dry.Len() }, d.dry)

	d.collision = widgets.NewRadioGroup(nil, true, func(s string) {
		if s != "" {
			_ = p.SetCollision(s)
		}
	})
	d.collRow = widgets.NewFormRow(i18n.T("types.import.collision"), d.collision)
	d.collRow.Hide()

	d.dryHead = widgets.NewLabel(i18n.T("types.import.dry"), widgets.LabelHeading)
	d.results = widgets.VBox(
		widgets.NewLabel(i18n.T("types.import.results"), widgets.LabelHeading),
		d.findings,
		widgets.NewLabel(i18n.T("types.import.rules"), widgets.LabelHeading),
		d.rulesBox,
		d.example,
		d.dryHead,
		d.dryBox,
		d.extra,
		d.collRow,
	)
	d.results.Hide()

	body := widgets.VBox(
		widgets.NewWrapLabel(i18n.T("types.import.intro"), widgets.LabelMuted),
		d.snippet,
		widgets.HBox(paste, addSample, clearSamples, widgets.Spacer(), d.checkBtn),
		d.samples,
		d.privacy,
		d.results,
	)
	win := sh.Win.Canvas().Size()
	d.minWidth = min(max(win.Width*0.8, 560), 940)
	d.content = fyne.NewContainerWithLayout(sizeLayout{h: min(max(win.Height*0.7, 360), 640)},
		widgets.Scroll(widgets.PagePadded(body)))
	d.refreshSamples()
	return d
}

func gridHeight(rows int) float32 {
	row := theme.Size(theme.SizeRow)
	return min(float32(rows+1)*row+8, 10*row)
}

// tableScroll puts a table into its own scroll area, so a wide table scrolls
// sideways and the rest of the dialog keeps the dialog width. The height
// follows the row count (plus room for the scroll bar).
func tableScroll(rows func() int, table fyne.CanvasObject) fyne.CanvasObject {
	bar := theme.Size(theme.SizeRow) / 2
	return withFitHeight(func() float32 { return gridHeight(rows()) + bar }, widgets.Scroll(table))
}

// noteRow is a status badge with a text that wraps at the dialog width.
func noteRow(st theme.Status, word, text string) fyne.CanvasObject {
	return widgets.Border(nil, nil, widgets.NewStatusBadge(st, word), nil, widgets.NewWrapLabel(text, widgets.LabelBody))
}

// show opens the dialog. Cancel is the cancel button of the footer, the two
// save buttons follow it; they do nothing until a check allowed saving.
func (d *importDialog) show() {
	d.dlg = widgets.ShowDialog(d.sh.Win, i18n.T("types.import.title"), d.content, widgets.DialogOptions{
		MinWidth:    d.minWidth,
		CancelLabel: i18n.T("types.import.cancel"),
		Buttons: []widgets.DialogButton{
			{Label: i18n.T("types.import.saveEdit"), Variant: widgets.ButtonSecondary, KeepOpen: true, OnTap: func() { d.save(true) }},
			{Label: i18n.T("types.import.save"), Variant: widgets.ButtonPrimary, KeepOpen: true, OnTap: func() { d.save(false) }},
		},
	})
	d.sh.Win.Canvas().Focus(d.snippet)
}

func (d *importDialog) notify(st theme.Status, msg string) {
	d.sh.Toasts.Notify(st, msg, toastDur(6*time.Second))
}

func (d *importDialog) ruleColumns() []grid.Column[presenter.ImportRuleRow] {
	p := d.p
	return []grid.Column[presenter.ImportRuleRow]{
		{ID: "kind", Title: i18n.T("col.kind"), Width: 110,
			Value: func(r presenter.ImportRuleRow) string { return presenter.LabelOf(p.KindChoices(), r.Rule.Kind) }},
		{ID: "value", Title: i18n.T("col.value"), Width: 220, Value: func(r presenter.ImportRuleRow) string { return r.Rule.Value }},
		{ID: "mode", Title: i18n.T("col.mode"), Width: 80,
			Value: func(r presenter.ImportRuleRow) string { return presenter.LabelOf(p.ModeChoices(), r.Rule.Mode) }},
		{ID: "strength", Title: i18n.T("col.strength"), Width: 90,
			Value: func(r presenter.ImportRuleRow) string {
				return presenter.LabelOf(p.StrengthChoices(), presenter.StrengthID(r.Rule.Weight))
			}},
		{ID: "where", Title: i18n.T("col.where"), Width: 90,
			Value: func(r presenter.ImportRuleRow) string { return presenter.LabelOf(p.WhereChoices(), r.Rule.Where) }},
		{ID: "on", Title: i18n.T("col.enabled"), Width: 60,
			Value: func(r presenter.ImportRuleRow) string {
				if r.Rule.Enabled {
					return i18n.T("types.import.yes")
				}
				return i18n.T("types.import.no")
			}},
		{ID: "why", Title: i18n.T("types.import.why"), Width: 200, Value: func(r presenter.ImportRuleRow) string { return r.Why }},
	}
}

func (d *importDialog) dryColumns() []grid.Column[presenter.DryRunRow] {
	return []grid.Column[presenter.DryRunRow]{
		{ID: "sample", Title: i18n.T("types.import.dry.sample"), Width: 200, Value: func(r presenter.DryRunRow) string { return r.Sample }},
		{ID: "score", Title: i18n.T("types.import.dry.score"), Width: 80, Value: func(r presenter.DryRunRow) string { return r.Score }},
		{ID: "confident", Title: i18n.T("types.import.dry.confident"), Width: 100,
			Value: func(r presenter.DryRunRow) string {
				if r.Confident {
					return i18n.T("types.import.yes")
				}
				return i18n.T("types.import.no")
			}},
		{ID: "others", Title: i18n.T("types.import.dry.others"), Width: 300, Value: func(r presenter.DryRunRow) string { return r.Others }},
	}
}

// reset hides the results; the snippet or the samples changed.
func (d *importDialog) reset() {
	d.results.Hide()
	d.canSave = false
}

func (d *importDialog) refreshSamples() {
	names := make([]string, 0)
	for _, s := range d.p.Samples() {
		names = append(names, s.Name)
	}
	if len(names) == 0 {
		d.samples.SetText(i18n.T("types.import.sample.none"))
		return
	}
	d.samples.SetText(i18n.T("types.import.sample.list", "Names", strings.Join(names, ", ")))
}

// refreshPrivacy lists the own identifiers found in the snippet and samples.
func (d *importDialog) refreshPrivacy() {
	d.privSerial++
	serial := d.privSerial
	go func() {
		hits := d.p.PrivacyHits(context.Background())
		fyne.Do(func() {
			if serial != d.privSerial {
				return
			}
			d.privacy.Objects = nil
			if len(hits) > 0 {
				d.privacy.Add(noteRow(theme.StatusWarning, i18n.T("types.import.level.warning"),
					i18n.T("types.import.privacy", "Tokens", strings.Join(hits, ", "))))
				d.privacy.Add(widgets.NewWrapLabel(i18n.T("types.import.privacy.hint"), widgets.LabelMuted))
			}
			d.privacy.Refresh()
		})
	}()
}

func (d *importDialog) pickSample() {
	widgets.ShowFileOpen(d.sh.Win, widgets.FileDialogOptions{Extensions: []string{".pdf"}}, func(r io.ReadCloser, path string) {
		_ = r.Close()
		d.addSample(path)
	})
}

func (d *importDialog) addSample(path string) {
	go func() {
		err := d.p.AddSample(context.Background(), path)
		fyne.Do(func() {
			if err != nil {
				d.notify(theme.StatusDanger, i18n.T("types.import.sample.failed", "Reason", err.Error()))
				return
			}
			d.reset()
			d.refreshSamples()
			d.refreshPrivacy()
		})
	}()
}

func (d *importDialog) check() {
	d.checkBtn.Disable()
	go func() {
		err := d.p.Check(context.Background())
		fyne.Do(func() {
			d.checkBtn.Enable()
			if err != nil {
				d.showError(err)
				return
			}
			d.render()
		})
	}()
}

// showError shows a failed parse as one error row.
func (d *importDialog) showError(err error) {
	d.reset()
	d.findings.Objects = []fyne.CanvasObject{noteRow(theme.StatusDanger, i18n.T("types.import.level.error"), err.Error())}
	d.findings.Refresh()
	d.rules.SetItems(nil)
	d.dry.SetItems(nil)
	d.example.SetText("")
	d.extra.Objects = nil
	d.dryHead.Hide()
	d.dryBox.Hide()
	d.collRow.Hide()
	d.results.Show()
}

// render shows the report of the presenter.
func (d *importDialog) render() {
	p := d.p
	rep := p.Report()
	d.findings.Objects = nil
	for _, f := range rep.Findings {
		st, word := statusOf(f.Level)
		d.findings.Add(noteRow(st, word, p.FindingText(f)))
	}
	d.findings.Refresh()

	d.rules.SetItems(p.ImportRules())
	if pv := p.ImportPreview(); pv.Invalid {
		d.example.SetText(i18n.T("types.import.example.invalid", "Reason", pv.Message))
	} else {
		d.example.SetText(i18n.T("types.import.example", "Path", pv.Path))
	}
	dry := p.DryRun()
	d.dry.SetItems(dry)
	// No sample, no dry run: an empty table would only leave a gap.
	d.dryHead.Show()
	d.dryBox.Show()
	if len(dry) == 0 {
		d.dryHead.Hide()
		d.dryBox.Hide()
	}

	d.extra.Objects = nil
	for _, w := range p.ImportWarnings() {
		d.extra.Add(noteRow(theme.StatusWarning, i18n.T("types.import.level.warning"), i18n.T("types.import.note", "Text", w)))
	}
	for _, h := range p.LibraryHits() {
		d.extra.Add(noteRow(theme.StatusWarning, i18n.T("types.import.level.warning"),
			i18n.T("types.import.libhit", "Path", h.Path, "Type", h.TypeName, "Score", h.Score)))
	}
	d.extra.Refresh()

	if p.Collides() {
		var labels []string
		for _, c := range p.CollisionChoices() {
			labels = append(labels, c.Label)
		}
		d.collision.Options = labels
		d.collision.SetSelected(presenter.LabelOf(p.CollisionChoices(), p.Collision()))
		d.collision.Refresh()
		d.collRow.Show()
	} else {
		d.collRow.Hide()
	}
	d.canSave = p.CanSave()
	d.rulesBox.Refresh()
	d.dryBox.Refresh()
	d.results.Show()
}

func (d *importDialog) save(andEdit bool) {
	if !d.canSave {
		return
	}
	d.canSave = false
	go func() {
		id, err := d.p.ImportType(context.Background(), andEdit)
		fyne.Do(func() {
			if err != nil {
				d.notify(theme.StatusDanger, i18n.T("types.save.failed", "Reason", err.Error()))
				d.render()
				return
			}
			if d.dlg != nil {
				d.dlg.Hide()
			}
			d.notify(theme.StatusSuccess, i18n.T("types.import.saved"))
			if d.onDone != nil {
				d.onDone(id, andEdit)
			}
		})
	}()
}
