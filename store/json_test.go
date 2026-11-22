package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/72nd/sanity/domain"
)

func TestOpenJSONCreatesDefaultProject(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "Muster GmbH")

	j, err := OpenJSON(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "office", "sanity.json")); err != nil {
		t.Fatalf("sanity.json not created: %v", err)
	}
	p, _ := j.Project(ctx)
	if p.Name != "Muster GmbH" {
		t.Errorf("default name: want folder name, got %q", p.Name)
	}
	if p.Inbox != domain.DefaultInbox {
		t.Errorf("inbox: want %q, got %q", domain.DefaultInbox, p.Inbox)
	}
	if p.Root != root {
		t.Errorf("Root: want %q, got %q", root, p.Root)
	}
	if types, _ := j.Types(ctx); len(types) != 0 {
		t.Errorf("default project should have no types, got %d", len(types))
	}
}

func TestJSONRoundTrip(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	j, err := OpenJSON(root)
	if err != nil {
		t.Fatal(err)
	}
	dt := domain.DocumentType{
		ID:           "telekom",
		Name:         "Telekom",
		PathTemplate: "receipts/{date:2006}/telecom/{date:2006-01-02}_telekom_{invoice}.pdf",
		Rules: []domain.Rule{
			{Kind: "keyword", Value: "Telekom", Mode: "should", Weight: 0.9, Where: "any", Enabled: true},
		},
		Threshold:  0.8,
		UseLearned: true,
		Date: domain.DateConfig{
			PreferredLabels: []string{"Rechnungsdatum"},
			Source:          "text",
			MaxAgeYears:     3,
		},
	}
	if err := j.SaveType(ctx, dt); err != nil {
		t.Fatal(err)
	}
	entry := domain.LibraryEntry{
		ID:           "lib-1",
		TypeID:       "telekom",
		Path:         "receipts/2025/telecom/2025-01-08_telekom_4500123401.pdf",
		OriginalPath: "/Users/demo/Downloads/telekom.pdf",
		Date:         time.Date(2025, 1, 8, 0, 0, 0, 0, time.UTC),
		Invoice:      "4500123401",
		Amount:       "49.95 EUR",
		SHA256:       "abc",
		FiledAt:      time.Date(2025, 1, 10, 9, 14, 0, 0, time.UTC),
		Overrides:    []string{"type"},
	}
	if err := j.AddEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenJSON(root)
	if err != nil {
		t.Fatal(err)
	}
	types, _ := reopened.Types(ctx)
	if len(types) != 1 || !reflect.DeepEqual(types[0], dt) {
		t.Fatalf("type after reopen differs:\nwant %+v\ngot  %+v", dt, types)
	}
	lib, _ := reopened.Library(ctx)
	if len(lib) != 1 {
		t.Fatalf("want 1 entry after reopen, got %d", len(lib))
	}
	got := lib[0]
	if got.ID != entry.ID || got.Path != entry.Path || got.Amount != entry.Amount ||
		!got.Date.Equal(entry.Date) || !got.FiledAt.Equal(entry.FiledAt) ||
		!reflect.DeepEqual(got.Overrides, entry.Overrides) {
		t.Fatalf("entry after reopen differs:\nwant %+v\ngot  %+v", entry, got)
	}

	// The file is indented for diffs.
	data, err := os.ReadFile(filepath.Join(root, "office", "sanity.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\n  \"name\"") {
		t.Errorf("sanity.json is not indented:\n%s", data)
	}
	if !json.Valid(data) {
		t.Error("sanity.json is not valid JSON")
	}
}

func TestJSONAtomicWriteLeavesNoTempFiles(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	j, err := OpenJSON(root)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		id := "type-" + string(rune('a'+i))
		if err := j.SaveType(ctx, domain.DocumentType{ID: id, Name: id}); err != nil {
			t.Fatal(err)
		}
	}
	if err := j.AddEntry(ctx, domain.LibraryEntry{ID: "lib-1", TypeID: "type-a"}); err != nil {
		t.Fatal(err)
	}
	if err := j.RemoveEntry(ctx, "lib-1"); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Join(root, "office"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "sanity.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("want only sanity.json in office/, got %v", names)
	}
}

func TestJSONSaveTypeReplaceAndInsert(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	j, err := OpenJSON(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.SaveType(ctx, domain.DocumentType{ID: "a", Name: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := j.SaveType(ctx, domain.DocumentType{ID: "b", Name: "B"}); err != nil {
		t.Fatal(err)
	}
	// Replace keeps position and count.
	if err := j.SaveType(ctx, domain.DocumentType{ID: "a", Name: "A2"}); err != nil {
		t.Fatal(err)
	}
	types, _ := j.Types(ctx)
	if len(types) != 2 || types[0].ID != "a" || types[0].Name != "A2" {
		t.Fatalf("replace: unexpected types %+v", types)
	}
	// Insert appends.
	if err := j.SaveType(ctx, domain.DocumentType{ID: "c", Name: "C"}); err != nil {
		t.Fatal(err)
	}
	types, _ = j.Types(ctx)
	if len(types) != 3 || types[2].ID != "c" {
		t.Fatalf("insert: unexpected types %+v", types)
	}
}

func TestJSONCRUD(t *testing.T) {
	j, err := OpenJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testStoreCRUD(t, j)
}

func TestJSONFailedWriteKeepsMemoryState(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	ctx := context.Background()
	root := t.TempDir()
	j, err := OpenJSON(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "office")
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	if err := j.AddEntry(ctx, domain.LibraryEntry{ID: "lib-1"}); err == nil {
		t.Fatal("AddEntry into read-only folder: want error")
	}
	lib, _ := j.Library(ctx)
	if len(lib) != 0 {
		t.Fatalf("failed write must not change memory, got %d entries", len(lib))
	}
}

func TestJSONDeleteTypePersists(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	j, err := OpenJSON(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		if err := j.SaveType(ctx, domain.DocumentType{ID: id, Name: id}); err != nil {
			t.Fatal(err)
		}
	}
	if err := j.DeleteType(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	j2, err := OpenJSON(root)
	if err != nil {
		t.Fatal(err)
	}
	types, _ := j2.Types(ctx)
	if len(types) != 1 || types[0].ID != "b" {
		t.Fatalf("after reopen: %+v", types)
	}
}
