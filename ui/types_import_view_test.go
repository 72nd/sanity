package ui

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/sanity/locales"
	"github.com/72nd/sanity/store"
	"github.com/72nd/sanity/ui/presenter"
)

const importSnippet = `{"type": {"id": "hetzner-invoice", "name": "Hetzner Invoice",
 "pathTemplate": "receipts/{date:2006}/hosting/{date:2006-01-02}_hetzner_{invoice}.pdf",
 "rules": [
  {"kind": "domain", "value": "hetzner.com", "mode": "should", "weight": 0.9, "where": "header", "enabled": true},
  {"kind": "vatId", "value": "DE812871812", "mode": "must", "weight": 0.9, "where": "any", "enabled": true}],
 "threshold": 0.8, "useLearned": true, "date": {"source": "auto", "maxAgeYears": 2}},
 "notes": [{"rule": 0, "why": "Sender domain in the letterhead"}]}`

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for i := 0; i < 300 && !ok(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if !ok() {
		t.Fatalf("timeout: %s", what)
	}
}

// The import dialog checks the worked example of the plan, enables Save and
// stores the type through the (fake) store.
func TestImportDialogChecksAndSaves(t *testing.T) {
	app := test.NewTempApp(t)
	i18n.MustAddFiles(locales.FS, locales.Files...)
	i18n.SetLanguage("en")
	st := store.NewFake()
	tp := presenter.NewTypes(st, i18n.T)
	preload(t, tp)
	if err := tp.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	sh := shell.New(app, shell.Config{TitleKey: "app.title", Width: 1200, Height: 760,
		Pages: []shell.Page{{ID: "types", TitleKey: "nav.types",
			Build: func(*shell.Shell) fyne.CanvasObject { return widgets.VBox() }}}})
	sh.Show("types")

	var saved string
	d := newImportDialog(sh, tp, func(id string, andEdit bool) { saved = id })
	d.show()
	if !d.saveBtn.Disabled() {
		t.Error("Save enabled before the check")
	}
	d.snippet.SetText(importSnippet)
	test.Tap(d.checkBtn)
	waitFor(t, "check result", func() bool { return !d.saveBtn.Disabled() })
	if !d.results.Visible() || len(d.findings.Objects) == 0 || d.rules.Len() != 2 {
		t.Errorf("results visible %v findings %d rules %d", d.results.Visible(), len(d.findings.Objects), d.rules.Len())
	}
	test.Tap(d.saveEdit)
	waitFor(t, "saved", func() bool { return saved != "" })
	types, _ := st.Types(context.Background())
	found := false
	for _, ty := range types {
		found = found || ty.ID == "hetzner-invoice"
	}
	if !found || saved != "hetzner-invoice" || tp.CurrentID() != "hetzner-invoice" {
		t.Errorf("saved %q current %q stored %v", saved, tp.CurrentID(), found)
	}
}

// A broken snippet shows an error row and keeps Save disabled.
func TestImportDialogRejectsBrokenSnippet(t *testing.T) {
	app := test.NewTempApp(t)
	i18n.MustAddFiles(locales.FS, locales.Files...)
	i18n.SetLanguage("en")
	tp := presenter.NewTypes(store.NewFake(), i18n.T)
	preload(t, tp)
	if err := tp.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	sh := shell.New(app, shell.Config{TitleKey: "app.title", Width: 1200, Height: 760,
		Pages: []shell.Page{{ID: "types", TitleKey: "nav.types",
			Build: func(*shell.Shell) fyne.CanvasObject { return widgets.VBox() }}}})
	sh.Show("types")
	d := newImportDialog(sh, tp, nil)
	d.show()
	d.snippet.SetText("not json")
	test.Tap(d.checkBtn)
	waitFor(t, "error row", func() bool { return d.results.Visible() })
	if !d.saveBtn.Disabled() || len(d.findings.Objects) != 1 {
		t.Errorf("save disabled %v rows %d", d.saveBtn.Disabled(), len(d.findings.Objects))
	}
}

// The commands exist and call the page actions.
func TestTypesCommands(t *testing.T) {
	a := NewTypesActions()
	var n int
	a.Import, a.CopyPrompt = func() { n++ }, func() { n += 10 }
	cs := TypesCommands(a)
	if len(cs) != 2 || cs[0].ID != "types.import" || cs[1].ID != "types.copyprompt" {
		t.Fatalf("commands %+v", cs)
	}
	cs[0].Run()
	cs[1].Run()
	if n != 11 {
		t.Errorf("n = %d", n)
	}
}
