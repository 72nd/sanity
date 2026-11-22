package ui

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"

	"github.com/72nd/fine/widgets"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

// The duplicate banner names the reason in words and its button opens the
// library with only that entry visible and selected.
func TestDuplicateBannerShowsInLibrary(t *testing.T) {
	st := store.NewFake()
	sh, sp, lib := newShellWith(t, 1, st)
	first := sp.Items()[0]
	sum, err := inbox.FileHash(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	dup := domain.LibraryEntry{ID: "dup-1", TypeID: "x", Path: "receipts/2025/old copy.pdf", SHA256: sum, Date: time.Now(), FiledAt: time.Now()}
	if err := st.AddEntry(context.Background(), dup); err != nil {
		t.Fatal(err)
	}
	// Analyse before the page exists: the test driver runs fyne.Do inline, so a
	// worker goroutine must not lay the page out meanwhile.
	sp.SetAnalyzer(&inbox.Analyzer{PDF: pdf.NewFake("Some text"), Store: st})
	done := make(chan string, 1)
	sp.Analyse(context.Background(), func(id string) { done <- id })
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("analysis did not finish")
	}
	if err := lib.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	sh.Show("sort")
	settle()
	if _, why, ok := sp.Duplicate(); !ok || sp.DuplicateReasonText(why) != "same file" {
		t.Fatalf("duplicate not found: %q %v", why, ok)
	}
	var btn *widgets.Button
	walk(sh.Win.Content(), func(o fyne.CanvasObject) {
		if b, ok := o.(*widgets.Button); ok && b.Label == "Show in library" {
			btn = b
		}
	})
	if btn == nil {
		t.Fatal("no 'Show in library' button")
	}
	btn.Tapped(nil)
	settle()
	if sh.Current() != "library" {
		t.Fatalf("page %q, want library", sh.Current())
	}
	if lib.Query != dup.Path || len(lib.SelKeys) != 1 || lib.SelKeys[0] != dup.ID {
		t.Errorf("query %q sel %v", lib.Query, lib.SelKeys)
	}
	if rows := lib.Visible(lib.Query); len(rows) != 1 || rows[0].Entry.ID != dup.ID {
		t.Errorf("visible rows %v", rows)
	}
}
