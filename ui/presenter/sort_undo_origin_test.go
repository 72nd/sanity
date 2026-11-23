package presenter

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/pdf"
)

// importedBatch imports a.pdf and b.pdf from outside the inbox and analyses them.
func importedBatch(t *testing.T) (p *Sort, root string, srcs []string) {
	t.Helper()
	root = t.TempDir()
	p = newSortIn(t, root)
	dir := filepath.Join(root, "office", "inbox")
	f := pdf.NewFake()
	f.Docs = map[string]pdf.FakeDoc{}
	for i, n := range []string{"a.pdf", "b.pdf"} {
		src := writeDoc(t, filepath.Join(root, "receipts", n))
		srcs = append(srcs, src)
		txt := strings.NewReplacer("12.03.2025", "12.03.2025", "R-88123", "R-8810"+string(rune('1'+i))).Replace(telekomText)
		f.Docs[filepath.Join(dir, n)] = pdf.FakeDoc{Pages: []pdf.PageText{{Text: txt, Source: pdf.SourceLayer}}}
	}
	p.pdf = f
	p.SetAnalyzer(&inbox.Analyzer{PDF: f, Store: p.store, Now: func() time.Time { return time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC) }})
	if !p.Import(srcs...) {
		t.Fatal("not imported")
	}
	done := make(chan string, 2)
	p.Analyse(context.Background(), func(id string) { done <- id })
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("analysis did not finish")
		}
	}
	return p, root, srcs
}

func TestUndoKeepsOriginSoPutBackWorks(t *testing.T) {
	root := t.TempDir()
	src := writeDoc(t, filepath.Join(root, "receipts", "x.pdf"))
	p := newSortIn(t, root)
	p.Import(src)
	p.SetType(telekom(t, p).ID)
	p.SetDate(time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC))
	p.SetInvoice("R-1")
	if _, err := p.File(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Undo(context.Background()); err != nil {
		t.Fatal(err)
	}
	cur := inbox.Item{ID: filepath.Join(root, "office", "inbox", "x.pdf")}
	p.Open(cur.ID)
	if o, ok := p.queue.Origin(cur.ID); !ok || o != src {
		t.Fatalf("origin after undo %q %v", o, ok)
	}
	// After a restart the origin comes from .origins.json.
	q2, err := inbox.OpenQueue(filepath.Join(root, "office", "inbox"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if o, ok := q2.Origin(cur.ID); !ok || o != src {
		t.Fatalf("origin after restart %q %v", o, ok)
	}
	if !p.CanPutBack() {
		t.Fatal("cannot put back after undo")
	}
	if err := p.PutBack(); err != nil {
		t.Fatal(err)
	}
	if !exists(src) {
		t.Error("document not back at its origin")
	}
}

func TestUndoBatchKeepsOrigins(t *testing.T) {
	p, root, srcs := importedBatch(t)
	p.SetSelection([]string{p.Items()[0].ID, p.Items()[1].ID})
	filed, failed := p.FileBatch(context.Background(), telekom(t, p).ID)
	if len(filed) != 2 || len(failed) != 0 {
		t.Fatalf("filed %d failed %v", len(filed), failed)
	}
	if es, err := p.UndoBatch(context.Background()); err != nil || len(es) != 2 {
		t.Fatalf("undo batch %d %v", len(es), err)
	}
	q2, err := inbox.OpenQueue(filepath.Join(root, "office", "inbox"), nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, n := range []string{"a.pdf", "b.pdf"} {
		id := filepath.Join(root, "office", "inbox", n)
		if o, ok := p.queue.Origin(id); !ok || o != srcs[i] {
			t.Errorf("%s origin %q %v", n, o, ok)
		}
		if o, ok := q2.Origin(id); !ok || o != srcs[i] {
			t.Errorf("%s origin after restart %q %v", n, o, ok)
		}
		if _, err := p.queue.PutBack(id); err != nil {
			t.Errorf("%s put back: %v", n, err)
		}
		if !exists(srcs[i]) {
			t.Errorf("%s not at origin", n)
		}
	}
}
