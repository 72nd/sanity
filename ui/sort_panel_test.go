package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

// The side panel never asks for more than 340 px, however long the duplicate
// path or the target path is, and File follows the form state.
func TestSortPanelWidthAndFileButton(t *testing.T) {
	st := store.NewFake()
	sh, sp, _ := newShellWith(t, 1, st)
	sum, err := inbox.FileHash(sp.Items()[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	long := "receipts/2025/telecom/" + strings.Repeat("2025-03-12_telekom_R-88123456789012_", 4) + "old-copy.pdf"
	dup := domain.LibraryEntry{ID: "dup-1", TypeID: "x", Path: long, SHA256: sum, Date: time.Now(), FiledAt: time.Now()}
	if err := st.AddEntry(context.Background(), dup); err != nil {
		t.Fatal(err)
	}
	sp.SetAnalyzer(&inbox.Analyzer{PDF: pdf.NewFake("Some text"), Store: st})
	done := make(chan string, 1)
	sp.Analyse(context.Background(), func(id string) { done <- id })
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("analysis did not finish")
	}
	sh.Show("sort")
	settle()
	pnl := lastAct.fields.panel
	if _, _, ok := sp.Duplicate(); !ok || !pnl.dup.Visible() {
		t.Fatal("duplicate callout not shown")
	}
	if w := pnl.root.MinSize().Width; w > 340 {
		t.Errorf("min width with a long duplicate path = %v, want <= 340", w)
	}
	if pnl.fileBtn.Disabled() == false {
		t.Error("File enabled without a type")
	}

	sp.SetType(sp.Types()[0].ID)
	pnl.refreshSingle()
	if !pnl.fileBtn.Disabled() {
		t.Error("File enabled while fields are missing")
	}
	sp.SetDate(time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC))
	sp.SetInvoice(strings.Repeat("R-0123456789-", 6))
	pnl.refreshSingle()
	if pnl.fileBtn.Disabled() {
		t.Errorf("File disabled although complete (missing %v)", sp.Missing())
	}
	if rel, _ := sp.PathPreview(); !strings.Contains(pnl.pathLabel.Text, rel) || len(rel) < 80 {
		t.Errorf("path label %q, preview %q", pnl.pathLabel.Text, rel)
	}
	t.Logf("panel min width %v", pnl.root.MinSize().Width)
	if w := pnl.root.MinSize().Width; w > 340 {
		t.Errorf("min width with a long target path = %v, want <= 340", w)
	}
}

// The more menu calls the page's actions.
func TestSortPanelMoreMenuActions(t *testing.T) {
	sh, _, _ := newShell(t, 1)
	sh.Show("sort")
	settle()
	got := map[string]int{}
	act := lastAct
	act.PutBack = func() { got["putback"]++ }
	act.AdoptInPlace = func() { got["adopt"]++ }
	act.Remove = func() { got["remove"]++ }
	act.LearnRules = func() { got["learn"]++ }
	items := act.fields.panel.moreItems()
	n := 0
	for _, it := range items {
		if it.Separator {
			continue
		}
		n++
		it.Action()
	}
	if n != 4 {
		t.Fatalf("%d menu actions, want 4", n)
	}
	for _, k := range []string{"putback", "adopt", "remove", "learn"} {
		if got[k] != 1 {
			t.Errorf("%s called %d times", k, got[k])
		}
	}
	last := items[len(items)-1]
	if !last.Danger || last.Icon == "" {
		t.Error("delete entry must be danger with an icon")
	}
}
