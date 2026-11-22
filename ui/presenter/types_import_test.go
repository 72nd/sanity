package presenter

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/domain"
)

// workedSnippet is the worked example of PLAN.md section 3a.
const workedSnippet = `{
  "type": {
    "id": "hetzner-invoice",
    "name": "Hetzner Invoice",
    "pathTemplate": "receipts/{date:2006}/hosting/{date:2006-01-02}_hetzner_{invoice}.pdf",
    "rules": [
      {"kind": "domain",  "value": "hetzner.com", "mode": "should", "weight": 0.9, "where": "header", "enabled": true},
      {"kind": "vatId",   "value": "DE812871812", "mode": "must",   "weight": 0.9, "where": "any",    "enabled": true},
      {"kind": "keyword", "value": "rechnung",    "mode": "should", "weight": 0.2, "where": "header", "enabled": true}
    ],
    "threshold": 0.8,
    "useLearned": true,
    "date": {"preferredLabels": ["Rechnungsdatum"], "ignoreLabels": ["Fälligkeitsdatum"], "source": "auto", "maxAgeYears": 2}
  },
  "notes": [
    {"rule": 0, "why": "Sender domain in the letterhead"},
    {"warning": "The IBAN in the sample was redacted, so no iban rule was created."}
  ]
}`

// fakeImport replaces every ImportFuncs piece with a recording fake.
func fakeImport(p *Types, rep analysis.Report) *analysis.CheckInput {
	var got analysis.CheckInput
	p.Import = ImportFuncs{
		Parse: func(b []byte) (domain.DocumentType, []analysis.SnippetNote, error) {
			if strings.Contains(string(b), "bad") {
				return domain.DocumentType{}, nil, errors.New("boom")
			}
			return domain.DocumentType{ID: "telekom-invoice", Name: "New", PathTemplate: "a/{date:2006}/x.pdf",
					Rules: []domain.Rule{{Kind: "keyword", Value: "x", Mode: "should", Weight: 0.5, Where: "any", Enabled: true}}},
				[]analysis.SnippetNote{{Rule: 0, Why: "because"}, {Warning: "careful"}}, nil
		},
		Check:    func(in analysis.CheckInput) analysis.Report { got = in; return rep },
		TypeJSON: func(t domain.DocumentType) ([]byte, error) { return []byte("json:" + t.ID), nil },
		Prompt:   func(ids []string) string { return "prompt:" + strings.Join(ids, ",") },
		Extract: func(ctx context.Context, path string) (string, error) {
			return "text of " + path + " DE123456789", nil
		},
	}
	return &got
}

func TestImportCheckBuildsInputAndGatesSave(t *testing.T) {
	p, _ := newTypes(t)
	got := fakeImport(p, analysis.Report{Findings: []analysis.Finding{{Level: analysis.LevelOK, Code: "id.ok", RuleIdx: -1}}})
	p.ImportReset()
	if p.CanSave() {
		t.Error("CanSave before any check")
	}
	p.SetSnippet("{}")
	if err := p.AddSample(context.Background(), "/x/sample.pdf"); err != nil {
		t.Fatal(err)
	}
	if err := p.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(got.Existing) != 5 || len(got.Library) == 0 || len(got.IgnoreTokens) != 3 || len(got.Samples) != 1 || got.Samples[0].Name != "sample.pdf" {
		t.Errorf("input: existing %d library %d tokens %d samples %+v", len(got.Existing), len(got.Library), len(got.IgnoreTokens), got.Samples)
	}
	if !p.CanSave() {
		t.Error("CanSave false without errors")
	}
	if rows := p.ImportRules(); len(rows) != 1 || rows[0].Why != "because" {
		t.Errorf("rules %+v", rows)
	}
	if w := p.ImportWarnings(); len(w) != 1 || w[0] != "careful" {
		t.Errorf("warnings %v", w)
	}
	p.SetSnippet("{ }") // a change invalidates the check
	if p.CanSave() {
		t.Error("CanSave after the snippet changed")
	}
}

func TestImportErrorsBlockSave(t *testing.T) {
	p, _ := newTypes(t)
	fakeImport(p, analysis.Report{Findings: []analysis.Finding{{Level: analysis.LevelError, Code: "rule.regex", RuleIdx: 0}}})
	p.ImportReset()
	p.SetSnippet("{}")
	if err := p.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p.CanSave() {
		t.Error("CanSave with an error finding")
	}
	if _, err := p.ImportType(context.Background(), false); err == nil {
		t.Error("ImportType stored a type with errors")
	}
	p.SetSnippet("bad")
	if err := p.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("parse error = %v", err)
	}
}

func TestImportCollisionNewIDAndReplace(t *testing.T) {
	p, st := newTypes(t)
	collide := analysis.Report{Findings: []analysis.Finding{{Level: analysis.LevelError, Code: "id.collision", RuleIdx: -1, Argument: "telekom-invoice-2"}}}
	fakeImport(p, collide)
	ctx := context.Background()
	p.ImportReset()
	p.SetSnippet("{}")
	if err := p.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if !p.Collides() || !p.CanSave() {
		t.Fatalf("collides %v canSave %v", p.Collides(), p.CanSave())
	}
	id, err := p.ImportType(ctx, true)
	if err != nil || id != "telekom-invoice-2" {
		t.Fatalf("new id: %q, %v", id, err)
	}
	if p.CurrentID() != id || p.Dirty() {
		t.Errorf("not selected: current %q dirty %v", p.CurrentID(), p.Dirty())
	}
	types, _ := st.Types(ctx)
	if len(types) != 6 {
		t.Fatalf("types = %d, want 6", len(types))
	}

	p.ImportReset()
	p.SetSnippet("{}")
	if err := p.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.SetCollision("replace"); err != nil {
		t.Fatal(err)
	}
	if id, err = p.ImportType(ctx, false); err != nil || id != "telekom-invoice" {
		t.Fatalf("replace: %q, %v", id, err)
	}
	if types, _ = st.Types(ctx); len(types) != 6 {
		t.Errorf("replace added a type: %d", len(types))
	}
	if cur, _ := p.Current(); cur.Name != "New" {
		t.Errorf("selected type name %q", cur.Name)
	}
}

func TestImportPrivacyHits(t *testing.T) {
	p, _ := newTypes(t)
	fakeImport(p, analysis.Report{})
	ctx := context.Background()
	p.ImportReset()
	p.SetSnippet("my iban de89 3704 0044 0532 0130 00 here")
	if hits := p.PrivacyHits(ctx); len(hits) != 1 || hits[0] != "DE89370400440532013000" {
		t.Errorf("snippet hits %v", hits)
	}
	if err := p.AddSample(ctx, "/s.pdf"); err != nil { // the fake sample text has the VAT ID
		t.Fatal(err)
	}
	if hits := p.PrivacyHits(ctx); len(hits) != 2 {
		t.Errorf("hits with sample %v", hits)
	}
	p.ImportReset()
	if hits := p.PrivacyHits(ctx); len(hits) != 0 {
		t.Errorf("hits after reset %v", hits)
	}
}

func TestImportCopyJSONAndPrompt(t *testing.T) {
	p, _ := newTypes(t)
	fakeImport(p, analysis.Report{})
	js, err := p.CopyTypeJSON()
	if err != nil || !strings.HasPrefix(js, "json:") {
		t.Errorf("json %q, %v", js, err)
	}
	if _, err := p.TypeJSONOf("nope"); !errors.Is(err, ErrNoType) {
		t.Errorf("unknown id: %v", err)
	}
	if got := p.PromptText(); !strings.HasPrefix(got, "prompt:") || strings.Count(got, ",") != 4 {
		t.Errorf("prompt %q", got)
	}
}

func TestFindingTextFallsBackToMessage(t *testing.T) {
	p, _ := newTypes(t) // stubT returns the id, so the English message is used
	f := analysis.Finding{Code: "rule.regex", Message: "bad regex", RuleIdx: 1}
	if got := p.FindingText(f); got != "bad regex" {
		t.Errorf("text %q", got)
	}
	p.T = func(id string, kv ...any) string { return "L:" + id }
	if got := p.FindingText(f); got != "L:types.check.rule.regex" {
		t.Errorf("localized %q", got)
	}
}

// The real pieces: the worked example of the plan passes and is stored.
func TestImportWorkedExampleWithRealChecks(t *testing.T) {
	p, st := newTypes(t)
	ctx := context.Background()
	p.ImportReset()
	p.SetSnippet(workedSnippet)
	if err := p.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if !p.CanSave() {
		t.Fatalf("worked example blocked: %+v", p.Report().Findings)
	}
	if pv := p.ImportPreview(); pv.Invalid || !strings.Contains(pv.Path, "hetzner") {
		t.Errorf("preview %+v", pv)
	}
	if rows := p.ImportRules(); len(rows) != 3 || rows[0].Why == "" || rows[1].Why != "" {
		t.Errorf("rules %+v", rows)
	}
	if len(p.ImportWarnings()) != 1 {
		t.Errorf("warnings %v", p.ImportWarnings())
	}
	id, err := p.ImportType(ctx, true)
	if err != nil || id != "hetzner-invoice" {
		t.Fatalf("import: %q, %v", id, err)
	}
	if types, _ := st.Types(ctx); len(types) != 6 {
		t.Errorf("types = %d", len(types))
	}
	if js, err := p.CopyTypeJSON(); err != nil || !strings.Contains(js, "hetzner-invoice") {
		t.Errorf("json %q, %v", js, err)
	}
	if !strings.Contains(p.PromptText(), "hetzner-invoice") {
		t.Error("prompt lacks the new id")
	}
}
