package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/locales"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/ui"
	"github.com/72nd/sanity/ui/presenter"
)

func TestOpenServicesAndClose(t *testing.T) {
	newEngine = func() pdf.Engine { return pdf.NewFake() }
	t.Cleanup(func() { newEngine = pdf.NewEngine })
	root := t.TempDir()
	act := ui.NewSortActions()

	s, err := openServices(root, false, act)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != filepath.Base(root) {
		t.Errorf("Name = %q", s.Name)
	}
	if _, err := os.Stat(filepath.Join(root, "office", "sanity.json")); err != nil {
		t.Errorf("project not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "office", "inbox")); err != nil {
		t.Errorf("inbox not created: %v", err)
	}
	s.Close()
	s.Close() // idempotent

	// A second project opens next to the first without interference.
	other := t.TempDir()
	s2, err := openServices(other, false, act)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if s2.Root == s.Root {
		t.Error("roots equal")
	}

	d, err := openServices(t.TempDir(), true, act)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Muster GmbH" {
		t.Errorf("demo name = %q", d.Name)
	}
	d.Close()
}

func TestSwitchToReplacesServices(t *testing.T) {
	newEngine = func() pdf.Engine { return pdf.NewFake() }
	t.Cleanup(func() { newEngine = pdf.NewEngine })
	i18n.MustAddFiles(locales.FS, locales.Files...)
	i18n.SetLanguage("en")
	a := &application{act: ui.NewSortActions(), keys: presenter.NewKeys(nil)}
	first, second := t.TempDir(), t.TempDir()
	s, err := openServices(first, false, a.act)
	if err != nil {
		t.Fatal(err)
	}
	a.cur = s
	a.nav = &ui.ProjectNav{Switch: a.switchTo}
	a.sh = shell.New(test.NewTempApp(t), shell.Config{TitleKey: "app.title", Pages: a.pages(), Width: 1000, Height: 700})
	a.recents = presenter.NewRecents(a.sh.Prefs)
	a.nav.Recents = a.recents

	a.switchTo(second)
	defer a.cur.Close()
	if a.cur.Root != second || s.stop != nil {
		t.Errorf("root=%q (want %q), old watcher stopped=%v", a.cur.Root, second, s.stop == nil)
	}
	if got := a.recents.List(); len(got) != 1 || got[0].Root != second {
		t.Errorf("recents = %v", got)
	}
	if want := filepath.Base(second) + " – "; !strings.HasPrefix(a.sh.Win.Title(), want) {
		t.Errorf("window title = %q", a.sh.Win.Title())
	}
	if a.sh.Current() != "library" {
		t.Errorf("current page = %q", a.sh.Current())
	}
}

func TestMain(m *testing.M) {
	ui.SyncBackground()
	os.Exit(m.Run())
}

// A project writes its correction log under office/.sanity; the demo does not.
func TestServicesWireCorrectionLog(t *testing.T) {
	newEngine = func() pdf.Engine { return pdf.NewFake() }
	t.Cleanup(func() { newEngine = pdf.NewEngine })
	root := t.TempDir()
	s, err := openServices(root, false, ui.NewSortActions())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.sort.OnCorrection == nil {
		t.Fatal("OnCorrection not set")
	}
	s.sort.OnCorrection(inbox.Correction{SHA256: "abc", TypeID: "x"})
	cs, err := inbox.ReadAll(s.Root)
	if err != nil || len(cs) != 1 || cs[0].SHA256 != "abc" {
		t.Errorf("log %v %v", cs, err)
	}
	d, err := openServices(t.TempDir(), true, ui.NewSortActions())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.sort.OnCorrection != nil {
		t.Error("the demo must not write a log")
	}
}
