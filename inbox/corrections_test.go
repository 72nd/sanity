package inbox

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/domain"
)

func TestNewCorrectionRecordsSuggestionVersusChoice(t *testing.T) {
	d := func(day int) time.Time { return time.Date(2025, 3, day, 0, 0, 0, 0, time.UTC) }
	res := Result{
		Suggestions: []analysis.Suggestion{{TypeID: "tk", Score: 0.91, Confident: true}, {TypeID: "x", Score: 0.3}},
		Dates:       []analysis.DateCandidate{{Date: d(12)}, {Date: d(1), Fallback: true}},
		Invoices:    []analysis.Found{{Value: "R-1"}},
		Amounts:     []analysis.Amount{{Cents: 4995, Currency: "EUR"}},
	}
	v := domain.Values{Date: d(14), Invoice: "R-1", Amount: "49.95 EUR"}
	at := time.Date(2025, 5, 1, 10, 0, 0, 0, time.UTC)
	c := NewCorrection(res, true, "other", v, "sha1", at, []string{"type", "date"})
	if c.TypeID != "other" || c.SHA256 != "sha1" || !c.At.Equal(at) {
		t.Errorf("basics %+v", c)
	}
	if c.Suggested.TypeID != "tk" || c.Suggested.Score != 0.91 || !c.Suggested.Confident {
		t.Errorf("suggested %+v", c.Suggested)
	}
	if c.DateSuggested != "2025-03-12" || c.DateChosen != "2025-03-14" {
		t.Errorf("date %q -> %q", c.DateSuggested, c.DateChosen)
	}
	if c.InvoiceSuggested != "R-1" || c.InvoiceChosen != "R-1" || c.AmountSuggested != "49.95 EUR" || c.AmountChosen != "49.95 EUR" {
		t.Errorf("fields %+v", c)
	}
	if len(c.Overrides) != 2 {
		t.Errorf("overrides %v", c.Overrides)
	}
	// no analysis: nothing suggested
	n := NewCorrection(Result{}, false, "other", v, "sha2", at, nil)
	if n.Suggested.TypeID != "" || n.DateSuggested != "" || n.DateChosen != "2025-03-14" {
		t.Errorf("no analysis %+v", n)
	}
}

func TestCorrectionLogAppendAndReadAll(t *testing.T) {
	root := t.TempDir()
	if cs, err := ReadAll(root); err != nil || len(cs) != 0 {
		t.Fatalf("empty log: %v %v", cs, err)
	}
	l := NewCorrectionLog(root)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.Append(Correction{SHA256: "s", TypeID: "a"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if err := l.Append(Correction{SHA256: "s", Undone: true}); err != nil {
		t.Fatal(err)
	}
	// a torn line is skipped, the rest still reads
	f, _ := os.OpenFile(l.path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("{\"sha256\":\n")
	f.Close()
	cs, err := ReadAll(root)
	if err != nil || len(cs) != 21 || !cs[20].Undone {
		t.Fatalf("read %d lines, err %v", len(cs), err)
	}
	if want := filepath.Join(root, "office", ".sanity", "corrections.jsonl"); l.path != want {
		t.Errorf("path %q", l.path)
	}
}

func TestStats(t *testing.T) {
	cs := []Correction{
		{SHA256: "1", TypeID: "tk", Suggested: SuggestedType{TypeID: "tk", Confident: true}, DateSuggested: "2025-01-02", DateChosen: "2025-01-02"},
		{SHA256: "2", TypeID: "tk", Suggested: SuggestedType{TypeID: "hz", Confident: true}, DateSuggested: "2025-01-02", DateChosen: "2025-01-03"},
		{SHA256: "3", TypeID: "hz", Suggested: SuggestedType{TypeID: "hz"}},
		{SHA256: "4", TypeID: "hz"}, // nothing suggested
		{SHA256: "5", TypeID: "hz", Suggested: SuggestedType{TypeID: "hz", Confident: true}},
		{SHA256: "5", Undone: true}, // 5 was undone: neither it nor the undo line counts
	}
	r := Stats(cs)
	if r.Total != 4 || r.TypeHits != 2 || r.ConfidentTotal != 2 || r.ConfidentHits != 1 || r.DateTotal != 2 || r.DateHits != 1 {
		t.Errorf("report %+v", r)
	}
	if got := r.ByType["tk"]; got.Total != 2 || got.Hits != 1 {
		t.Errorf("tk %+v", got)
	}
	if got := r.ByType["hz"]; got.Total != 2 || got.Hits != 1 {
		t.Errorf("hz %+v", got)
	}
	if (Report{}).TypeRate() != 0 {
		t.Error("empty rate must be 0")
	}
}
