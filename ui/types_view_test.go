package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
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
