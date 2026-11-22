package analysis

import (
	"testing"
	"time"

	"github.com/72nd/sanity/domain"
)

const telekomText = `Telekom Deutschland GmbH
Landgrabenweg 151
53227 Bonn

Rechnung
Kundennummer: 123456789
Rechnungsnummer 4500123401
Rechnungsdatum 08.01.2025
Mobilfunk Grundpreis 49,95 EUR
Gesamtbetrag 49,95 EUR

Telekom Deutschland GmbH, Sitz Bonn, www.telekom.de
USt-IdNr. DE123475223
IBAN DE89 3704 0044 0532 0130 00`

const vodafoneText = `Vodafone GmbH
Ferdinand-Braun-Platz 1
40549 Düsseldorf
Kundennummer 2.123.456.789
Rechnungsdatum 14.02.2025
Rechnungsnummer VF-2025-0218734
Gesamtbetrag 39,99 EUR
vodafone.de`

const appleText = `Apple Distribution International Ltd.
Hollyhill Industrial Estate, Cork
Invoice
Order ID W3214578901
Invoice Date: March 22, 2025
Total 10.99 EUR
apple.com/legal`

func ids(s []Suggestion) []string {
	var o []string
	for _, x := range s {
		o = append(o, x.TypeID)
	}
	return o
}

func testTypes() []domain.DocumentType {
	kw := func(v string, w float64, where string) domain.Rule {
		return domain.Rule{Kind: "keyword", Value: v, Mode: "should", Weight: w, Where: where, Enabled: true}
	}
	return []domain.DocumentType{
		{ID: "telekom", Threshold: 0.8, UseLearned: true, Rules: []domain.Rule{
			kw("Telekom Deutschland GmbH", 0.9, "any"),
			{Kind: "domain", Value: "telekom.de", Mode: "should", Weight: 0.2, Where: "footer", Enabled: true},
			{Kind: "vatId", Value: "DE123475223", Mode: "should", Weight: 0.9, Where: "any", Enabled: true},
		}},
		{ID: "vodafone", UseLearned: true, Rules: []domain.Rule{kw("Vodafone GmbH", 0.9, "any"), kw("Kundennummer", 0.2, "any")}},
		{ID: "apple", UseLearned: true, Rules: []domain.Rule{kw("Apple Distribution", 0.9, "header")}},
	}
}

func TestClassifyRulesOnly(t *testing.T) {
	cases := []struct {
		name, text, file, want string
		confident              bool
	}{
		{"telekom", telekomText, "", "telekom", true},
		{"vodafone", vodafoneText, "", "vodafone", true},
		{"apple", appleText, "", "apple", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(Doc{Text: c.text, FileName: c.file}, testTypes(), nil)
			if len(got) == 0 || got[0].TypeID != c.want {
				t.Fatalf("got %v", ids(got))
			}
			if got[0].Confident != c.confident {
				t.Errorf("confident = %v, score %.2f", got[0].Confident, got[0].Score)
			}
		})
	}
}

func TestClassifyNotAndSorting(t *testing.T) {
	types := testTypes()
	types[0].Rules = append(types[0].Rules, domain.Rule{Kind: "keyword", Value: "Landgrabenweg", Mode: "not", Weight: 1, Where: "any", Enabled: true})
	got := Classify(Doc{Text: telekomText}, types, nil)
	for _, s := range got {
		if s.TypeID == "telekom" {
			t.Fatal("excluded type returned")
		}
	}
	got = Classify(Doc{Text: vodafoneText + "\nApple Distribution"}, testTypes(), nil)
	for i := 1; i < len(got); i++ {
		if got[i].Score > got[i-1].Score {
			t.Fatal("not sorted")
		}
	}
}

func TestClassifyConfidentNeedsLead(t *testing.T) {
	types := []domain.DocumentType{
		{ID: "a", Rules: []domain.Rule{{Kind: "keyword", Value: "foo", Mode: "should", Weight: 0.9, Where: "any", Enabled: true}}},
		{ID: "b", Rules: []domain.Rule{{Kind: "keyword", Value: "foo", Mode: "should", Weight: 0.85, Where: "any", Enabled: true}}},
	}
	got := Classify(Doc{Text: "foo"}, types, nil)
	if got[0].Confident || got[1].Confident {
		t.Errorf("no clear lead, got %+v", got)
	}
	types[0].Threshold = 0.95
	got = Classify(Doc{Text: "foo bar"}, types[:1], nil)
	if got[0].Confident {
		t.Error("below threshold must not be confident")
	}
}

func TestClassifyLearnedCap(t *testing.T) {
	types := []domain.DocumentType{{ID: "x", UseLearned: true}, {ID: "y", UseLearned: false}}
	idx := NewIndex(nil)
	idx.Add("x", telekomText)
	idx.Add("y", telekomText)
	got := Classify(Doc{Text: telekomText}, types, idx)
	if len(got) != 1 || got[0].TypeID != "x" {
		t.Fatalf("got %+v", got)
	}
	if got[0].LearnedScore > 0.7+1e-9 || got[0].Confident {
		t.Errorf("capped learned score expected, got %+v", got[0])
	}
	idx.Add("x", telekomText)
	idx.Add("x", telekomText)
	got = Classify(Doc{Text: telekomText}, types, idx)
	if got[0].LearnedScore < 0.9 || !got[0].Confident {
		t.Errorf("with 3 filed the cap is gone, got %+v", got[0])
	}
}

func TestClassifyLearnedDiscriminates(t *testing.T) {
	idx := NewIndex([]string{"DE00 1111 2222 3333 4444 55", "Muster GmbH"})
	for i := 0; i < 3; i++ {
		idx.Add("telekom", telekomText+"\nSeite 1")
		idx.Add("vodafone", vodafoneText)
		idx.Add("apple", appleText)
	}
	types := []domain.DocumentType{{ID: "telekom", UseLearned: true}, {ID: "vodafone", UseLearned: true}, {ID: "apple", UseLearned: true}}
	got := Classify(Doc{Text: appleText + "\nOrder ID W9999999999"}, types, idx)
	if got[0].TypeID != "apple" || !got[0].Confident {
		t.Errorf("got %+v", got)
	}
}

func TestRules(t *testing.T) {
	doc := Doc{Text: telekomText, FileName: "Telekom_Rechnung_2025.pdf"}
	r := func(kind, v, mode, where string, w float64) domain.Rule {
		return domain.Rule{Kind: kind, Value: v, Mode: mode, Weight: w, Where: where, Enabled: true}
	}
	cases := []struct {
		name  string
		rules []domain.Rule
		score float64
		excl  bool
	}{
		{"keyword case-insens", []domain.Rule{r("keyword", "telekom deutschland gmbh", "should", "any", 0.5)}, 0.5, false},
		{"noisy or", []domain.Rule{r("keyword", "Rechnung", "should", "any", 0.5), r("keyword", "Bonn", "should", "any", 0.5)}, 0.75, false},
		{"header only miss", []domain.Rule{r("keyword", "Landgrabenweg", "should", "footer", 0.5)}, 0, false},
		{"footer hit", []domain.Rule{r("domain", "telekom.de", "should", "footer", 0.5)}, 0.5, false},
		{"regex", []domain.Rule{r("regex", `kundennummer:?\s*\d{8,12}`, "should", "any", 0.5)}, 0.5, false},
		{"bad regex", []domain.Rule{r("regex", `(`, "should", "any", 0.5)}, 0, false},
		{"iban spaces", []domain.Rule{r("iban", "DE89370400440532013000", "should", "any", 0.9)}, 0.9, false},
		{"vat", []domain.Rule{r("vatId", "de 123475223", "should", "any", 0.9)}, 0.9, false},
		{"customerNo", []domain.Rule{r("customerNo", "123456789", "should", "any", 0.5)}, 0.5, false},
		{"filename", []domain.Rule{r("filename", "telekom", "should", "filename", 0.5)}, 0.5, false},
		{"where filename keyword", []domain.Rule{r("keyword", "rechnung_2025", "should", "filename", 0.5)}, 0.5, false},
		{"must missing", []domain.Rule{r("keyword", "Rechnung", "should", "any", 0.9), r("keyword", "Vodafone", "must", "any", 0.5)}, 0, false},
		{"must present", []domain.Rule{r("keyword", "Bonn", "must", "any", 0.5), r("keyword", "Rechnung", "should", "any", 0.5)}, 0.75, false},
		{"not excludes", []domain.Rule{r("keyword", "Rechnung", "should", "any", 0.9), r("keyword", "Bonn", "not", "any", 1)}, 0, true},
		{"not absent", []domain.Rule{r("keyword", "Rechnung", "should", "any", 0.9), r("keyword", "Berlin", "not", "any", 1)}, 0.9, false},
		{"disabled", []domain.Rule{{Kind: "keyword", Value: "Rechnung", Mode: "should", Weight: 0.9, Where: "any"}}, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EvalRules(c.rules, doc)
			if d := got.Score - c.score; d > 1e-9 || d < -1e-9 || got.Excluded != c.excl {
				t.Errorf("got %+v, want %.2f excluded=%v", got, c.score, c.excl)
			}
		})
	}
}

func TestExplicitHeaderFooter(t *testing.T) {
	d := Doc{Text: "x", Header: []string{"Acme Corp"}, Footer: []string{"acme.io"}}
	rs := []domain.Rule{
		{Kind: "keyword", Value: "acme corp", Mode: "should", Weight: 0.5, Where: "header", Enabled: true},
		{Kind: "domain", Value: "acme.io", Mode: "should", Weight: 0.5, Where: "footer", Enabled: true},
	}
	if s := EvalRules(rs, d).Score; s < 0.74 || s > 0.76 {
		t.Errorf("got %v", s)
	}
}

func TestFingerprint(t *testing.T) {
	ignore := []string{"DE89 3704 0044 0532 0130 00", "DE123475223", "Landgrabenweg 151"}
	fp := Fingerprint(telekomText, ignore)
	for _, tok := range fp {
		if tok == "landgrabenweg" || tok == "de123475223" || tok == "151" || tok == "0044" {
			t.Errorf("ignored token %q in fingerprint", tok)
		}
	}
	has := func(s string) bool {
		for _, x := range fp {
			if x == s {
				return true
			}
		}
		return false
	}
	if !has("telekom") || !has("telekom deutschland") {
		t.Errorf("expected token and bigram, got %v", fp)
	}
	other := telekomText + "\nZusatz"
	if j := Jaccard(fp, Fingerprint(other, ignore)); j < 0.8 {
		t.Errorf("same letterhead similarity %.2f", j)
	}
	if j := Jaccard(fp, Fingerprint(vodafoneText, ignore)); j > 0.3 {
		t.Errorf("different letterhead similarity %.2f", j)
	}
	if Jaccard(nil, fp) != 0 {
		t.Error("empty set")
	}
}

func TestFingerprintFirstFifteenLastFive(t *testing.T) {
	var lines []string
	for i := 0; i < 40; i++ {
		lines = append(lines, "line"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	text := ""
	for _, l := range lines {
		text += l + "\n"
	}
	fp := Fingerprint(text, nil)
	has := map[string]bool{}
	for _, f := range fp {
		has[f] = true
	}
	if !has[lines[14]] || has[lines[15]] || has[lines[34]] || !has[lines[35]] || !has[lines[39]] {
		t.Errorf("wrong line window: %v", fp)
	}
}

func TestFingerprintIndex(t *testing.T) {
	var x FingerprintIndex
	x.Add("t", Fingerprint(telekomText, nil))
	x.Add("t", Fingerprint(vodafoneText, nil))
	x.Add("a", Fingerprint(appleText, nil))
	s := x.Score(Fingerprint(telekomText, nil))
	if s["t"] != 1 || s["a"] >= 0.3 {
		t.Errorf("got %v", s)
	}
}

func TestCorpus(t *testing.T) {
	var c Corpus
	if len(c.Score("x")) != 0 {
		t.Error("empty corpus")
	}
	c.Add("telekom", telekomText)
	c.Add("telekom", telekomText+" Mobilfunk")
	c.Add("vodafone", vodafoneText)
	c.Add("apple", appleText)
	s := c.Score(telekomText + " Roaming")
	if s["telekom"] < 0.6 || s["telekom"] <= s["vodafone"] || s["telekom"] <= s["apple"] {
		t.Errorf("got %v", s)
	}
	if c.Len() != 4 {
		t.Error("len")
	}
}

func TestFindDuplicate(t *testing.T) {
	d := time.Date(2025, 1, 8, 0, 0, 0, 0, time.UTC)
	lib := []domain.LibraryEntry{
		{ID: "1", TypeID: "telekom", Invoice: "4500123401", SHA256: "aaa", TextHash: "t1", Date: d},
		{ID: "2", TypeID: "vodafone", Invoice: "VF-1", SHA256: "bbb", TextHash: "t2", Date: d},
		{ID: "3", TypeID: "cursor", SHA256: "ccc", Date: d},
	}
	cases := []struct {
		name                  string
		sha, text, typ, inv   string
		wantID, wantReasonHas string
	}{
		{"sha", "bbb", "", "telekom", "", "2", "file"},
		{"text hash", "zzz", "t1", "vodafone", "", "1", "text"},
		{"invoice same type", "zzz", "zz", "telekom", " 4500123401 ", "1", "invoice"},
		{"invoice other type", "zzz", "zz", "vodafone", "4500123401", "", ""},
		{"vf case", "zzz", "zz", "vodafone", "vf-1", "2", "invoice"},
		{"no invoice no match", "zzz", "zz", "cursor", "", "", ""},
		{"empty hashes never match", "", "", "cursor", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, reason, ok := FindDuplicate(lib, c.sha, c.text, c.typ, c.inv)
			if ok != (c.wantID != "") || e.ID != c.wantID {
				t.Fatalf("got %v %q %v", e.ID, reason, ok)
			}
			if ok && !contains(reason, c.wantReasonHas) {
				t.Errorf("reason %q", reason)
			}
		})
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestLearnRules(t *testing.T) {
	text := `Hetzner Online GmbH
Industriestr. 25, 91710 Gunzenhausen
Kundennummer: K12345678
Rechnung R0012345
Zahlung an IBAN DE89 3704 0044 0532 0130 00
Eigene IBAN CH93 0076 2011 6238 5295 7
USt-IdNr. DE812871812   Kunde: DE123456789
Fragen an billing@hetzner.com oder www.hetzner.com, privat: max@gmail.com`
	got := LearnRules(text, []string{"CH93 0076 2011 6238 5295 7", "DE123456789"})
	type k struct{ kind, val string }
	have := map[k]domain.Rule{}
	for _, r := range got {
		have[k{r.Kind, r.Value}] = r
		if r.Mode != "should" || r.Where != "any" || r.Weight <= 0 {
			t.Errorf("bad rule %+v", r)
		}
	}
	for _, w := range []k{{"iban", "DE89370400440532013000"}, {"vatId", "DE812871812"}, {"domain", "hetzner.com"}, {"customerNo", "K12345678"}} {
		if _, ok := have[w]; !ok {
			t.Errorf("missing %v in %+v", w, got)
		}
	}
	for _, bad := range []k{{"iban", "CH9300762011623852957"}, {"vatId", "DE123456789"}, {"domain", "gmail.com"}} {
		if _, ok := have[bad]; ok {
			t.Errorf("unexpected %v", bad)
		}
	}
	if !have[k{"iban", "DE89370400440532013000"}].Enabled || !have[k{"domain", "hetzner.com"}].Enabled {
		t.Error("iban/vat/domain enabled by default")
	}
	if have[k{"customerNo", "K12345678"}].Enabled {
		t.Error("customerNo disabled by default")
	}
	n := 0
	for _, r := range got {
		if r.Kind == "domain" && r.Value == "hetzner.com" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("domain duplicated %d", n)
	}
}

func TestLearnedRulesMatchSource(t *testing.T) {
	rules := LearnRules(telekomText, nil)
	for i := range rules {
		rules[i].Enabled = true
	}
	if EvalRules(rules, Doc{Text: telekomText}).Score < 0.9 {
		t.Errorf("learned rules should match their source: %+v", rules)
	}
}
