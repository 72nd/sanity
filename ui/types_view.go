package ui

import (
	"context"
	"slices"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/theme"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/fine/widgets/grid"
	"github.com/72nd/sanity/ui/presenter"
)

// typeColumns is the list of document types on the left.
func typeColumns() []grid.Column[presenter.TypeRow] {
	return []grid.Column[presenter.TypeRow]{
		{ID: "name", Title: i18n.T("col.type"), Width: 240,
			Value: func(r presenter.TypeRow) string { return r.Name }},
	}
}

// choiceLabels returns the labels of a choice list for a Select.
func choiceLabels(cs []presenter.Choice) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Label
	}
	return out
}

// dropdown turns a choice column into a drop-down over the labels of choices.
func dropdown(col grid.Column[presenter.RuleRow], choices func() []presenter.Choice) grid.Column[presenter.RuleRow] {
	col.Options = func(presenter.RuleRow) []string { return choiceLabels(choices()) }
	return col
}

// ruleColumns builds the editable rule grid. Choice columns take the id or the
// localized label as text; the presenter validates it.
func ruleColumns(p *presenter.Types) []grid.Column[presenter.RuleRow] {
	choice := func(id, title string, w float32, choices func() []presenter.Choice, get func(presenter.RuleRow) string,
		set func(int, string) error) grid.Column[presenter.RuleRow] {
		return grid.Column[presenter.RuleRow]{ID: id, Title: title, Width: w, Editable: true,
			Value: func(r presenter.RuleRow) string { return presenter.LabelOf(choices(), get(r)) },
			Less:  func(a, b presenter.RuleRow) bool { return get(a) < get(b) },
			Set: func(r *presenter.RuleRow, text string) error {
				if err := set(r.Index, text); err != nil {
					return err
				}
				*r = p.Rules()[r.Index]
				return nil
			}}
	}
	return []grid.Column[presenter.RuleRow]{
		{ID: "on", Title: i18n.T("col.enabled"), Width: 70, Editable: true,
			Check: func(r presenter.RuleRow) bool { return r.Rule.Enabled },
			SetCheck: func(r *presenter.RuleRow, on bool) error {
				p.SetRuleEnabled(r.Index, on)
				r.Rule.Enabled = on
				return nil
			}},
		dropdown(choice("kind", i18n.T("col.kind"), 130, p.KindChoices, func(r presenter.RuleRow) string { return r.Rule.Kind }, p.SetRuleKind),
			p.KindChoices),
		{ID: "value", Title: i18n.T("col.value"), Width: 280, Editable: true,
			Value: func(r presenter.RuleRow) string { return r.Rule.Value },
			Set: func(r *presenter.RuleRow, text string) error {
				if err := p.SetRuleValue(r.Index, text); err != nil {
					return err
				}
				*r = p.Rules()[r.Index]
				return nil
			}},
		dropdown(choice("mode", i18n.T("col.mode"), 110, p.ModeChoices, func(r presenter.RuleRow) string { return r.Rule.Mode }, p.SetRuleMode),
			p.ModeChoices),
		dropdown(choice("strength", i18n.T("col.strength"), 120, p.StrengthChoices,
			func(r presenter.RuleRow) string { return presenter.StrengthID(r.Rule.Weight) }, p.SetRuleStrength), p.StrengthChoices),
		dropdown(choice("where", i18n.T("col.where"), 120, p.WhereChoices, func(r presenter.RuleRow) string { return r.Rule.Where }, p.SetRuleWhere),
			p.WhereChoices),
	}
}

// typesWidgets are the grids of the types page, for tests.
type typesWidgets struct {
	list  *grid.Grid[presenter.TypeRow]
	rules *grid.Grid[presenter.RuleRow]
}

// TypesPage builds the types editor: the list of types on the left, the
// detail cards on the right.
func TypesPage(sh *shell.Shell, p *presenter.Types) fyne.CanvasObject {
	obj, _ := typesPage(sh, p)
	return obj
}

func typesPage(sh *shell.Shell, p *presenter.Types) (fyne.CanvasObject, *typesWidgets) {
	var syncing bool // true while code, not the user, fills the widgets

	// --- list ---
	list := grid.New(typeColumns())
	list.AccessibleLabel = i18n.T("nav.types")
	list.Mode = grid.SelectSingle
	list.Key = func(r presenter.TypeRow) string { return r.ID }
	list.StretchLast = true

	// --- basics ---
	name := widgets.NewEntry()
	nameRow := widgets.NewFormRow(i18n.T("types.name"), name)
	tpl := widgets.NewEntry()
	tpl.SetPlaceHolder("receipts/{date:2006}/{date:2006-01-02}_name_{invoice}.pdf")
	tplRow := widgets.NewFormRow(i18n.T("types.template"), tpl)
	badge := widgets.NewStatusBadge(theme.StatusSuccess, i18n.T("types.preview.ok"))
	previewText := widgets.NewLabel("", widgets.LabelBody)
	chooseBtn := widgets.NewButton(i18n.T("types.folder.choose"), widgets.ButtonSecondary, nil)

	// --- recognition ---
	threshold := widgets.NewNumberEntry()
	threshold.SetRange(0, 1)
	threshold.SetStep(0.05)
	threshold.SetDecimals(2)
	learned := widgets.NewSwitch(i18n.T("types.learned"), nil)
	rules := grid.New(ruleColumns(p))
	rules.AccessibleLabel = i18n.T("types.rules")
	rules.Key = func(r presenter.RuleRow) string { return strconv.Itoa(r.Index) }
	rules.StretchLast = true
	// The grid is as tall as its rows (header included), between three rows and
	// a limit; beyond that it scrolls on its own.
	rulesBox := withFitHeight(func() float32 {
		row := theme.Size(theme.SizeRow)
		return min(max(float32(rules.Len()+1)*row+8, 4*row), 12*row)
	}, rules)
	relayout := func() {} // set once the page scroll exists; the grid height changes with the rule count

	// --- date ---
	preferred := widgets.NewEntry()
	ignored := widgets.NewEntry()
	source := widgets.NewSelect(choiceLabels(p.SourceChoices()), nil)
	maxAge := widgets.NewNumberEntry()
	maxAge.SetRange(0, 100)
	maxAge.SetDecimals(0)

	// --- bar ---
	state := widgets.NewLabel("", widgets.LabelMuted)
	saveBtn := widgets.NewButton(i18n.T("types.save"), widgets.ButtonPrimary, nil)
	revertBtn := widgets.NewButton(i18n.T("types.revert"), widgets.ButtonSecondary, nil)
	addBtn := widgets.NewButton(i18n.T("types.add"), widgets.ButtonSecondary, nil)
	dupBtn := widgets.NewButton(i18n.T("types.duplicate"), widgets.ButtonSecondary, nil)
	delBtn := widgets.NewButton(i18n.T("types.delete"), widgets.ButtonDanger, nil)

	notify := func(st theme.Status, msg string) { sh.Toasts.Notify(st, msg, toastDur(5*time.Second)) }

	createBtn := widgets.NewButton(i18n.T("types.empty.create"), widgets.ButtonPrimary, nil)
	importBtn := widgets.NewButton(i18n.T("types.import.button"), widgets.ButtonSecondary, nil)
	importEmptyBtn := widgets.NewButton(i18n.T("types.import.button"), widgets.ButtonSecondary, nil)
	promptBtn := widgets.NewButton(i18n.T("types.copyPrompt.button"), widgets.ButtonSecondary, nil)
	jsonBtn := widgets.NewButton(i18n.T("types.copyJSON.button"), widgets.ButtonSecondary, nil)
	emptyState := widgets.NewEmptyState(fynetheme.IconNameDocument, i18n.T("types.empty.title"), i18n.T("types.empty.desc"), createBtn)
	empty := widgets.Border(nil, widgets.HBox(widgets.Spacer(), importEmptyBtn, widgets.Spacer()), nil, nil, emptyState)

	refreshPreview := func() {
		pv := p.Preview()
		switch {
		case pv.Invalid:
			badge.Status, badge.Label = theme.StatusDanger, i18n.T("types.preview.bad")
			previewText.SetText(pv.Message)
			tplRow.SetError(pv.Message)
		default:
			badge.Status, badge.Label = theme.StatusSuccess, i18n.T("types.preview.ok")
			previewText.SetText("→ " + pv.Path)
			tplRow.SetError("")
		}
		badge.Refresh()
	}
	refreshState := func() {
		state.SetText(p.StateText())
		if p.Dirty() {
			saveBtn.Enable()
			revertBtn.Enable()
		} else {
			saveBtn.Disable()
			revertBtn.Disable()
		}
		refreshPreview()
	}
	setText := func(e *widgets.Entry, s string) {
		if e.Text != s {
			e.SetText(s)
		}
	}
	refreshList := func() {
		syncing = true
		list.SetItems(p.List())
		if id := p.CurrentID(); id != "" {
			list.SelectKeys([]string{id})
		}
		syncing = false
	}

	var detailPane fyne.CanvasObject // the cards; hidden while no type exists
	var populate func()
	populate = func() {
		syncing = true
		cur, ok := p.Current()
		if ok {
			empty.Hide()
			detailPane.Show()
			setText(name, cur.Name)
			setText(tpl, cur.PathTemplate)
			threshold.SetValue(cur.Threshold)
			learned.SetOn(cur.UseLearned)
			rules.SetItems(p.Rules())
			rulesBox.Refresh()
			relayout()
			setText(preferred, presenter.JoinList(cur.Date.PreferredLabels))
			setText(ignored, presenter.JoinList(cur.Date.IgnoreLabels))
			source.SetSelected(presenter.LabelOf(p.SourceChoices(), cur.Date.Source))
			maxAge.SetValue(float64(cur.Date.MaxAgeYears))
		} else {
			detailPane.Hide()
			empty.Show()
		}
		syncing = false
		refreshList()
		refreshState()
	}

	name.OnChanged = func(s string) {
		if syncing {
			return
		}
		p.SetName(s)
		refreshList()
		refreshState()
	}
	tpl.OnChanged = func(s string) {
		if !syncing {
			p.SetPathTemplate(s)
			refreshState()
		}
	}
	threshold.OnChanged = func(v float64) {
		if !syncing {
			p.SetThreshold(v)
			refreshState()
		}
	}
	learned.OnChanged = func(on bool) {
		if !syncing {
			p.SetUseLearned(on)
			refreshState()
		}
	}
	preferred.OnChanged = func(s string) {
		if !syncing {
			p.SetPreferredLabels(s)
			refreshState()
		}
	}
	ignored.OnChanged = func(s string) {
		if !syncing {
			p.SetIgnoreLabels(s)
			refreshState()
		}
	}
	source.OnChanged = func(s string) {
		if !syncing {
			if err := p.SetDateSource(s); err != nil {
				notify(theme.StatusDanger, err.Error())
			}
			refreshState()
		}
	}
	maxAge.OnChanged = func(v float64) {
		if !syncing {
			p.SetMaxAgeYears(int(v))
			refreshState()
		}
	}
	rules.OnEdited = func(*presenter.RuleRow, int) { refreshState() }

	// Legend: a few lines of Go layout tokens.
	items := p.Legend()
	var legendLines []fyne.CanvasObject
	for i := 0; i < len(items); i += 4 {
		line := ""
		for _, it := range items[i:min(i+4, len(items))] {
			if line != "" {
				line += "   "
			}
			line += it.Token + " = " + it.Meaning + " (" + it.Example + ")"
		}
		legendLines = append(legendLines, widgets.NewLabel(line, widgets.LabelSmall))
	}
	legendBox := widgets.VBox(legendLines...)

	chooseBtn.OnTapped = func() {
		widgets.ShowFolderOpen(sh.Win, widgets.FolderDialogOptions{StartDir: p.Root()}, func(dir string) {
			if _, err := p.InsertFolder(dir); err != nil {
				notify(theme.StatusDanger, i18n.T("types.folder.outside"))
				return
			}
			cur, _ := p.Current()
			syncing = true
			setText(tpl, cur.PathTemplate)
			syncing = false
			refreshState()
		})
	}

	addRule := widgets.NewButton(i18n.T("types.rule.add"), widgets.ButtonSecondary, func() {
		i := p.AddRule()
		rules.SetItems(p.Rules())
		rulesBox.Refresh()
		relayout()
		rules.SelectKeys([]string{strconv.Itoa(i)})
		refreshState()
	})
	removeRule := widgets.NewButton(i18n.T("types.rule.remove"), widgets.ButtonSecondary, func() {
		sel := rules.Selected()
		idx := make([]int, 0, len(sel))
		for _, r := range sel {
			idx = append(idx, r.Index)
		}
		slices.SortFunc(idx, func(a, b int) int { return b - a }) // highest first keeps the others valid
		for _, i := range idx {
			p.RemoveRule(i)
		}
		rules.SetItems(p.Rules())
		rulesBox.Refresh()
		relayout()
		refreshState()
	})

	save := func() {
		go func() {
			err := p.Save(context.Background())
			fyne.Do(func() {
				if err != nil {
					notify(theme.StatusDanger, i18n.T("types.save.failed", "Reason", err.Error()))
					return
				}
				populate()
				notify(theme.StatusSuccess, i18n.T("types.saved"))
			})
		}()
	}
	saveBtn.OnTapped = save
	revertBtn.OnTapped = func() { p.Revert(); populate() }
	addBtn.OnTapped = func() { p.Add(); populate() }
	createBtn.OnTapped = addBtn.OnTapped

	// --- import, copy as JSON, copy LLM prompt ---
	copyText := func(text, doneID string) {
		sh.App.Clipboard().SetContent(text)
		notify(theme.StatusSuccess, i18n.T(doneID))
	}
	copyPrompt := func() { copyText(p.PromptText(), "types.copyPrompt.done") }
	copyJSON := func(id string) {
		js, err := p.TypeJSONOf(id)
		if err != nil {
			notify(theme.StatusDanger, i18n.T("types.copyJSON.failed", "Reason", err.Error()))
			return
		}
		copyText(js, "types.copyJSON.done")
	}
	openImport := func() {
		open := func() {
			newImportDialog(sh, p, func(id string, andEdit bool) {
				populate()
				if andEdit {
					sh.Win.Canvas().Focus(name)
				}
			}).show()
		}
		if !p.Dirty() {
			open()
			return
		}
		widgets.ShowConfirm(sh.Win, i18n.T("types.discard.title"), i18n.T("types.discard.message"),
			i18n.T("types.discard"), true, func(yes bool) {
				if yes {
					open()
				}
			})
	}
	importBtn.OnTapped, importEmptyBtn.OnTapped = openImport, openImport
	promptBtn.OnTapped = copyPrompt
	jsonBtn.OnTapped = func() { copyJSON(p.CurrentID()) }
	act := SharedTypesActions()
	act.Import, act.CopyPrompt = openImport, copyPrompt
	dupBtn.OnTapped = func() {
		if err := p.Duplicate(); err == nil {
			populate()
		}
	}
	delBtn.OnTapped = func() {
		cur, ok := p.Current()
		if !ok {
			return
		}
		widgets.ShowConfirm(sh.Win, i18n.T("types.delete.title"), i18n.T("types.delete.message", "Name", cur.Name),
			i18n.T("types.delete"), true, func(yes bool) {
				if !yes {
					return
				}
				go func() {
					err := p.Delete(context.Background())
					fyne.Do(func() {
						if err != nil {
							notify(theme.StatusDanger, i18n.T("types.delete.failed", "Reason", err.Error()))
							return
						}
						populate()
					})
				}()
			})
	}

	list.OnContextMenu = func(r *presenter.TypeRow, ev *fyne.PointEvent) {
		if r == nil {
			return
		}
		id := r.ID
		widgets.ShowContextMenu(list, ev.AbsolutePosition, []widgets.MenuItem{
			{Label: i18n.T("types.copyJSON.button"), Action: func() { copyJSON(id) }},
		})
	}
	list.OnSelectionChanged = func() {
		if syncing {
			return
		}
		sel := list.Selected()
		if len(sel) != 1 || sel[0].ID == p.CurrentID() {
			return
		}
		id := sel[0].ID
		switch {
		case !p.Dirty():
			p.Select(id)
			populate()
		default:
			widgets.ShowConfirm(sh.Win, i18n.T("types.discard.title"), i18n.T("types.discard.message"),
				i18n.T("types.discard"), true, func(yes bool) {
					if yes {
						p.Select(id)
					}
					populate()
				})
		}
	}

	basics := widgets.NewCard(i18n.T("types.card.basics"), "", widgets.VBox(
		nameRow, tplRow,
		widgets.HBox(badge, previewText),
		legendBox,
		widgets.HBox(chooseBtn),
	))
	recognition := widgets.NewCard(i18n.T("types.card.recognition"), i18n.T("types.card.recognition.desc"), widgets.VBox(
		widgets.NewFormRow(i18n.T("types.threshold"), threshold),
		learned,
		rulesBox,
		widgets.HBox(addRule, removeRule),
	))
	dateCard := widgets.NewCard(i18n.T("types.card.date"), i18n.T("types.card.date.desc"), widgets.VBox(
		widgets.NewFormRow(i18n.T("types.date.preferred"), preferred),
		widgets.NewFormRow(i18n.T("types.date.ignored"), ignored),
		widgets.NewFormRow(i18n.T("types.date.source"), source),
		widgets.NewFormRow(i18n.T("types.date.maxage"), maxAge),
	))
	body := widgets.Scroll(widgets.PagePadded(widgets.VBox(basics, recognition, dateCard)))
	relayout = body.Refresh
	bar := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeBottom, 6,
		widgets.HBox(state, widgets.Spacer(), jsonBtn, revertBtn, saveBtn))
	detailPane = widgets.Border(bar, nil, nil, nil, body)

	listButtons := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeTop, 6, widgets.HBox(addBtn, dupBtn, delBtn))
	listTools := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeBottom, 6, widgets.HBox(importBtn, promptBtn))
	left := widgets.Border(listTools, listButtons, nil, nil, list)
	right := widgets.Stack(detailPane, empty)

	split := widgets.HSplit(left, right)
	split.SetOffset(0.3)

	if p.Fresh() || p.Dirty() {
		populate() // loaded a moment ago, or unsaved edits that a reload would drop
	} else {
		background(func() { // never block the UI thread on the store
			err := p.Load(context.Background())
			fyne.Do(func() {
				if err != nil {
					notify(theme.StatusDanger, i18n.T("toast.load.failed"))
				}
				populate()
			})
		})
	}
	return split, &typesWidgets{list: list, rules: rules}
}
