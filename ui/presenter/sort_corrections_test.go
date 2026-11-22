package presenter

import (
	"context"
	"testing"
	"time"

	"github.com/72nd/sanity/inbox"
)

func collect(p *Sort) *[]inbox.Correction {
	var got []inbox.Correction
	p.OnCorrection = func(c inbox.Correction) { got = append(got, c) }
	return &got
}

func TestFilingLogsSuggestionVersusChoice(t *testing.T) {
	p := batchSort(t)
	got := collect(p)
	ty := telekom(t, p).ID
	p.Step(0)
	if p.Form().TypeID != ty {
		t.Fatalf("analysis did not preselect the type: %+v", p.Form())
	}
	if _, err := p.File(context.Background()); err != nil { // as suggested
		t.Fatal(err)
	}
	if p.Form().TypeID != ty {
		t.Fatalf("next document not prefilled: %+v", p.Form())
	}
	p.SetDate(time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC)) // overrides the date b.pdf suggests
	if _, err := p.File(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 {
		t.Fatalf("%d lines, want 2", len(*got))
	}
	c0, c1 := (*got)[0], (*got)[1]
	if c0.TypeID != ty || c0.Suggested.TypeID != ty || !c0.Suggested.Confident || c0.Suggested.Score <= 0 {
		t.Errorf("first %+v", c0)
	}
	if c0.DateSuggested != "2025-03-12" || c0.DateChosen != c0.DateSuggested || c0.InvoiceSuggested != "R-88101" || c0.SHA256 == "" || len(c0.Overrides) != 0 {
		t.Errorf("first %+v", c0)
	}
	if c1.DateSuggested != "2025-04-14" || c1.DateChosen != "2025-04-15" || len(c1.Overrides) != 1 || c1.Overrides[0] != "date" {
		t.Errorf("second %+v", c1)
	}
	// undo appends an undone line with the same hash and deletes nothing
	if _, err := p.Undo(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(*got); n != 3 || !(*got)[2].Undone || (*got)[2].SHA256 != c1.SHA256 {
		t.Errorf("after undo: %+v", *got)
	}
}

func TestBatchLogsEveryDocumentAndUndo(t *testing.T) {
	p := batchSort(t)
	got := collect(p)
	ty := telekom(t, p).ID
	p.SetSelection(ids(p, 0, 1))
	filed, failed := p.FileBatch(context.Background(), ty)
	if len(filed) != 2 || len(failed) != 0 {
		t.Fatalf("filed %d failed %+v", len(filed), failed)
	}
	if len(*got) != 2 || (*got)[0].SHA256 == (*got)[1].SHA256 {
		t.Fatalf("lines %+v", *got)
	}
	if (*got)[0].InvoiceChosen != "R-88101" || (*got)[1].DateSuggested != "2025-04-14" || (*got)[1].TypeID != ty {
		t.Errorf("each document uses its own analysis: %+v", *got)
	}
	if _, err := p.UndoBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 4 || !(*got)[2].Undone || !(*got)[3].Undone {
		t.Errorf("undo lines: %+v", *got)
	}
	if r := inbox.Stats(*got); r.Total != 0 {
		t.Errorf("undone filings still counted: %+v", r)
	}
}
