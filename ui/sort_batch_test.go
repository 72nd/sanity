package ui

import (
	"context"
	"os"
	"testing"
	"time"

	"fyne.io/fyne/v2"

	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/fine/widgets/grid"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

const batchText = `Telekom Deutschland GmbH
Rechnung
Rechnungsdatum 12.03.2025
Rechnungsnummer R-88123
Gesamtbetrag 49,95 EUR`

func queueGrid(t *testing.T, root fyne.CanvasObject) *grid.Grid[inbox.Item] {
	t.Helper()
	var g *grid.Grid[inbox.Item]
	walk(root, func(o fyne.CanvasObject) {
		if v, ok := o.(*grid.Grid[inbox.Item]); ok {
			g = v
		}
	})
	if g == nil {
		t.Fatal("queue grid not found")
	}
	return g
}

// Two selected documents are filed with one command; both reach the library,
// the queue is empty and undo restores them, one by one and as a batch.
func TestSortBatchFilesSelectedDocuments(t *testing.T) {
	for _, together := range []bool{false, true} {
		sh, sp, lib := newShell(t, 2)
		sp.SetAnalyzer(&inbox.Analyzer{
			PDF: pdf.NewFake(batchText), Store: store.NewFake(),
			Now: func() time.Time { return time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC) },
		})
		before := 0
		if err := lib.Load(context.Background()); err == nil {
			before = lib.Total()
		}
		sh.Show("sort")
		for i := 0; i < 300; i++ { // wait for the analysis of both documents
			ready := 0
			for _, it := range sp.Items() {
				if it.Status == inbox.StatusReady {
					ready++
				}
			}
			if ready == 2 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		settle()
		if !sh.Registry.Run("sort.batch") {
			t.Fatal("command sort.batch missing")
		}
		settle()
		if sp.Count() != 2 {
			t.Fatal("the batch command without a batch must do nothing")
		}

		g := queueGrid(t, sh.Win.Content())
		var keys []string
		for _, it := range sp.Items() {
			keys = append(keys, it.ID)
		}
		g.SelectKeys(keys)
		settle()
		if got := sp.Selection(); len(got) != 2 {
			t.Fatalf("presenter selection %v", got)
		}
		found := false
		walk(sh.Win.Content(), func(o fyne.CanvasObject) {
			if l, ok := o.(*widgets.Label); ok && l.Text == i18n.T("sort.batch.title", "Count", "2") && l.Visible() {
				found = true
			}
		})
		if !found {
			t.Error("batch panel not shown")
		}

		if !sh.Registry.Run("sort.batch") {
			t.Fatal("command sort.batch missing")
		}
		settle()
		if sp.Count() != 0 {
			t.Fatalf("queue has %d documents left", sp.Count())
		}
		if err := lib.Load(context.Background()); err != nil || lib.Total() != before+2 {
			t.Fatalf("library %d -> %d (err %v)", before, lib.Total(), err)
		}
		if abs, ok := sp.CopyPath(true); !ok {
			t.Error("no last path")
		} else if _, err := os.Stat(abs); err != nil {
			t.Error(err)
		}

		if together {
			sh.Registry.Run("sort.undobatch")
		} else {
			sh.Registry.Run("sort.undo")
			settle()
			sh.Registry.Run("sort.undo")
		}
		settle()
		if sp.Count() != 2 || sp.CanUndo() {
			t.Fatalf("together=%v: queue %d, canUndo %v", together, sp.Count(), sp.CanUndo())
		}
		if err := lib.Load(context.Background()); err != nil || lib.Total() != before {
			t.Errorf("library after undo %d, want %d", lib.Total(), before)
		}
	}
}
