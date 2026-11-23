package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/locales"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
	"github.com/72nd/sanity/ui/presenter"
)

func focusedKind(sh *shell.Shell) string { return fmt.Sprintf("%T", sh.Win.Canvas().Focused()) }

// Enter goes to the next field even when it is filled; the step helper (what
// Shift+Enter calls) goes back and stops at both ends.
func TestSortFocusChainForwardAndBack(t *testing.T) {
	sh, sp, _ := newShell(t, 1)
	sh.Show("sort")
	settle()
	sp.SetDate(time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC))
	sp.SetInvoice("R-1")
	sp.SetAmount("1.00 EUR")
	w := lastAct.fields
	w.date.OnSubmitted(nil)
	if sh.Win.Canvas().Focused() != nil && sh.Win.Canvas().Focused() != any(w.invoice) {
		t.Errorf("date + Enter: focus %s, want invoice", focusedKind(sh))
	}
	w.invoice.OnSubmitted("")
	if sh.Win.Canvas().Focused() != any(w.amount) {
		t.Errorf("invoice + Enter: focus %s, want amount", focusedKind(sh))
	}
	w.step("amount", -1)
	if sh.Win.Canvas().Focused() != any(w.invoice) {
		t.Errorf("back from amount: focus %s, want invoice", focusedKind(sh))
	}
	w.step("invoice", -1)
	if got := focusedKind(sh); got != "*widgets.dateField" {
		t.Errorf("back from invoice: focus %s, want date", got)
	}
	w.step("date", -1)
	w.step("type", -1)  // stops at the start
	w.step("amount", 1) // and at the end (no text rows)
	if sh.Win.Canvas().Focused() != any(w.amount) && sh.Win.Canvas().Focused() == nil {
		t.Error("focus lost at the end of the chain")
	}
}

// Cmd++ in each variant, Cmd+= and Cmd+- zoom the preview.
func TestSortZoomShortcuts(t *testing.T) {
	act := NewSortActions()
	in, out := 0, 0
	act.ZoomIn, act.ZoomOut = func() { in++ }, func() { out++ }
	want := map[string]bool{"+": false, "=": false, "-": false}
	for _, c := range Commands(act) {
		if c.Shortcut == nil || c.Shortcut.Modifier&fyne.KeyModifierShortcutDefault == 0 {
			continue
		}
		switch {
		case strings.HasPrefix(c.ID, "sort.zoomin") && (c.Shortcut.KeyName == fyne.KeyPlus || c.Shortcut.KeyName == fyne.KeyEqual):
			want[string(c.Shortcut.KeyName)] = true
			c.Run()
		case c.ID == "sort.zoomout" && c.Shortcut.KeyName == fyne.KeyMinus:
			want["-"] = true
			c.Run()
		}
	}
	for k, ok := range want {
		if !ok {
			t.Errorf("no zoom shortcut on %q", k)
		}
	}
	if in != 3 || out != 1 {
		t.Errorf("zoom in %d (want 3), out %d (want 1)", in, out)
	}
	var shifted bool
	for _, c := range Commands(act) {
		if c.ID == "sort.zoomin.shift" && c.Shortcut.Modifier&fyne.KeyModifierShift != 0 {
			shifted = true
		}
	}
	if !shifted {
		t.Error("Cmd+Shift+= (Cmd++ on a US layout) missing")
	}
}

// The new commands have shortcuts that collide with nothing else: the shell
// registers them with the registry, which rejects duplicates.
func TestSortPutBackCommandsRegistered(t *testing.T) {
	sh, _, _ := newShell(t, 0)
	for _, id := range []string{"sort.putback", "sort.adopt", "sort.remove"} {
		found := false
		for _, c := range sh.Registry.Commands() {
			if c.ID == id {
				found = c.Title() != "" && !strings.HasPrefix(c.Title(), "cmd.")
			}
		}
		if !found {
			t.Errorf("command %s missing or untitled", id)
		}
	}
}

// importShell is newShell with an inbox folder set, so imports move files
// (and remember their origin). It returns the project root.
func importShell(t *testing.T) (*shell.Shell, *presenter.Sort, string) {
	t.Helper()
	app := test.NewTempApp(t)
	i18n.MustAddFiles(locales.FS, locales.Files...)
	i18n.SetLanguage("en")
	st := store.NewFake()
	root := t.TempDir()
	dir := filepath.Join(root, "office", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	q := inbox.NewQueue(nil)
	if err := q.SetDir(dir); err != nil {
		t.Fatal(err)
	}
	lib := presenter.NewLibrary(st, i18n.T)
	if err := lib.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	sp := presenter.NewSort(st, q, inbox.NewFiler(root, st), pdf.NewFake(), i18n.T)
	act := NewSortActions()
	lastAct = act
	sh := shell.New(app, shell.Config{TitleKey: "app.title", Pages: Pages(lib, sp, q, act, presenter.NewKeys(nil), presenter.NewTypes(st, i18n.T)), Width: 1200, Height: 760})
	return sh, sp, root
}

func writePDF(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// An imported document goes back where it came from; with an empty queue the
// action does nothing.
func TestSortPutBack(t *testing.T) {
	sh, sp, _ := importShell(t)
	sh.Show("sort")
	settle()
	lastAct.PutBack() // nothing open: no-op
	src := writePDF(t, filepath.Join(t.TempDir(), "elsewhere.pdf"))
	lastAct.Import(src)
	settle()
	if _, err := os.Stat(src); err == nil {
		t.Fatal("import must move the file into the inbox")
	}
	if !sp.CanPutBack() || sp.Count() != 1 {
		t.Fatalf("imported document must be put-back-able (count %d)", sp.Count())
	}
	lastAct.PutBack()
	settle()
	if _, err := os.Stat(src); err != nil {
		t.Errorf("file not back at its origin: %v", err)
	}
	if sp.Count() != 0 {
		t.Errorf("queue %d after put back, want 0", sp.Count())
	}
}

// A document that already lay in the inbox cannot be put back: the action
// refuses and the document stays.
func TestSortPutBackWithoutOrigin(t *testing.T) {
	sh, sp, _ := newShell(t, 1)
	sh.Show("sort")
	settle()
	if sp.CanPutBack() {
		t.Fatal("a document that lay in the inbox has no origin")
	}
	lastAct.PutBack()
	settle()
	if sp.Count() != 1 {
		t.Errorf("queue %d after refused put back, want 1", sp.Count())
	}
}

// Delete asks first: until the dialog is answered the file stays.
func TestSortDeleteAsksFirst(t *testing.T) {
	sh, sp, _ := importShell(t)
	sh.Show("sort")
	settle()
	src := writePDF(t, filepath.Join(t.TempDir(), "gone.pdf"))
	lastAct.Import(src)
	settle()
	cur, ok := sp.Current()
	if !ok {
		t.Fatal("no document")
	}
	lastAct.Remove()
	settle()
	if _, err := os.Stat(cur.Path); err != nil {
		t.Errorf("deleted without confirmation: %v", err)
	}
}

// "Adopt in place" is refused for a document from outside the project and
// works for one that came from inside it.
func TestSortAdoptInPlace(t *testing.T) {
	sh, sp, root := importShell(t)
	sh.Show("sort")
	settle()
	outside := writePDF(t, filepath.Join(t.TempDir(), "o.pdf"))
	lastAct.Import(outside)
	settle()
	if sp.CanAdoptInPlace() {
		t.Fatal("origin outside the project cannot be adopted")
	}
	lastAct.AdoptInPlace()
	settle()
	if sp.Count() != 1 {
		t.Fatalf("refused adopt changed the queue: %d", sp.Count())
	}
	inside := writePDF(t, filepath.Join(root, "docs", "i.pdf"))
	lastAct.Import(inside)
	settle()
	if !sp.CanAdoptInPlace() {
		t.Fatal("origin inside the project must be adoptable")
	}
	types := sp.Types()
	if len(types) == 0 {
		t.Fatal("no types")
	}
	sp.SetType(types[0].ID)
	sp.SetDate(time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC))
	lastAct.AdoptInPlace()
	settle()
	if _, err := os.Stat(inside); err != nil {
		t.Errorf("adopted file must lie at its origin: %v", err)
	}
}

// The Vim keys p (put back) and a (adopt) are bound; dd (delete) stays.
func TestSortVimPutBackKeys(t *testing.T) {
	k := presenter.NewKeys(nil)
	for r, want := range map[rune]string{'p': "putback", 'a': "adopt"} {
		if cmd, _ := k.Feed(presenter.ContextSort, r); cmd != want {
			t.Errorf("%c = %q, want %q", r, cmd, want)
		}
	}
	k.Feed(presenter.ContextSort, 'd')
	if cmd, _ := k.Feed(presenter.ContextSort, 'd'); cmd != "remove" {
		t.Errorf("dd = %q, want remove", cmd)
	}
}

// A type saved on the Types page is offered on the sort page the next time it
// is shown, without a restart.
func TestSortReloadsTypesOnShow(t *testing.T) {
	st := store.NewFake()
	sh, sp, _ := newShellWith(t, 1, st)
	sh.Show("sort")
	settle()
	before := len(sp.Types())
	if before == 0 {
		t.Fatal("no types loaded")
	}
	sh.Show("types")
	settle()
	err := st.SaveType(context.Background(), domain.DocumentType{ID: "t-new", Name: "Brand New", PathTemplate: "new/{date:2006-01-02}.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	sh.Show("sort")
	settle()
	if got := len(sp.Types()); got != before+1 {
		t.Errorf("types after show %d, want %d", got, before+1)
	}
}
