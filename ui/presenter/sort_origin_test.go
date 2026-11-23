package presenter

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

// newSortIn builds a presenter whose queue owns office/inbox of a fresh project.
func newSortIn(t *testing.T, root string) *Sort {
	t.Helper()
	st := store.NewFake()
	q, err := inbox.OpenQueue(filepath.Join(root, "office", "inbox"), nil)
	if err != nil {
		t.Fatal(err)
	}
	p := NewSort(st, q, inbox.NewFiler(root, st), pdf.NewFake(), stubT)
	if err := p.LoadTypes(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p
}

func writeDoc(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("%PDF "+filepath.Base(path)), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestImportMovesToInboxAndPutBackReturns(t *testing.T) {
	root := t.TempDir()
	src := writeDoc(t, filepath.Join(root, "receipts", "2026", "x.pdf"))
	p := newSortIn(t, root)
	if !p.Import(src) {
		t.Fatal("not opened")
	}
	cur, _ := p.Current()
	if exists(src) || filepath.Dir(cur.Path) != filepath.Join(root, "office", "inbox") {
		t.Fatalf("not moved: current %q", cur.Path)
	}
	if !p.CanPutBack() {
		t.Fatal("cannot put back")
	}
	if err := p.PutBack(); err != nil {
		t.Fatal(err)
	}
	if !exists(src) || p.Count() != 0 {
		t.Error("document not back / still queued")
	}
}

func TestPutBackWithoutOriginFails(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, filepath.Join(root, "office", "inbox", "a.pdf"))
	p := newSortIn(t, root)
	p.Step(0)
	if p.CanPutBack() {
		t.Fatal("must not be possible")
	}
	if err := p.PutBack(); err == nil {
		t.Fatal("expected an error")
	}
	if p.Count() != 1 {
		t.Error("document vanished")
	}
}

func TestResortCancelKeepsLibraryEntryValid(t *testing.T) {
	root := t.TempDir()
	lib := writeDoc(t, filepath.Join(root, "receipts", "2026", "x.pdf"))
	p := newSortIn(t, root)
	p.Import(lib) // what Library.Resort triggers through ImportFn
	if err := p.PutBack(); err != nil {
		t.Fatal(err)
	}
	if !exists(lib) {
		t.Error("file is not at its library path again")
	}
}

func TestInboxSurvivesRestartWithOrigins(t *testing.T) {
	root := t.TempDir()
	src := writeDoc(t, filepath.Join(root, "docs", "r.pdf"))
	p := newSortIn(t, root)
	p.Import(src)
	p2 := newSortIn(t, root) // restart
	if p2.Count() != 1 {
		t.Fatalf("count %d", p2.Count())
	}
	p2.Step(0)
	if !p2.CanPutBack() {
		t.Fatal("origin lost over the restart")
	}
	if err := p2.PutBack(); err != nil || !exists(src) {
		t.Fatalf("err %v", err)
	}
}

func TestRemoveForgetsOrigin(t *testing.T) {
	root := t.TempDir()
	p := newSortIn(t, root)
	src := writeDoc(t, filepath.Join(root, "docs", "r.pdf"))
	p.Import(src)
	cur, _ := p.Current()
	os.Remove(cur.Path) // what Trash does, without touching the real trash
	p.queue.Remove(cur.ID)
	if exists(filepath.Join(root, "office", "inbox", inbox.OriginsFile)) {
		t.Error("origin kept for a deleted file")
	}
}

func fillForm(t *testing.T, p *Sort) {
	t.Helper()
	p.SetType(telekom(t, p).ID)
	p.SetDate(time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC))
	p.SetInvoice("R-1")
	p.SetAmount("49.95 EUR")
}

func TestAdoptInPlaceKeepsNameAndAddsEntry(t *testing.T) {
	root := t.TempDir()
	src := writeDoc(t, filepath.Join(root, "belege", "telekom maerz.pdf"))
	p := newSortIn(t, root)
	p.Import(src)
	if !p.CanAdoptInPlace() {
		t.Fatal("cannot adopt")
	}
	if _, err := p.AdoptInPlace(context.Background()); err == nil {
		t.Fatal("type and date are required")
	}
	if !exists(filepath.Join(root, "office", "inbox", "telekom maerz.pdf")) {
		t.Fatal("failed adopt must leave the document in the inbox")
	}
	fillForm(t, p)
	n := len(mustLibrary(t, p))
	e, err := p.AdoptInPlace(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !exists(src) || p.Count() != 0 {
		t.Error("document not back at its origin / still queued")
	}
	if e.Path != "belege/telekom maerz.pdf" || e.Invoice != "R-1" || e.Amount != "49.95 EUR" || e.TypeID != telekom(t, p).ID {
		t.Errorf("entry %+v", e)
	}
	if got := len(mustLibrary(t, p)); got != n+1 {
		t.Errorf("library %d -> %d", n, got)
	}
	if got, _ := p.LastPath(); got != e.Path {
		t.Errorf("LastPath %q", got)
	}
}

func mustLibrary(t *testing.T, p *Sort) []string {
	t.Helper()
	es, err := p.store.Library(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(es))
	for i, e := range es {
		ids[i] = e.ID
	}
	return ids
}

func TestAdoptInPlaceNeedsOriginInsideProject(t *testing.T) {
	root := t.TempDir()
	outside := writeDoc(t, filepath.Join(t.TempDir(), "o.pdf"))
	p := newSortIn(t, root)
	p.Import(outside)
	if p.CanAdoptInPlace() {
		t.Fatal("origin outside the project must not be adoptable")
	}
	fillForm(t, p)
	if _, err := p.AdoptInPlace(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}
