package analysis

import (
	"strings"
	"testing"

	"github.com/72nd/sanity/domain"
)

const envelope = `{"type":{"id":"x","name":"X","pathTemplate":"a/{date:2006}.pdf",
"rules":[{"kind":"domain","value":"x.com","mode":"should","weight":"strong","where":"header"},
{"kind":"keyword","value":"rechnung","mode":"should","weight":"weak","where":"any","enabled":false},
{"kind":"keyword","value":"abc","mode":"must","weight":"normal","where":"any","enabled":true},
{"kind":"keyword","value":"num","mode":"should","weight":0.33,"where":"any"}],
"useLearned":false,"date":{"preferredLabels":["Datum"]}},
"notes":[{"rule":0,"why":"letterhead"},{"warning":"no iban"}]}`

func TestParseSnippetEnvelope(t *testing.T) {
	ty, notes, err := ParseSnippet([]byte(envelope))
	if err != nil {
		t.Fatal(err)
	}
	if ty.Threshold != 0.8 || ty.UseLearned || ty.Date.Source != "auto" || ty.Date.PreferredLabels[0] != "Datum" {
		t.Fatalf("%+v", ty)
	}
	wantEn := []bool{true, false, true, true}
	wantW := []float64{0.9, 0.2, 0.5, 0.33}
	for i, r := range ty.Rules {
		if r.Enabled != wantEn[i] || r.Weight != wantW[i] {
			t.Errorf("rule %d: %+v", i, r)
		}
	}
	if len(notes) != 2 || notes[0] != (SnippetNote{Rule: 0, Why: "letterhead"}) || notes[1].Rule != -1 || notes[1].Warning != "no iban" {
		t.Fatalf("%+v", notes)
	}
}

func TestParseSnippetBareAndDefaults(t *testing.T) {
	ty, notes, err := ParseSnippet([]byte(`{"id":"b","name":"B","pathTemplate":"x/{original}.pdf","rules":[{"kind":"keyword","value":"k","mode":"should","weight":0.5,"where":"any"}]}`))
	if err != nil || notes != nil {
		t.Fatal(err, notes)
	}
	if !ty.UseLearned || ty.Threshold != 0.8 || ty.Date.Source != "auto" || !ty.Rules[0].Enabled {
		t.Fatalf("%+v", ty)
	}
	ty, _, _ = ParseSnippet([]byte(`{"id":"b","useLearned":true,"threshold":0.6,"date":{"source":"text"}}`))
	if ty.Threshold != 0.6 || ty.Date.Source != "text" {
		t.Fatalf("%+v", ty)
	}
}

func TestParseSnippetTolerant(t *testing.T) {
	in := "Sure! Here is the type:\n```json\n" + envelope + "\n```\nLet me know {if} you need more."
	ty, notes, err := ParseSnippet([]byte(in))
	if err != nil || ty.ID != "x" || len(notes) != 2 {
		t.Fatal(err, ty, notes)
	}
	// braces inside strings do not confuse the scanner
	ty, _, err = ParseSnippet([]byte(`text {"id":"a}b","name":"q\"}"} tail`))
	if err != nil || ty.ID != "a}b" || ty.Name != `q"}` {
		t.Fatal(err, ty)
	}
}

func TestParseSnippetErrors(t *testing.T) {
	for _, in := range []string{"", "no json", `{"id":`, `{"id":1}`, `{"rules":[{"weight":"huge"}]}`, `{"type":{"id":"a"},"notes":5}`} {
		if _, _, err := ParseSnippet([]byte(in)); err == nil {
			t.Errorf("%q: want error", in)
		}
	}
}

func TestTypeJSONRoundTrip(t *testing.T) {
	ty := domain.DocumentType{ID: "a", Name: "A", PathTemplate: "x/{original}.pdf", Threshold: 0.7,
		Rules: []domain.Rule{{Kind: "keyword", Value: "v", Mode: "not", Weight: 0.4, Where: "any", Enabled: false}}}
	b, err := TypeJSON(ty)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "\n  \"id\"") || strings.Contains(string(b), `"type"`) {
		t.Fatalf("%s", b)
	}
	back, _, err := ParseSnippet(b)
	if err != nil {
		t.Fatal(err)
	}
	if back.Rules[0].Enabled || back.Rules[0].Mode != "not" || back.Threshold != 0.7 || back.UseLearned {
		// UseLearned false was written explicitly, so it must survive.
		t.Fatalf("%+v", back)
	}
}

func TestSnippetNewFieldsRoundTrip(t *testing.T) {
	ty, _, err := ParseSnippet([]byte(`{"id":"k","name":"K","pathTemplate":"a/{date:2006}.pdf","rules":[],
"statement":true,"currency":" chf ","amount":{"preferredLabels":["Betrag","Total"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !ty.IsStatement || ty.Currency != "CHF" || strings.Join(ty.Amount.PreferredLabels, "|") != "Betrag|Total" {
		t.Fatalf("%+v", ty)
	}
	data, err := TypeJSON(ty)
	if err != nil {
		t.Fatal(err)
	}
	back, _, err := ParseSnippet(data)
	if err != nil {
		t.Fatal(err)
	}
	if !back.IsStatement || back.Currency != "CHF" || strings.Join(back.Amount.PreferredLabels, "|") != "Betrag|Total" {
		t.Errorf("round trip lost fields: %s", data)
	}
}
