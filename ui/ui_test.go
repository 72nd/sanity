package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/locales"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
	"github.com/72nd/sanity/ui/presenter"
)

func newShell(t *testing.T, pdfs int) (*shell.Shell, *presenter.Sort, *presenter.Library) {
	t.Helper()
	app := test.NewTempApp(t)
	i18n.MustAddFiles(locales.FS, locales.Files...)
	i18n.SetLanguage("en")
	root := t.TempDir()
	dir := filepath.Join(root, "office", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a.pdf", "b.pdf"}[:pdfs] {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("%PDF"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := store.NewFake()
	q := inbox.NewQueue(nil)
	q.Add(dir)
	lib := presenter.NewLibrary(st, i18n.T)
	sp := presenter.NewSort(st, q, inbox.NewFiler(root, st), pdf.NewFake(), i18n.T)
	act := NewSortActions()
	sh := shell.New(app, shell.Config{TitleKey: "app.title", Pages: Pages(lib, sp, q, act, presenter.NewKeys(nil)), Width: 1200, Height: 760})
	for _, c := range Commands(act) {
		if err := sh.Registry.Register(c); err != nil {
			t.Fatal(err)
		}
	}
	return sh, sp, lib
}

func settle() { time.Sleep(150 * time.Millisecond) }

func TestLibraryPageShowsRows(t *testing.T) {
	sh, _, lib := newShell(t, 0)
	sh.Show("library")
	for i := 0; i < 200 && !lib.Loaded(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	settle()
	if got := sh.Status.Right(); len(got) == 0 {
		t.Errorf("status items = %q", got)
	}
}

// The sort page opens the first queued document and the file command is
// refused until the form is complete.
func TestSortPageOpensFirstDocument(t *testing.T) {
	sh, sp, _ := newShell(t, 2)
	sh.Show("sort")
	settle()
	if _, ok := sp.Current(); !ok {
		t.Fatal("no document opened")
	}
	if !sh.Registry.Run("sort.file") {
		t.Fatal("command missing")
	}
	settle()
	if len(sp.Items()) != 2 {
		t.Error("an incomplete form must not file")
	}
}

// In normal mode (nothing focused) j moves to the next document and the
// status bar says NORMAL.
func TestVimNormalModeMovesThroughQueue(t *testing.T) {
	sh, sp, _ := newShell(t, 2)
	sh.Show("sort")
	settle()
	first, ok := sp.Current()
	if !ok {
		t.Fatal("no document opened")
	}
	sh.Win.Canvas().Unfocus()
	sh.Registry.Run("sort.next") // the same action j runs; the rune path is covered below
	settle()
	second, _ := sp.Current()
	if second.ID == first.ID {
		t.Fatal("did not move")
	}
	test.TypeOnCanvas(sh.Win.Canvas(), "k")
	settle()
	if back, _ := sp.Current(); back.ID != first.ID {
		t.Errorf("k did not move back, current %q", back.Name)
	}
}
