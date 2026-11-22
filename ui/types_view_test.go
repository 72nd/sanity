package ui

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/widgets/grid"
	"github.com/72nd/sanity/locales"
	"github.com/72nd/sanity/store"
	"github.com/72nd/sanity/ui/presenter"
)

// The types page opens with the fake store: the list shows the types and the
// rules grid the rules of the first one.
func TestTypesPageShowsRules(t *testing.T) {
	app := test.NewTempApp(t)
	i18n.MustAddFiles(locales.FS, locales.Files...)
	i18n.SetLanguage("en")
	tp := presenter.NewTypes(store.NewFake(), i18n.T)
	var w *typesWidgets
	sh := shell.New(app, shell.Config{TitleKey: "app.title", Width: 1200, Height: 760,
		Pages: []shell.Page{{ID: "types", TitleKey: "nav.types",
			Build: func(sh *shell.Shell) fyne.CanvasObject {
				var obj fyne.CanvasObject
				obj, w = typesPage(sh, tp)
				return obj
			}}}})
	sh.Show("types")
	for i := 0; i < 300 && w.rules.Len() == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if got := w.list.Len(); got != 5 {
		t.Errorf("type list rows = %d, want 5", got)
	}
	if got := w.rules.Len(); got == 0 || got != len(tp.Rules()) {
		t.Errorf("rule rows = %d, presenter has %d", got, len(tp.Rules()))
	}
	if tp.Dirty() {
		t.Error("dirty after opening")
	}
}

// The Kind, Mode, Strength and Where cells are drop-downs: a pick of another
// option goes through the presenter.
func TestRuleChoiceColumnsAreDropdowns(t *testing.T) {
	test.NewTempApp(t)
	i18n.MustAddFiles(locales.FS, locales.Files...)
	i18n.SetLanguage("en")
	tp := presenter.NewTypes(store.NewFake(), i18n.T)
	if err := tp.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	g := grid.New(ruleColumns(tp))
	g.SetItems(tp.Rules())
	w := test.NewTempWindow(t, g)
	w.Resize(fyne.NewSize(900, 300))
	cols := []struct {
		col     int
		choices func() []presenter.Choice
		id      func(presenter.RuleRow) string
	}{
		{1, tp.KindChoices, func(r presenter.RuleRow) string { return r.Rule.Kind }},
		{3, tp.ModeChoices, func(r presenter.RuleRow) string { return r.Rule.Mode }},
		{4, tp.StrengthChoices, func(r presenter.RuleRow) string { return presenter.StrengthID(r.Rule.Weight) }},
		{5, tp.WhereChoices, func(r presenter.RuleRow) string { return r.Rule.Where }},
	}
	for _, c := range cols {
		if !g.StartEdit(0, c.col) || !g.Editing() {
			t.Fatalf("column %d did not open an editor", c.col)
		}
		cur := presenter.LabelOf(c.choices(), c.id(tp.Rules()[0]))
		target := ""
		for _, l := range choiceLabels(c.choices()) {
			if l != cur {
				target = l
				break
			}
		}
		g.SetEditText(target)
		if !g.CommitEdit() {
			t.Fatalf("column %d: pick %q rejected", c.col, target)
		}
		if got := presenter.LabelOf(c.choices(), c.id(tp.Rules()[0])); got != target {
			t.Errorf("column %d = %q, want %q", c.col, got, target)
		}
	}
}
