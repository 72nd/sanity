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
		choice("kind", i18n.T("col.kind"), 110, p.KindChoices, func(r presenter.RuleRow) string { return r.Rule.Kind }, p.SetRuleKind),
		{ID: "value", Title: i18n.T("col.value"), Width: 280, Editable: true,
			Value: func(r presenter.RuleRow) string { return r.Rule.Value },
			Set: func(r *presenter.RuleRow, text string) error {
				if err := p.SetRuleValue(r.Index, text); err != nil {
					return err
				}
				*r = p.Rules()[r.Index]
				return nil
			}},
		choice("mode", i18n.T("col.mode"), 90, p.ModeChoices, func(r presenter.RuleRow) string { return r.Rule.Mode }, p.SetRuleMode),
		choice("strength", i18n.T("col.strength"), 100, p.StrengthChoices,
			func(r presenter.RuleRow) string { return presenter.StrengthID(r.Rule.Weight) }, p.SetRuleStrength),
		choice("where", i18n.T("col.where"), 100, p.WhereChoices, func(r presenter.RuleRow) string { return r.Rule.Where }, p.SetRuleWhere),
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
	rulesBox := withMinHeight(240, rules)

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

	notify := func(st theme.Status, msg string) { sh.Toasts.Notify(st, msg, 5*time.Second) }

	empty := widgets.NewEmptyState(fynetheme.IconNameDocument, i18n.T("types.empty.title"), i18n.T("types.empty.desc"), nil)

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
	bar := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeBottom, 6,
		widgets.HBox(state, widgets.Spacer(), revertBtn, saveBtn))
	detailPane = widgets.Border(bar, nil, nil, nil, body)

	listButtons := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeTop, 6, widgets.HBox(addBtn, dupBtn, delBtn))
	left := widgets.Border(nil, listButtons, nil, nil, list)
	right := widgets.Stack(detailPane, empty)

	split := widgets.HSplit(left, right)
	split.SetOffset(0.25)

	go func() { // never block the UI thread on the store
		var err error
		if !p.Dirty() { // unsaved edits survive a visit to another page
			err = p.Load(context.Background())
		}
		fyne.Do(func() {
			if err != nil {
				notify(theme.StatusDanger, i18n.T("toast.load.failed"))
			}
			populate()
		})
	}()
	return split, &typesWidgets{list: list, rules: rules}
}
