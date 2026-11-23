package analysis

import (
	"strings"
	"testing"

	"github.com/72nd/sanity/domain"
)

func goodType() domain.DocumentType {
	return domain.DocumentType{
		ID: "hetzner", Name: "Hetzner", PathTemplate: "receipts/{date:2006}/{date:2006-01-02}_hetzner_{invoice}.pdf",
		Threshold: 0.8, UseLearned: true,
		Rules: []domain.Rule{
			{Kind: "domain", Value: "hetzner.com", Mode: "should", Weight: 0.9, Where: "header", Enabled: true},
			{Kind: "keyword", Value: "rechnung", Mode: "should", Weight: 0.2, Where: "header", Enabled: true},
		},
		Date: domain.DateConfig{Source: "auto"},
	}
}

func find(r Report, code string) (Finding, bool) {
	for _, f := range r.Findings {
		if f.Code == code {
			return f, true
		}
	}
	return Finding{}, false
}

func TestCheckTypeStatic(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*domain.DocumentType)
		code string
		lvl  Level
		idx  int
	}{
		{"bad kind", func(d *domain.DocumentType) { d.Rules[0].Kind = "magic" }, "rule.kind", LevelError, 0},
		{"bad mode", func(d *domain.DocumentType) { d.Rules[1].Mode = "maybe" }, "rule.mode", LevelError, 1},
		{"bad where", func(d *domain.DocumentType) { d.Rules[0].Where = "middle" }, "rule.where", LevelError, 0},
		{"bad source", func(d *domain.DocumentType) { d.Date.Source = "oracle" }, "date.source", LevelError, -1},
		{"bad regex", func(d *domain.DocumentType) {
			d.Rules[0] = domain.Rule{Kind: "regex", Value: "(", Mode: "should", Weight: 0.5, Where: "any", Enabled: true}
		}, "rule.regex", LevelError, 0},
		{"weight high", func(d *domain.DocumentType) { d.Rules[0].Weight = 1.5 }, "rule.weight", LevelError, 0},
		{"weight neg", func(d *domain.DocumentType) { d.Rules[0].Weight = -0.1 }, "rule.weight", LevelError, 0},
		{"empty id", func(d *domain.DocumentType) { d.ID = "" }, "id.empty", LevelError, -1},
		{"empty name", func(d *domain.DocumentType) { d.Name = " " }, "name.empty", LevelError, -1},
		{"template unknown", func(d *domain.DocumentType) { d.PathTemplate = "a/{foo}.pdf" }, "template.invalid", LevelError, -1},
		{"template escapes", func(d *domain.DocumentType) { d.PathTemplate = "../x/{date:2006}.pdf" }, "template.unsafe", LevelError, -1},
		{"template absolute", func(d *domain.DocumentType) { d.PathTemplate = "/x/{date:2006}.pdf" }, "template.unsafe", LevelError, -1},
		{"no enabled", func(d *domain.DocumentType) { d.Rules[0].Enabled, d.Rules[1].Enabled = false, false }, "rules.none", LevelError, -1},
		{"empty value", func(d *domain.DocumentType) { d.Rules[0].Value = "" }, "rule.empty", LevelError, 0},
		{"threshold", func(d *domain.DocumentType) { d.Threshold = 2 }, "threshold.range", LevelError, -1},
		{"redacted", func(d *domain.DocumentType) { d.Rules[0].Value = "[REDACTED]" }, "rule.redacted", LevelError, 0},
		{"xxxx", func(d *domain.DocumentType) {
			d.Rules[0] = domain.Rule{Kind: "iban", Value: "DE89 XXXX 0000", Mode: "should", Weight: 0.9, Where: "any", Enabled: true}
		}, "rule.redacted", LevelError, 0},
		{"stars", func(d *domain.DocumentType) { d.Rules[0].Value = "DE***" }, "rule.redacted", LevelError, 0},
		{"angle", func(d *domain.DocumentType) { d.Rules[0].Value = "<redacted>" }, "rule.redacted", LevelError, 0},
		{"block", func(d *domain.DocumentType) { d.Rules[0].Value = "▇▇▇▇" }, "rule.redacted", LevelError, 0},
		{"weak", func(d *domain.DocumentType) { d.Rules[0].Weight = 0.2 }, "rules.weak", LevelWarning, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ty := goodType()
			c.mut(&ty)
			r := CheckType(CheckInput{Type: ty})
			f, ok := find(r, c.code)
			if !ok || f.Level != c.lvl || f.RuleIdx != c.idx {
				t.Fatalf("want %s %s idx %d, got %+v", c.lvl, c.code, c.idx, r.Findings)
			}
			if c.lvl == LevelError && !r.HasErrors() {
				t.Fatal("HasErrors false")
			}
		})
	}
}

func TestCheckTypeGoodHasOKs(t *testing.T) {
	r := CheckType(CheckInput{Type: goodType()})
	if r.HasErrors() {
		t.Fatalf("errors: %+v", r.Findings)
	}
	for _, code := range []string{"id.ok", "template.ok", "enums.ok", "weights.ok", "regex.ok", "redaction.ok", "rules.ok"} {
		if f, ok := find(r, code); !ok || f.Level != LevelOK {
			t.Errorf("missing ok %s: %+v", code, r.Findings)
		}
	}
}

func TestCheckTypeIDCollision(t *testing.T) {
	ex := []domain.DocumentType{{ID: "hetzner"}, {ID: "hetzner-2"}}
	r := CheckType(CheckInput{Type: goodType(), Existing: ex})
	f, ok := find(r, "id.collision")
	if !ok || f.Level != LevelError || f.Argument != "hetzner-3" {
		t.Fatalf("%+v", r.Findings)
	}
	if _, ok := find(r, "id.ok"); ok {
		t.Fatal("id.ok next to a collision")
	}
}

func TestCheckTypeIgnoreToken(t *testing.T) {
	ty := goodType()
	ty.Rules = append(ty.Rules, domain.Rule{Kind: "iban", Value: "DE89 3704 0044 0532 0130 00", Mode: "should", Weight: 0.9, Where: "any", Enabled: true})
	r := CheckType(CheckInput{Type: ty, IgnoreTokens: []string{"DE89370400440532013000"}})
	f, ok := find(r, "rule.ignoreToken")
	if !ok || f.Level != LevelWarning || f.RuleIdx != 2 {
		t.Fatalf("%+v", r.Findings)
	}
	if r.HasErrors() {
		t.Fatal("a warning blocked")
	}
}

func TestCheckTypeSamples(t *testing.T) {
	other := domain.DocumentType{ID: "invoice", Name: "Invoice", Rules: []domain.Rule{
		{Kind: "keyword", Value: "rechnung", Mode: "should", Weight: 0.5, Where: "any", Enabled: true},
	}}
	samples := []Sample{
		{Name: "a.pdf", Text: "Hetzner Online GmbH\nhetzner.com\nRechnung 123"},
		{Name: "b.pdf", Text: "Something else\nRechnung 5"},
	}
	r := CheckType(CheckInput{Type: goodType(), Existing: []domain.DocumentType{other}, Samples: samples})
	if len(r.Samples) != 2 {
		t.Fatalf("%+v", r.Samples)
	}
	a := r.Samples[0]
	if a.Score < 0.9 || !a.Confident || len(a.Others) != 1 || a.Others[0].TypeID != "invoice" {
		t.Fatalf("a: %+v", a)
	}
	b := r.Samples[1]
	if b.Score < 0.19 || b.Score > 0.21 || b.Confident || len(b.Others) != 1 {
		t.Fatalf("b: %+v", b)
	}
	if f, ok := find(r, "sample.weak"); !ok || f.Argument != "b.pdf" || f.Level != LevelWarning {
		t.Fatalf("%+v", r.Findings)
	}
	if r.HasErrors() {
		t.Fatal("dry run must only warn")
	}
}

func TestCheckTypeLibrary(t *testing.T) {
	ty := goodType()
	ty.Rules = []domain.Rule{{Kind: "keyword", Value: "telekom", Mode: "should", Weight: 0.9, Where: "any", Enabled: true}}
	lib := []domain.LibraryEntry{
		{ID: "1", TypeID: "other", Path: "x/1.pdf", SHA256: "s1"},
		{ID: "2", TypeID: "other", Path: "x/2.pdf", SHA256: "s2"},
		{ID: "3", TypeID: "other", Path: "x/3.pdf", SHA256: "s3"}, // no text
		{ID: "4", TypeID: ty.ID, Path: "x/4.pdf", SHA256: "s4"},   // same type: not a false positive
	}
	texts := map[string]string{"s1": "Deutsche Telekom Rechnung", "s2": "Hetzner", "s4": "telekom"}
	r := CheckType(CheckInput{Type: ty, Library: lib, Text: func(s string) (string, bool) { v, ok := texts[s]; return v, ok }})
	if len(r.Library) != 1 || r.Library[0].EntryID != "1" || r.Library[0].TypeID != "other" || r.Library[0].Score < 0.89 {
		t.Fatalf("%+v", r.Library)
	}
	if r.Skipped != 1 {
		t.Fatalf("skipped %d", r.Skipped)
	}
	if _, ok := find(r, "library.falsePositive"); !ok {
		t.Fatalf("%+v", r.Findings)
	}
	// Without a text source the library check is skipped.
	r = CheckType(CheckInput{Type: ty, Library: lib})
	if len(r.Library) != 0 || r.Skipped != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestEnumsDocumented(t *testing.T) {
	for _, l := range [][]string{RuleKinds, RuleModes, RuleWheres, DateSources} {
		if len(l) == 0 || strings.Join(l, "") == "" {
			t.Fatal("empty enum")
		}
	}
}

func TestCheckTypeCurrencyAndSampleFields(t *testing.T) {
	ty := goodType()
	ty.Currency = "XYZ"
	if f, ok := find(CheckType(CheckInput{Type: ty}), "currency.unknown"); !ok || f.Level != LevelError {
		t.Fatal("unknown currency accepted")
	}
	ty.Currency = "CHF"
	ty.IsStatement = true
	text := "Hetzner Online GmbH\nhetzner.com\nKontoauszug\n30.04.2025 Zahlung 90.00\n15.04.2025 Zahlung 12.50\n31.10.2025\n"
	r := CheckType(CheckInput{Type: ty, Samples: []Sample{{Name: "s.pdf", Text: text}}})
	if len(r.Samples) != 1 || r.Samples[0].Date != "2025-04-30" || r.Samples[0].Amount == "" || !strings.HasSuffix(r.Samples[0].Amount, "CHF") {
		t.Errorf("sample result %+v", r.Samples)
	}
}
