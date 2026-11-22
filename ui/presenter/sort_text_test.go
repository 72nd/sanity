package presenter

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/72nd/sanity/domain"
)

// projectType adds a type whose template has a {text:Project} placeholder.
func projectType(t *testing.T, p *Sort) domain.DocumentType {
	t.Helper()
	ty := domain.DocumentType{ID: "t-proj", Name: "Project Invoice",
		PathTemplate: "projects/{text:Project}/{date:2006-01-02}_{text:Project}_{invoice}.pdf"}
	if err := p.store.SaveType(context.Background(), ty); err != nil {
		t.Fatal(err)
	}
	if err := p.LoadTypes(context.Background()); err != nil {
		t.Fatal(err)
	}
	return ty
}

func TestTextFieldsMissingAndPreview(t *testing.T) {
	p, _ := newSort(t, 1)
	ty := projectType(t, p)
	p.Step(0)
	if got := p.TextFields(); got != nil {
		t.Fatalf("no type: %v", got)
	}
	p.SetType(telekom(t, p).ID)
	if got := p.TextFields(); len(got) != 0 {
		t.Fatalf("telekom has no text fields: %v", got)
	}
	p.SetType(ty.ID)
	if got := p.TextFields(); !reflect.DeepEqual(got, []string{"Project"}) {
		t.Fatalf("TextFields = %v (template order, deduplicated)", got)
	}
	p.SetDate(time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC))
	p.SetInvoice("R-1")
	if m := p.Missing(); !reflect.DeepEqual(m, []string{"text:Project"}) {
		t.Fatalf("Missing = %v", m)
	}
	if p.CanFile() {
		t.Fatal("must not file without the text")
	}
	p.SetText("Project", "  Alpha ")
	rel, ok := p.PathPreview()
	if !ok || rel != "projects/Alpha/2025-03-12_Alpha_R-1.pdf" {
		t.Fatalf("preview %q ok=%v", rel, ok)
	}
}

func TestTextKeptPerLabelWhenSwitchingTypes(t *testing.T) {
	p, _ := newSort(t, 1)
	ty := projectType(t, p)
	p.Step(0)
	p.SetType(ty.ID)
	p.SetText("Project", "Alpha")
	p.SetType(telekom(t, p).ID)
	p.SetType(ty.ID)
	if got := p.Form().Text["Project"]; got != "Alpha" {
		t.Errorf("text lost on type switch: %q", got)
	}
}

func TestFileWithTextTemplate(t *testing.T) {
	p, root := newSort(t, 1)
	ty := projectType(t, p)
	p.Step(0)
	p.SetType(ty.ID)
	p.SetDate(time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC))
	p.SetInvoice("R-1")
	p.SetText("Project", "Alpha")
	e, err := p.File(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := "projects/Alpha/2025-03-12_Alpha_R-1.pdf"
	if e.Path != want {
		t.Fatalf("filed %q, want %q", e.Path, want)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(want))); err != nil {
		t.Error(err)
	}
}

func TestBatchBlocksOnMissingText(t *testing.T) {
	p := batchSort(t)
	ty := projectType(t, p)
	p.SetSelection(ids(p, 0, 1))
	plan := p.BatchPlan(ty.ID)
	for _, b := range plan {
		if !b.Blocked() || !reflect.DeepEqual(b.Missing, []string{"text:Project"}) {
			t.Errorf("%s: path %q missing %v, want blocked on text:Project", b.Name, b.Path, b.Missing)
		}
	}
	if p.CanBatch(ty.ID) {
		t.Error("batch must not be fileable without the text")
	}
}

func TestDuplicateReasonText(t *testing.T) {
	p, _ := newSort(t, 1)
	for in, want := range map[string]string{
		"same file (SHA-256)": "sort.duplicate.reason.file",
		"same text":           "sort.duplicate.reason.text",
		"same invoice number": "sort.duplicate.reason.invoice",
		"something new":       "something new",
	} {
		if got := p.DuplicateReasonText(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}
