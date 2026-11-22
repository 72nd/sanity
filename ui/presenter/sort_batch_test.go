package presenter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/pdf"
)

// batchSort builds a presenter whose a.pdf and b.pdf are Telekom invoices with
// their own date and number, and whose c.pdf has no date at all.
func batchSort(t *testing.T) *Sort {
	t.Helper()
	p, root := newSort(t, 3)
	dir := filepath.Join(root, "office", "inbox")
	mk := func(day, no string) pdf.FakeDoc {
		txt := strings.NewReplacer("12.03.2025", day, "R-88123", no).Replace(telekomText)
		return pdf.FakeDoc{Pages: []pdf.PageText{{Text: txt, Source: pdf.SourceLayer}}}
	}
	f := pdf.NewFake()
	f.Docs = map[string]pdf.FakeDoc{
		filepath.Join(dir, "a.pdf"): mk("12.03.2025", "R-88101"),
		filepath.Join(dir, "b.pdf"): mk("14.04.2025", "R-88102"),
		filepath.Join(dir, "c.pdf"): {Pages: []pdf.PageText{{Text: "Telekom Deutschland GmbH Rechnung", Source: pdf.SourceLayer}}},
	}
	p.pdf = f
	p.SetAnalyzer(&inbox.Analyzer{PDF: f, Store: p.store, Now: func() time.Time { return time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC) }})
	done := make(chan string, 3)
	p.Analyse(context.Background(), func(id string) { done <- id })
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("analysis did not finish")
		}
	}
	return p
}

func ids(p *Sort, n ...int) []string {
	var out []string
	for _, i := range n {
		out = append(out, p.Items()[i].ID)
	}
	return out
}

func TestBatchPlanUsesEachDocumentsAnalysis(t *testing.T) {
	p := batchSort(t)
	ty := telekom(t, p).ID
	p.SetSelection(append(ids(p, 1, 0), "/not/in/queue.pdf"))
	if sel := p.Selection(); len(sel) != 2 || sel[0] != p.Items()[0].ID {
		t.Fatalf("selection %v (queue order, unknown dropped)", sel)
	}
	plan := p.BatchPlan(ty)
	if len(plan) != 2 || plan[0].Path != "receipts/2025/telecom/2025-03-12_telekom_R-88101.pdf" || plan[1].Path != "receipts/2025/telecom/2025-04-14_telekom_R-88102.pdf" {
		t.Fatalf("plan %+v", plan)
	}
	if !p.CanBatch(ty) || !p.BatchActive() {
		t.Error("batch should be ready")
	}
	if p.CanBatch("") {
		t.Error("no type, no batch")
	}

	p.SetSelection(ids(p, 0, 2))
	plan = p.BatchPlan(ty)
	if !plan[1].Blocked() || len(plan[1].Missing) == 0 || !strings.HasPrefix(plan[1].Missing[0], "date") {
		t.Errorf("blocked item %+v", plan[1])
	}
	if p.CanBatch(ty) {
		t.Error("a document without date blocks the batch")
	}
	p.SetSelection(ids(p, 0))
	if p.CanBatch(ty) || p.BatchActive() {
		t.Error("one document is not a batch")
	}
}

func TestFileBatchUndoOneByOneAndTogether(t *testing.T) {
	for _, together := range []bool{false, true} {
		p := batchSort(t)
		ty := telekom(t, p).ID
		srcA, srcB := p.Items()[0].Path, p.Items()[1].Path
		p.SetSelection(ids(p, 0, 1))
		filed, failed := p.FileBatch(context.Background(), ty)
		if len(filed) != 2 || len(failed) != 0 {
			t.Fatalf("filed %d failed %v", len(filed), failed)
		}
		if p.Count() != 1 {
			t.Fatalf("queue %d, want only c.pdf", p.Count())
		}
		if cur, ok := p.Current(); !ok || cur.Name != "c.pdf" {
			t.Errorf("next document not opened: %+v", cur)
		}
		if n := p.LastBatchSize(); n != 2 {
			t.Errorf("batch size %d", n)
		}
		if together {
			es, err := p.UndoBatch(context.Background())
			if err != nil || len(es) != 2 {
				t.Fatalf("undo batch: %d %v", len(es), err)
			}
		} else {
			for i := 0; i < 2; i++ {
				if _, err := p.Undo(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
		}
		if p.CanUndo() || p.Count() != 3 {
			t.Errorf("together=%v: queue %d canUndo %v", together, p.Count(), p.CanUndo())
		}
		for _, s := range []string{srcA, srcB} {
			if _, err := os.Stat(s); err != nil {
				t.Errorf("not restored: %v", err)
			}
		}
	}
}

func TestFileBatchContinuesAfterFailure(t *testing.T) {
	p := batchSort(t)
	ty := telekom(t, p).ID
	p.SetSelection(ids(p, 0, 1, 2))
	if p.CanBatch(ty) {
		t.Fatal("c.pdf has no date")
	}
	os.Remove(p.Items()[1].Path) // b.pdf vanished: hashing fails
	filed, failed := p.FileBatch(context.Background(), ty)
	if len(filed) != 1 || len(failed) != 2 {
		t.Fatalf("filed %d failed %+v", len(filed), failed)
	}
	if failed[0].Name != "b.pdf" || failed[1].Name != "c.pdf" || failed[1].Err == nil {
		t.Errorf("failures %+v", failed)
	}
	if sel := p.Selection(); len(sel) != 2 {
		t.Errorf("failed documents stay selected: %v", sel)
	}
}
