package presenter

import (
	"reflect"
	"testing"
)

func TestFieldLabels(t *testing.T) {
	tr := func(id string, _ ...any) string { return id }
	tests := []struct {
		name  string
		names []string
		want  []string
	}{
		{"nil", nil, nil},
		{"two date layouts give one entry", []string{"date:2006", "date:2006-01-02"}, []string{"sort.date"}},
		{"text label as is", []string{"text:Project"}, []string{"Project"}},
		{"unknown name as is", []string{"mystery"}, []string{"mystery"}},
		{"known placeholders mapped", []string{"type", "invoice", "amount", "original"},
			[]string{"sort.type", "sort.invoice", "sort.amount", "sort.original"}},
		{"order preserved", []string{"invoice", "text:Project", "date:2006", "text:Client", "date:2006-01-02", "amount"},
			[]string{"sort.invoice", "Project", "sort.date", "Client", "sort.amount"}},
		{"duplicate text labels once", []string{"text:Project", "text:Project"}, []string{"Project"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FieldLabels(tr, tt.names); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("FieldLabels(%v) = %v, want %v", tt.names, got, tt.want)
			}
		})
	}
}

func TestSortMissingLabels(t *testing.T) {
	p, _ := newSort(t, 1)
	p.T = func(id string, _ ...any) string { return id }
	if got := p.MissingLabels(); got != nil {
		t.Fatalf("no type selected: %v", got)
	}
	ty := projectType(t, p)
	p.Step(0)
	p.SetType(ty.ID)
	want := []string{"Project", "sort.date", "sort.invoice"}
	if got := p.MissingLabels(); !reflect.DeepEqual(got, want) {
		t.Fatalf("MissingLabels() = %v, want %v", got, want)
	}
}
