package ui

import (
	"context"
	"encoding/json"
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

type testPrefs map[string]string

func (m testPrefs) LoadJSON(k string, v any) bool {
	s, ok := m[k]
	return ok && json.Unmarshal([]byte(s), v) == nil
}
func (m testPrefs) SaveJSON(k string, v any) { b, _ := json.Marshal(v); m[k] = string(b) }

// The project page opens with the fake store: the fields show the project, an
// edit marks it dirty, an invalid IBAN blocks Save and a valid edit saves.
func TestProjectPageEditsAndSaves(t *testing.T) {
	app := test.NewTempApp(t)
	i18n.MustAddFiles(locales.FS, locales.Files...)
	i18n.SetLanguage("en")
	st := store.NewFake()
	pp := presenter.NewProject(st, i18n.T)
	rec := presenter.NewRecents(testPrefs{})
	rec.Exists = func(string) bool { return true }
	rec.Add("/p/one", "One")
	nav := &ProjectNav{Recents: rec, Switch: func(string) {}}
	var w *projectWidgets
	sh := shell.New(app, shell.Config{TitleKey: "app.title", Width: 1200, Height: 760,
		Pages: []shell.Page{{ID: "project", TitleKey: "nav.project",
			Build: func(sh *shell.Shell) fyne.CanvasObject {
				var obj fyne.CanvasObject
				obj, w = projectPage(sh, pp, nav)
				return obj
			}}}})
	sh.Show("project")
	for i := 0; i < 300 && w.name.Text == ""; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if w.name.Text != "Muster GmbH" || w.inbox.Text != "office/inbox" {
		t.Fatalf("fields: %q %q", w.name.Text, w.inbox.Text)
	}
	if !w.inbox.Mono() {
		t.Error("inbox field is not monospace")
	}
	if !w.save.Disabled() {
		t.Errorf("save disabled=%v", w.save.Disabled())
	}
	w.name.SetText("Neu AG")
	if !pp.Dirty() || w.save.Disabled() {
		t.Error("edit did not enable Save")
	}
	w.ignore.SetText("DE89 3704 0044 0532 0130 01")
	if !w.save.Disabled() || w.ignoreRow.Error() == "" {
		t.Error("invalid IBAN must block Save and show an error")
	}
	w.ignore.SetText("DE89 3704 0044 0532 0130 00")
	if w.save.Disabled() || w.ignoreRow.Error() != "" {
		t.Error("valid IBAN rejected")
	}
	w.save.OnTapped()
	for i := 0; i < 300 && pp.Dirty(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	pr, _ := st.Project(context.Background())
	if pr.Name != "Neu AG" || len(pr.IgnoreTokens) != 1 {
		t.Errorf("not saved: %+v", pr)
	}
}
