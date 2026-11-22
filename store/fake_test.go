package store

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/72nd/sanity/domain"
)

// testStoreCRUD checks the write methods against any Store. It works for an
// empty store as well as for sample data.
func testStoreCRUD(t *testing.T, s Store) {
	t.Helper()
	ctx := context.Background()

	// SaveType replaces in place: count and position stay the same.
	types, err := s.Types(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(types) == 0 {
		if err := s.SaveType(ctx, domain.DocumentType{ID: "seed", Name: "Seed"}); err != nil {
			t.Fatal(err)
		}
		types, _ = s.Types(ctx)
	}
	first := types[0]
	first.Name = "Replaced name"
	if err := s.SaveType(ctx, first); err != nil {
		t.Fatal(err)
	}
	types2, _ := s.Types(ctx)
	if len(types2) != len(types) {
		t.Fatalf("replace changed type count: %d -> %d", len(types), len(types2))
	}
	if types2[0].ID != first.ID || types2[0].Name != "Replaced name" {
		t.Fatalf("replace did not update type %q in place: %+v", first.ID, types2[0])
	}

	// SaveType inserts an unknown id at the end.
	added := domain.DocumentType{ID: "test-type", Name: "Test", PathTemplate: "x/{invoice}.pdf"}
	if err := s.SaveType(ctx, added); err != nil {
		t.Fatal(err)
	}
	types3, _ := s.Types(ctx)
	if len(types3) != len(types)+1 {
		t.Fatalf("insert: want %d types, got %d", len(types)+1, len(types3))
	}
	if last := types3[len(types3)-1]; last.ID != "test-type" {
		t.Fatalf("insert: new type not last, got %q", last.ID)
	}

	// AddEntry appends, duplicate ids are rejected.
	before, _ := s.Library(ctx)
	entry := domain.LibraryEntry{ID: "test-entry", TypeID: "test-type", Path: "x/test.pdf", Date: time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)}
	if err := s.AddEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if err := s.AddEntry(ctx, entry); err == nil {
		t.Fatal("duplicate AddEntry: want error")
	}
	after, _ := s.Library(ctx)
	if len(after) != len(before)+1 || after[len(after)-1].ID != "test-entry" {
		t.Fatalf("AddEntry: library not appended correctly (%d -> %d)", len(before), len(after))
	}

	// RemoveEntry removes by id, unknown ids are an error.
	if err := s.RemoveEntry(ctx, "test-entry"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveEntry(ctx, "test-entry"); err == nil {
		t.Fatal("RemoveEntry of missing id: want error")
	}
	final, _ := s.Library(ctx)
	if len(final) != len(before) {
		t.Fatalf("RemoveEntry: want %d entries, got %d", len(before), len(final))
	}
}

func TestFakeSampleData(t *testing.T) {
	ctx := context.Background()
	f := NewFake()

	p, err := f.Project(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Muster GmbH" || p.Inbox != domain.DefaultInbox {
		t.Fatalf("unexpected project: name=%q inbox=%q", p.Name, p.Inbox)
	}

	types, _ := f.Types(ctx)
	if len(types) != 5 {
		t.Fatalf("want 5 types, got %d", len(types))
	}
	typeIDs := map[string]bool{}
	for _, dt := range types {
		typeIDs[dt.ID] = true
		if len(dt.Rules) == 0 {
			t.Errorf("type %q has no rules", dt.ID)
		}
		if !strings.Contains(dt.PathTemplate, "{date:") {
			t.Errorf("type %q: path template without date: %q", dt.ID, dt.PathTemplate)
		}
	}

	lib, _ := f.Library(ctx)
	if len(lib) != 25 {
		t.Fatalf("want 25 library entries, got %d", len(lib))
	}
	amount := regexp.MustCompile(`^\d+\.\d{2} EUR$`)
	seen := map[string]bool{}
	for _, e := range lib {
		if seen[e.ID] {
			t.Errorf("duplicate library id %q", e.ID)
		}
		seen[e.ID] = true
		if !typeIDs[e.TypeID] {
			t.Errorf("entry %q references unknown type %q", e.ID, e.TypeID)
		}
		if e.Date.Year() != 2025 {
			t.Errorf("entry %q outside 2025: %v", e.ID, e.Date)
		}
		if e.Amount != "" && !amount.MatchString(e.Amount) {
			t.Errorf("entry %q: amount %q is not like \"49.95 EUR\"", e.ID, e.Amount)
		}
		if e.Path == "" || e.OriginalPath == "" || e.SHA256 == "" {
			t.Errorf("entry %q misses path, original path or hash", e.ID)
		}
	}
}

func TestFakeReturnsCopies(t *testing.T) {
	ctx := context.Background()
	f := NewFake()

	lib, _ := f.Library(ctx)
	lib[0].Path = "changed"
	types, _ := f.Types(ctx)
	types[0].Rules[0].Value = "changed"
	p, _ := f.Project(ctx)
	p.IgnoreTokens[0] = "changed"

	lib2, _ := f.Library(ctx)
	if lib2[0].Path == "changed" {
		t.Error("Library result aliases stored data")
	}
	types2, _ := f.Types(ctx)
	if types2[0].Rules[0].Value == "changed" {
		t.Error("Types result aliases stored rules")
	}
	p2, _ := f.Project(ctx)
	if p2.IgnoreTokens[0] == "changed" {
		t.Error("Project result aliases stored tokens")
	}
}

func TestFakeCRUD(t *testing.T) {
	testStoreCRUD(t, NewFake())
}

func TestFakeConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	f := NewFake()

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := "conc-" + string(rune('a'+i))
			_ = f.AddEntry(ctx, domain.LibraryEntry{ID: id, TypeID: typeCursor, Path: id + ".pdf"})
			_, _ = f.Library(ctx)
			_ = f.SaveType(ctx, domain.DocumentType{ID: id, Name: id})
			_, _ = f.Types(ctx)
			_ = f.RemoveEntry(ctx, id)
		}()
	}
	wg.Wait()

	lib, _ := f.Library(ctx)
	if len(lib) != 25 {
		t.Fatalf("after concurrent add/remove want 25 entries, got %d", len(lib))
	}
}
