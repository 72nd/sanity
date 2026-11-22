package presenter

import (
	"context"
	"github.com/72nd/sanity/domain"
	"strings"
	"testing"

	"github.com/72nd/sanity/store"
)

func stubT(id string, kv ...any) string {
	s := id
	for i := 1; i < len(kv); i += 2 {
		s += " " + kv[i].(string)
	}
	return s
}

func loaded(t *testing.T) *Library {
	t.Helper()
	p := NewLibrary(store.NewFake(), stubT)
	if err := p.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadSortsNewestFirstAndResolvesTypes(t *testing.T) {
	p := loaded(t)
	if !p.Loaded() || p.Total() == 0 {
		t.Fatal("nothing loaded")
	}
	rows := p.Visible("")
	for i := 1; i < len(rows); i++ {
		if rows[i].Entry.Date.After(rows[i-1].Entry.Date) {
			t.Fatalf("row %d is newer than row %d", i, i-1)
		}
	}
	for _, r := range rows {
		if r.TypeName == "" || r.TypeName == r.Entry.TypeID {
			t.Errorf("type not resolved for %s: %q", r.Entry.ID, r.TypeName)
		}
	}
}

func TestVisibleFilters(t *testing.T) {
	p := loaded(t)
	got := p.Visible("telekom")
	if len(got) == 0 || len(got) >= p.Total() {
		t.Fatalf("filter returned %d of %d", len(got), p.Total())
	}
	for _, r := range got {
		hay := strings.ToLower(r.Entry.Path + r.TypeName)
		if !strings.Contains(hay, "telekom") {
			t.Errorf("unexpected row %q", r.Entry.Path)
		}
	}
	if len(p.Visible("zzzz-no-hit")) != 0 {
		t.Error("expected no hits")
	}
}

func TestStatusItems(t *testing.T) {
	p := loaded(t)
	if n := len(p.StatusItems(3, 1)); n != 1 {
		t.Errorf("one selected: %d items", n)
	}
	if n := len(p.StatusItems(3, 2)); n != 2 {
		t.Errorf("two selected: %d items", n)
	}
}

func TestParseAmount(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int64
		cur  string
		ok   bool
	}{
		{"49.95 EUR", 4995, "EUR", true},
		{"1.234,56 €", 123456, "EUR", true},
		{"$12.50", 1250, "USD", true},
		{"1,234.5 USD", 123450, "USD", true},
		{"1.234 EUR", 123400, "EUR", true},
		{"7", 700, "", true},
		{"-3,20 EUR", -320, "EUR", true},
		{"", 0, "", false},
		{"n/a", 0, "", false},
	} {
		got, cur, ok := ParseAmount(c.in)
		if got != c.want || cur != c.cur || ok != c.ok {
			t.Errorf("ParseAmount(%q) = %d %q %v, want %d %q %v", c.in, got, cur, ok, c.want, c.cur, c.ok)
		}
	}
}

func TestSelectionStatusTotals(t *testing.T) {
	p := loaded(t)
	rows := p.Visible("telekom")
	if len(rows) < 2 {
		t.Fatal("need telekom rows")
	}
	got := p.SelectionStatus(len(rows), rows[:2])
	if len(got) != 2 || !strings.Contains(got[1], "status.selected.total") || !strings.Contains(got[1], "EUR") {
		t.Errorf("status = %q", got)
	}
	if got := p.SelectionStatus(5, nil); len(got) != 1 {
		t.Errorf("no selection must show only the row count, got %q", got)
	}
	mixed := []Row{{Entry: rows[0].Entry}, {Entry: rows[1].Entry}}
	mixed[1].Entry.Amount = "10 USD"
	sums := p.Totals(mixed)
	if len(sums) != 2 || sums[0].Currency != "EUR" || sums[1].String() != "10.00 USD" {
		t.Errorf("sums = %+v", sums)
	}
}

func TestCopyPathsAndReveal(t *testing.T) {
	p := loaded(t)
	rows := p.Visible("")[:2]
	rel := p.CopyPaths(rows, false)
	lines := strings.Split(rel, "\n")
	if len(lines) != 2 || lines[0] != rows[0].Entry.Path || strings.Contains(rel, "/Users/") {
		t.Errorf("relative = %q", rel)
	}
	abs := strings.Split(p.CopyPaths(rows, true), "\n")
	if abs[1] != "/Users/demo/muster-gmbh/"+rows[1].Entry.Path {
		t.Errorf("absolute = %q", abs[1])
	}
	if p.RevealPath(rows[0]) != abs[0] {
		t.Errorf("reveal = %q", p.RevealPath(rows[0]))
	}
}

func TestNextHitWraps(t *testing.T) {
	p := loaded(t)
	rows := p.Visible("")
	idx := []int{}
	for i, r := range rows {
		if strings.Contains(strings.ToLower(r.TypeName), "vodafone") {
			idx = append(idx, i)
		}
	}
	if len(idx) < 2 {
		t.Fatal("need two vodafone rows")
	}
	if got := p.NextHit(rows, idx[0], "vodafone", true); got != idx[1] {
		t.Errorf("next = %d, want %d", got, idx[1])
	}
	if got := p.NextHit(rows, idx[0], "vodafone", false); got != idx[len(idx)-1] {
		t.Errorf("prev wraps to %d, want %d", got, idx[len(idx)-1])
	}
	if got := p.NextHit(rows, 0, "no such text", true); got != -1 {
		t.Errorf("no hit = %d", got)
	}
	if got := p.NextHit(rows, len(rows)-1, "", true); got != 0 {
		t.Errorf("empty query wraps to %d", got)
	}
}

func TestRemoveKeepsFilesAndUpdatesRows(t *testing.T) {
	p := loaded(t)
	total := p.Total()
	rows := p.Visible("")[:2]
	var removed []string
	p.OnRemoved = func(e domain.LibraryEntry) { removed = append(removed, e.ID) }
	n, err := p.Remove(context.Background(), rows)
	if len(removed) != 2 || removed[0] != rows[0].Entry.ID {
		t.Errorf("OnRemoved got %v, want the two removed entries", removed)
	}
	if err != nil || n != 2 || p.Total() != total-2 {
		t.Fatalf("n=%d err=%v total=%d", n, err, p.Total())
	}
	if err := p.Load(context.Background()); err != nil || p.Total() != total-2 {
		t.Errorf("store still has the entries: %d", p.Total())
	}
}

func TestResortCallsImportFn(t *testing.T) {
	p := loaded(t)
	var got []string
	p.ImportFn = func(paths ...string) { got = paths }
	rows := p.Visible("")[:2]
	paths := p.Resort(rows)
	if len(got) != 2 || got[0] != paths[0] || got[0] != p.AbsPath(rows[0]) {
		t.Errorf("import = %q, paths = %q", got, paths)
	}
}

func TestInboxHint(t *testing.T) {
	p := loaded(t)
	if _, ok := p.InboxHint(); ok {
		t.Error("hint without a counter")
	}
	n := 0
	p.InboxCount = func() int { return n }
	if _, ok := p.InboxHint(); ok {
		t.Error("hint for an empty inbox")
	}
	n = 3
	if h, ok := p.InboxHint(); !ok || !strings.Contains(h, "3") {
		t.Errorf("hint = %q %v", h, ok)
	}
}
