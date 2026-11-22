package domain

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func mustParse(t *testing.T, s string) Template {
	t.Helper()
	tpl, err := ParseTemplate(s)
	if err != nil {
		t.Fatalf("ParseTemplate(%q): unexpected error: %v", s, err)
	}
	return tpl
}

var (
	testDate = time.Date(2025, time.March, 12, 14, 30, 0, 0, time.UTC)
	planTmpl = "receipts/{date:2006}/telecom/{date:2006-01-02}_telekom_{invoice}.pdf"
)

func TestParseTemplate(t *testing.T) {
	valid := []string{
		"",
		"receipts/fixed.pdf",
		"{invoice}",
		planTmpl,
		"{date:15:04}",
		"{date:2006/01}/{type}",
		"{text:Project name}/{amount}_{original}",
		"{type}{type}",
	}
	for _, s := range valid {
		t.Run("valid "+s, func(t *testing.T) {
			if _, err := ParseTemplate(s); err != nil {
				t.Errorf("ParseTemplate(%q) = %v, want no error", s, err)
			}
		})
	}

	invalid := []string{
		"a}b",         // unbalanced close
		"a{b",         // unclosed open
		"{invoice",    // unclosed open
		"{{invoice}}", // nested braces
		"{date:2006{x}",
		"{}",           // empty placeholder
		"{unknown}",    // unknown name
		"{Invoice}",    // names are case sensitive
		"{date}",       // date needs a layout
		"{date:}",      // empty layout
		"{text}",       // text needs a label
		"{text:}",      // empty label
		"{invoice:x}",  // takes no argument
		"{type:x}",     // takes no argument
		"{original:x}", // takes no argument
	}
	for _, s := range invalid {
		t.Run("invalid "+s, func(t *testing.T) {
			_, err := ParseTemplate(s)
			if !errors.Is(err, ErrInvalidTemplate) {
				t.Errorf("ParseTemplate(%q) error = %v, want ErrInvalidTemplate", s, err)
			}
		})
	}
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    string
		values  Values
		want    string
		wantErr error
	}{
		{
			name:   "plan example",
			tmpl:   planTmpl,
			values: Values{Date: testDate, Invoice: "R-88123"},
			want:   "receipts/2025/telecom/2025-03-12_telekom_R-88123.pdf",
		},
		{
			name: "literal only",
			tmpl: "receipts/fixed.pdf",
			want: "receipts/fixed.pdf",
		},
		{
			name:   "type and original",
			tmpl:   "{type}/{original}.pdf",
			values: Values{Type: " Telekom Invoice ", Original: "scan 004"},
			want:   "Telekom Invoice/scan 004.pdf",
		},
		{
			name:   "amount keeps dots",
			tmpl:   "{amount}.pdf",
			values: Values{Amount: "49.95"},
			want:   "49.95.pdf",
		},
		{
			name:   "text label",
			tmpl:   "{text:Project}/{invoice}.pdf",
			values: Values{Invoice: "1", Text: map[string]string{"Project": "Alpha"}},
			want:   "Alpha/1.pdf",
		},
		{
			name:   "date layout with slash is a folder",
			tmpl:   "{date:2006/01}/{invoice}.pdf",
			values: Values{Date: testDate, Invoice: "R1"},
			want:   "2025/03/R1.pdf",
		},
		{
			name:   "date layout with colon is made safe",
			tmpl:   "{date:15:04}.pdf",
			values: Values{Date: testDate},
			want:   "14_30.pdf",
		},
		{
			name:   "date month name",
			tmpl:   "{date:Jan}",
			values: Values{Date: testDate},
			want:   "Mar",
		},
		{
			name:   "unsafe characters are replaced",
			tmpl:   "{invoice}.pdf",
			values: Values{Invoice: `a/b\c:d*e?f"g<h>i|j`},
			want:   "a_b_c_d_e_f_g_h_i_j.pdf",
		},
		{
			name:   "control characters are replaced",
			tmpl:   "{invoice}.pdf",
			values: Values{Invoice: "a\tb\x01c"},
			want:   "a_b_c.pdf",
		},
		{
			name:   "spaces and dots at the ends are trimmed",
			tmpl:   "{invoice}.pdf",
			values: Values{Invoice: "  .R-1. "},
			want:   "R-1.pdf",
		},
		{
			name:   "value with dots at both ends",
			tmpl:   "{invoice}.pdf",
			values: Values{Invoice: "..x.."},
			want:   "x.pdf",
		},
		{
			name:   "traversal inside a value cannot escape",
			tmpl:   "receipts/{invoice}.pdf",
			values: Values{Invoice: "../../etc"},
			want:   "receipts/_.._etc.pdf",
		},
		{
			name:   "date layout may not produce a placeholder escape",
			tmpl:   "receipts/{date:2006}/{invoice}.pdf",
			values: Values{Date: testDate, Invoice: "1"},
			want:   "receipts/2025/1.pdf",
		},

		// missing values
		{
			name:    "missing invoice",
			tmpl:    planTmpl,
			values:  Values{Date: testDate},
			wantErr: ErrMissingValue,
		},
		{
			name:    "invoice that is only spaces and dots counts as missing",
			tmpl:    "{invoice}.pdf",
			values:  Values{Invoice: " . "},
			wantErr: ErrMissingValue,
		},
		{
			name:    "missing text label",
			tmpl:    "{text:Project}/{invoice}.pdf",
			values:  Values{Invoice: "1", Text: map[string]string{"Other": "x"}},
			wantErr: ErrMissingValue,
		},
		{
			name:    "zero date is missing",
			tmpl:    "{date:2006}/x.pdf",
			wantErr: ErrMissingValue,
		},

		// path escape and unsafe paths
		{
			name:    "absolute path",
			tmpl:    "/receipts/{invoice}.pdf",
			values:  Values{Invoice: "1"},
			wantErr: ErrUnsafePath,
		},
		{
			name:    "leading parent segment",
			tmpl:    "../{invoice}.pdf",
			values:  Values{Invoice: "1"},
			wantErr: ErrUnsafePath,
		},
		{
			name:    "parent segment in the middle",
			tmpl:    "receipts/../{invoice}.pdf",
			values:  Values{Invoice: "1"},
			wantErr: ErrUnsafePath,
		},
		{
			name:    "parent segment from a date layout",
			tmpl:    "{date:..}/{invoice}.pdf",
			values:  Values{Date: testDate, Invoice: "1"},
			wantErr: ErrUnsafePath,
		},
		{
			name:    "empty segment",
			tmpl:    "receipts//{invoice}.pdf",
			values:  Values{Invoice: "1"},
			wantErr: ErrUnsafePath,
		},
		{
			name:    "trailing slash",
			tmpl:    "receipts/{invoice}/",
			values:  Values{Invoice: "1"},
			wantErr: ErrUnsafePath,
		},
		{
			name:    "colon in literal",
			tmpl:    "re:ceipts/{invoice}.pdf",
			values:  Values{Invoice: "1"},
			wantErr: ErrUnsafePath,
		},
		{
			name:    "backslash in literal",
			tmpl:    `receipts\{invoice}.pdf`,
			values:  Values{Invoice: "1"},
			wantErr: ErrUnsafePath,
		},
		{
			name:    "windows drive",
			tmpl:    "C:/receipts/{invoice}.pdf",
			values:  Values{Invoice: "1"},
			wantErr: ErrUnsafePath,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mustParse(t, tt.tmpl).Evaluate(tt.values)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Evaluate(%q) error = %v, want %v", tt.tmpl, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Evaluate(%q) unexpected error: %v", tt.tmpl, err)
			}
			if got != tt.want {
				t.Errorf("Evaluate(%q) = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestMissing(t *testing.T) {
	tests := []struct {
		name   string
		tmpl   string
		values Values
		want   []string
	}{
		{
			name:   "all values present",
			tmpl:   planTmpl,
			values: Values{Date: testDate, Invoice: "R1"},
			want:   nil,
		},
		{
			name:   "invoice missing",
			tmpl:   planTmpl,
			values: Values{Date: testDate},
			want:   []string{"invoice"},
		},
		{
			name:   "everything missing, in template order, each once",
			tmpl:   "{type}/{text:Project}/{invoice}_{invoice}_{text:Project}_{date:2006}",
			values: Values{},
			want:   []string{"type", "text:Project", "invoice", "date:2006"},
		},
		{
			name:   "text label missing from the map",
			tmpl:   "{text:Project}/{text:Period}",
			values: Values{Text: map[string]string{"Project": "Alpha"}},
			want:   []string{"text:Period"},
		},
		{
			name:   "value made of spaces counts as missing",
			tmpl:   "{amount}/{original}",
			values: Values{Amount: "   ", Original: "x"},
			want:   []string{"amount"},
		},
		{
			name:   "amount and original",
			tmpl:   "{amount}_{original}",
			values: Values{},
			want:   []string{"amount", "original"},
		},
		{
			name:   "literal only needs nothing",
			tmpl:   "receipts/fixed.pdf",
			values: Values{},
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustParse(t, tt.tmpl).Missing(tt.values)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Missing(%q) = %v, want %v", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	valid := []string{
		planTmpl,
		"{type}/{original}.pdf",
		"{text:Project}/{type}.pdf",
		"{date:2006/01}/{invoice}.pdf",
		"receipts/{amount}_{invoice}.pdf",
	}
	for _, s := range valid {
		t.Run("valid "+s, func(t *testing.T) {
			if err := mustParse(t, s).Validate(); err != nil {
				t.Errorf("Validate(%q) = %v, want nil", s, err)
			}
		})
	}

	invalid := []struct {
		tmpl    string
		wantErr error
	}{
		{"/abs/{invoice}.pdf", ErrUnsafePath},
		{"../{invoice}.pdf", ErrUnsafePath},
		{"a/../b/{invoice}.pdf", ErrUnsafePath},
		{"{date:..}/{invoice}.pdf", ErrUnsafePath},
		{"a//{invoice}.pdf", ErrUnsafePath},
		{"{invoice}/", ErrUnsafePath},
		{`a\b/{invoice}.pdf`, ErrUnsafePath},
		{"a:b/{invoice}.pdf", ErrUnsafePath},
		{"", ErrInvalidTemplate},
	}
	for _, tt := range invalid {
		t.Run("invalid "+tt.tmpl, func(t *testing.T) {
			err := mustParse(t, tt.tmpl).Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate(%q) = %v, want %v", tt.tmpl, err, tt.wantErr)
			}
		})
	}

	t.Run("zero template", func(t *testing.T) {
		if err := (Template{}).Validate(); !errors.Is(err, ErrInvalidTemplate) {
			t.Errorf("Validate(zero) = %v, want ErrInvalidTemplate", err)
		}
	})
}

func TestUniquePath(t *testing.T) {
	set := func(paths ...string) func(string) bool {
		m := make(map[string]bool, len(paths))
		for _, p := range paths {
			m[p] = true
		}
		return func(p string) bool { return m[p] }
	}
	tests := []struct {
		name   string
		rel    string
		exists func(string) bool
		want   string
	}{
		{"no collision", "a/b.pdf", set(), "a/b.pdf"},
		{"one collision", "a/b.pdf", set("a/b.pdf"), "a/b_2.pdf"},
		{"two collisions", "a/b.pdf", set("a/b.pdf", "a/b_2.pdf"), "a/b_3.pdf"},
		{"gap is filled", "a/b.pdf", set("a/b.pdf", "a/b_3.pdf"), "a/b_2.pdf"},
		{"no extension", "a/b", set("a/b"), "a/b_2"},
		{"dot in directory name", "a.v2/b", set("a.v2/b"), "a.v2/b_2"},
		{"dot in directory only", "a.v2/b.pdf", set(), "a.v2/b.pdf"},
		{"double extension", "x.tar.gz", set("x.tar.gz"), "x.tar_2.gz"},
		{"unrelated files ignored", "a/b.pdf", set("a/b_2.pdf", "a/c.pdf"), "a/b.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UniquePath(tt.rel, tt.exists); got != tt.want {
				t.Errorf("UniquePath(%q) = %q, want %q", tt.rel, got, tt.want)
			}
		})
	}
}
