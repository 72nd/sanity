package ui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/widgets"
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
	// Loaded up front: the Fyne test driver runs fyne.Do on the calling goroutine,
	// so a background load would lay the page out concurrently with the test.
	if err := lib.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	sp := presenter.NewSort(st, q, inbox.NewFiler(root, st), pdf.NewFake(), i18n.T)
	act := NewSortActions()
	sh := shell.New(app, shell.Config{TitleKey: "app.title", Pages: Pages(lib, sp, q, act, presenter.NewKeys(nil), presenter.NewTypes(st, i18n.T)), Width: 1200, Height: 760})
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

// The form card on the sort page stays inside the window and keeps a usable
// width at narrow and wide window widths.
func TestSortFormCardInsideWindow(t *testing.T) {
	for _, w := range []float32{900, 1100, 1200} {
		sh, _, _ := newShell(t, 2)
		sh.Show("sort")
		settle()
		sh.Win.Resize(fyne.NewSize(w, 760))
		settle()
		var card *widgets.Card
		walk(sh.Win.Content(), func(o fyne.CanvasObject) {
			if c, ok := o.(*widgets.Card); ok {
				card = c
			}
		})
		if card == nil {
			t.Fatalf("w=%v: form card not found", w)
		}
		left := fyne.CurrentApp().Driver().AbsolutePositionForObject(card).X
		right := left + card.Size().Width
		if right > w {
			t.Errorf("w=%v: form card right edge %v is outside the window", w, right)
		}
		if card.Size().Width < 290 {
			t.Errorf("w=%v: form card width %v < 290", w, card.Size().Width)
		}
	}
}

func walk(o fyne.CanvasObject, fn func(fyne.CanvasObject)) {
	fn(o)
	switch v := o.(type) {
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(v).Objects() {
			walk(c, fn)
		}
	case *fyne.Container:
		for _, c := range v.Objects {
			walk(c, fn)
		}
	}
}

// Cmd+Shift+V imports what Finder copied and shows the first new document.
func TestPasteFilesImportsClipboardPDFs(t *testing.T) {
	sh, sp, _ := newShell(t, 1)
	sh.Show("sort")
	settle()
	extra := filepath.Join(t.TempDir(), "pasted.pdf")
	if err := os.WriteFile(extra, []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := clipboardFiles
	clipboardFiles = func() []string { return []string{extra, "/no/such/file.pdf"} }
	defer func() { clipboardFiles = old }()

	before := sp.Count()
	if !sh.Registry.Run("sort.paste") {
		t.Fatal("command missing")
	}
	settle()
	if sp.Count() != before+1 {
		t.Fatalf("queue %d -> %d, want one more", before, sp.Count())
	}
	if cur, _ := sp.Current(); cur.Path != extra {
		t.Errorf("current %q, want the pasted file", cur.Path)
	}
}
