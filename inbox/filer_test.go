package inbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/store"
)

func newFiler(t *testing.T) (*Filer, string, store.Store) {
	t.Helper()
	root := t.TempDir()
	st := store.NewFake()
	f := NewFiler(root, st)
	return f, root, st
}

func writePDF(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("%PDF-1.4 "+name), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func req(path string) Request {
	return Request{
		Path:   path,
		Type:   domain.DocumentType{ID: "tel", Name: "Telekom", PathTemplate: "receipts/{date:2006}/{date:2006-01-02}_telekom_{invoice}.pdf"},
		Values: domain.Values{Date: time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC), Invoice: "R-88123"},
	}
}

func TestFileMovesAndUndoRestores(t *testing.T) {
	f, root, st := newFiler(t)
	src := writePDF(t, filepath.Join(root, "office", "inbox"), "a.pdf")
	before, _ := st.Library(context.Background())

	e, err := f.File(context.Background(), req(src))
	if err != nil {
		t.Fatal(err)
	}
	if e.Path != "receipts/2025/2025-03-12_telekom_R-88123.pdf" {
		t.Errorf("path %q", e.Path)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("source still exists after filing")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(e.Path))); err != nil {
		t.Error(err)
	}
	after, _ := st.Library(context.Background())
	if len(after) != len(before)+1 {
		t.Fatalf("library %d -> %d", len(before), len(after))
	}
	if !f.CanUndo() {
		t.Fatal("cannot undo")
	}

	if _, err := f.UndoLast(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Error("original not restored")
	}
	if _, err := os.Stat(filepath.Join(root, "receipts")); !os.IsNotExist(err) {
		t.Error("created folders were not removed")
	}
	again, _ := st.Library(context.Background())
	if len(again) != len(before) {
		t.Errorf("library not restored: %d", len(again))
	}
	if f.CanUndo() {
		t.Error("undo stack not empty")
	}
}

func TestFileCollisionGetsSuffix(t *testing.T) {
	f, root, _ := newFiler(t)
	in := filepath.Join(root, "in")
	e1, err := f.File(context.Background(), req(writePDF(t, in, "a.pdf")))
	if err != nil {
		t.Fatal(err)
	}
	e2, err := f.File(context.Background(), req(writePDF(t, in, "b.pdf")))
	if err != nil {
		t.Fatal(err)
	}
	if e2.Path == e1.Path || e2.Path != "receipts/2025/2025-03-12_telekom_R-88123_2.pdf" {
		t.Errorf("paths %q %q", e1.Path, e2.Path)
	}
}

func TestFileMissingValueIsError(t *testing.T) {
	f, root, _ := newFiler(t)
	src := writePDF(t, filepath.Join(root, "in"), "a.pdf")
	r := req(src)
	r.Values.Invoice = ""
	if _, err := f.File(context.Background(), r); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(src); err != nil {
		t.Error("source must stay when filing fails")
	}
}

func TestUndoNothing(t *testing.T) {
	f, _, _ := newFiler(t)
	if _, err := f.UndoLast(context.Background()); err == nil {
		t.Error("expected error")
	}
}
