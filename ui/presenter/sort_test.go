package presenter

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

func newSort(t *testing.T, n int) (*Sort, string) {
	t.Helper()
	root := t.TempDir()
	st := store.NewFake()
	q := inbox.NewQueue(nil)
	dir := filepath.Join(root, "office", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		name := string(rune('a'+i)) + ".pdf"
		if err := os.WriteFile(filepath.Join(dir, name), []byte("%PDF "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	q.Add(dir)
	p := NewSort(st, q, inbox.NewFiler(root, st), pdf.NewFake(), stubT)
	if err := p.LoadTypes(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p, root
}

func telekom(t *testing.T, p *Sort) domain.DocumentType {
	t.Helper()
	for _, ty := range p.Types() {
		if ty.Name == "Telekom Invoice" {
			return ty
		}
	}
	t.Fatal("no Telekom type in the fake")
	return domain.DocumentType{}
}

func TestSortFlowFilesAndAdvances(t *testing.T) {
	p, root := newSort(t, 2)
	if !p.Step(0) {
		t.Fatal("nothing opened")
	}
	first, _ := p.Current()
	if p.CanFile() {
		t.Fatal("must not file without a type")
	}
	p.SetType(telekom(t, p).ID)
	p.SetDate(time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC))
	if p.CanFile() {
		t.Fatalf("invoice is missing: %v", p.Missing())
	}
	if m := p.Missing(); len(m) != 1 || m[0] != "invoice" {
		t.Fatalf("missing %v", m)
	}
	p.SetInvoice("R-88123")
	rel, ok := p.PathPreview()
	if !ok {
		t.Fatal("no preview")
	}
	e, err := p.File(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if e.Path != rel {
		t.Errorf("filed %q, previewed %q", e.Path, rel)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
		t.Error(err)
	}
	cur, ok := p.Current()
	if !ok || cur.ID == first.ID {
		t.Error("next document not opened")
	}
	if p.Form().TypeID != "" {
		t.Error("form not reset")
	}
	if got, _ := p.LastPath(); got != rel {
		t.Errorf("LastPath %q", got)
	}
}

func TestSortUndoRequeues(t *testing.T) {
	p, _ := newSort(t, 1)
	p.Step(0)
	cur, _ := p.Current()
	p.SetType(telekom(t, p).ID)
	p.SetDate(time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC))
	p.SetInvoice("R-1")
	if _, err := p.File(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Current(); ok {
		t.Fatal("queue should be empty")
	}
	if !p.CanUndo() {
		t.Fatal("cannot undo")
	}
	if _, err := p.Undo(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.queue.Get(cur.ID); !ok {
		t.Error("document not back in queue")
	}
}

func TestStepWraps(t *testing.T) {
	p, _ := newSort(t, 3)
	p.Step(0)
	a, _ := p.Current()
	p.Step(-1)
	c, _ := p.Current()
	if a.ID == c.ID {
		t.Fatal("did not move")
	}
	p.Step(1)
	if b, _ := p.Current(); b.ID != a.ID {
		t.Error("wrap failed")
	}
}

func TestImportOpensTheNewDocument(t *testing.T) {
	p, root := newSort(t, 2)
	p.Step(0)
	before, _ := p.Current()
	extra := filepath.Join(root, "other", "z.pdf")
	if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(extra, []byte("%PDF z"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !p.Import(extra) {
		t.Fatal("new document not opened")
	}
	cur, _ := p.Current()
	if cur.Path != extra || cur.ID == before.ID {
		t.Errorf("current %q, want the added file", cur.Path)
	}
	if p.Import(extra) {
		t.Error("importing a known file must not reopen it")
	}
	if cur2, _ := p.Current(); cur2.Path != extra {
		t.Error("current changed on a no-op import")
	}
}

const telekomText = `Telekom Deutschland GmbH
Rechnung
Rechnungsdatum 12.03.2025
Rechnungsnummer R-88123
Gesamtbetrag 49,95 EUR`

func TestAnalysisPrefillsForm(t *testing.T) {
	p, _ := newSort(t, 1)
	p.pdf = pdf.NewFake(telekomText)
	p.SetAnalyzer(&inbox.Analyzer{
		PDF: p.pdf, Store: p.store,
		Now: func() time.Time { return time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC) },
	})
	p.Step(0)
	cur, _ := p.Current()
	done := make(chan string, 1)
	p.Analyse(context.Background(), func(id string) { done <- id })
	select {
	case id := <-done:
		if id != cur.ID {
			t.Fatalf("analysed %q", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("analysis did not finish")
	}
	if !p.Analysed() {
		t.Fatal("no result")
	}
	p.Prefill()
	f := p.Form()
	if f.Invoice != "R-88123" || f.Amount != "49.95 EUR" {
		t.Errorf("form %+v", f)
	}
	if !f.Date.Equal(time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("date %v", f.Date)
	}
	if len(p.DateChoices()) == 0 {
		t.Error("no date choices")
	}
	if len(p.TypeChoices()) != len(p.Types()) {
		t.Error("type choices incomplete")
	}
	// a second Analyse must not start the same document again
	p.Analyse(context.Background(), func(string) { t.Error("analysed twice") })
	time.Sleep(50 * time.Millisecond)
}

func TestPrefillNeverOverwritesUserInput(t *testing.T) {
	p, _ := newSort(t, 1)
	p.pdf = pdf.NewFake(telekomText)
	p.SetAnalyzer(&inbox.Analyzer{PDF: p.pdf, Store: p.store})
	p.Step(0)
	p.SetInvoice("MINE-1")
	done := make(chan string, 1)
	p.Analyse(context.Background(), func(id string) { done <- id })
	<-done
	p.Prefill()
	if p.Form().Invoice != "MINE-1" {
		t.Errorf("invoice overwritten: %q", p.Form().Invoice)
	}
}

func TestViewStatePerDocument(t *testing.T) {
	p, _ := newSort(t, 2)
	items := p.Items()
	if v := p.View(items[0].ID); v.Zoom != 1 || v.X != 0 || v.Y != 0 {
		t.Errorf("default view %+v", v)
	}
	p.SaveView(items[0].ID, ViewState{Zoom: 2.5, Y: 300})
	if v := p.View(items[0].ID); v.Zoom != 2.5 || v.Y != 300 {
		t.Errorf("saved view %+v", v)
	}
	if v := p.View(items[1].ID); v.Zoom != 1 {
		t.Errorf("view leaked to another document: %+v", v)
	}
	p.SaveView("", ViewState{Zoom: 9}) // no document: ignored
}
