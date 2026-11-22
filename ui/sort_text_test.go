package ui

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"

	"github.com/72nd/fine/widgets"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/store"
)

func entries(root fyne.CanvasObject) map[*widgets.Entry]bool {
	m := map[*widgets.Entry]bool{}
	walk(root, func(o fyne.CanvasObject) {
		if e, ok := o.(*widgets.Entry); ok {
			m[e] = true
		}
	})
	return m
}

func countEntries(root fyne.CanvasObject) int { return len(entries(root)) }

// A type with {text:Project} gets an extra entry in the form; the text is
// asked for, goes into the path, and the entry goes away with the type.
func TestSortFormTextEntry(t *testing.T) {
	st := store.NewFake()
	ty := domain.DocumentType{ID: "t-proj", Name: "Project Invoice",
		PathTemplate: "projects/{text:Project}/{date:2006-01-02}_{invoice}.pdf"}
	if err := st.SaveType(context.Background(), ty); err != nil {
		t.Fatal(err)
	}
	sh, sp, _ := newShellWith(t, 2, st)
	sh.Show("sort")
	settle()
	if _, ok := sp.Current(); !ok {
		t.Fatal("no document opened")
	}
	old := entries(sh.Win.Content())
	before := len(old)
	pick := func(q string) {
		w := lastAct.fields
		w.picker.SetQuery(q)
		if !w.picker.Commit() {
			t.Fatalf("no type for %q", q)
		}
	}
	pick("Project Inv")
	if got := sp.Form().TypeID; got != ty.ID {
		t.Fatalf("type %q", got)
	}
	if after := countEntries(sh.Win.Content()); after != before+1 {
		t.Fatalf("entries %d -> %d, want one more for {text:Project}", before, after)
	}
	sp.SetDate(time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC))
	sp.SetInvoice("R-1")
	if sp.CanFile() {
		t.Fatal("the text is still missing")
	}
	var te *widgets.Entry
	for e := range entries(sh.Win.Content()) {
		if !old[e] {
			te = e
		}
	}
	if te == nil {
		t.Fatal("no new entry")
	}
	te.SetText("Alpha")
	if got := sp.Form().Text["Project"]; got != "Alpha" {
		t.Fatalf("form text %q", got)
	}
	if rel, ok := sp.PathPreview(); !ok || rel != "projects/Alpha/2025-03-12_R-1.pdf" {
		t.Fatalf("preview %q %v", rel, ok)
	}
	if !sh.Registry.Run("sort.file") {
		t.Fatal("command missing")
	}
	settle()
	if p, _ := sp.LastPath(); p != "projects/Alpha/2025-03-12_R-1.pdf" {
		t.Errorf("filed %q", p)
	}
	// the next document starts clean: no text row for a document without a type
	if got := countEntries(sh.Win.Content()); got != before {
		t.Errorf("entries after filing %d, want %d", got, before)
	}
}

// Enter after the last filled field moves on to the empty text entry.
func TestSortFormEnterReachesTextEntry(t *testing.T) {
	st := store.NewFake()
	ty := domain.DocumentType{ID: "t-proj", Name: "Project Invoice",
		PathTemplate: "projects/{text:Project}/{date:2006-01-02}_{invoice}.pdf"}
	if err := st.SaveType(context.Background(), ty); err != nil {
		t.Fatal(err)
	}
	sh, sp, _ := newShellWith(t, 1, st)
	sh.Show("sort")
	settle()
	sp.SetDate(time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC))
	sp.SetInvoice("R-1")
	sp.SetAmount("1.00 EUR")
	old := entries(sh.Win.Content())
	w := lastAct.fields
	w.picker.SetQuery("Project Inv")
	w.picker.Commit() // Enter in the type field: the next empty field is the text entry
	var te *widgets.Entry
	for e := range entries(sh.Win.Content()) {
		if !old[e] {
			te = e
		}
	}
	if te == nil {
		t.Fatal("no text entry")
	}
	if f, ok := sh.Win.Canvas().Focused().(*widgets.Entry); !ok || f != te {
		t.Errorf("focus is %T, want the text entry", sh.Win.Canvas().Focused())
	}
}
