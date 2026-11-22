package presenter

import (
	"context"
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
