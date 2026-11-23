package analysis

import (
	"strings"
	"testing"

	"github.com/72nd/sanity/domain"
)

func statementTypes() []domain.DocumentType {
	types := testTypes()
	return append(types, domain.DocumentType{ID: "bank", Threshold: 0.8, IsStatement: true, Rules: []domain.Rule{
		{Kind: "keyword", Value: "Musterbank", Mode: "should", Weight: 0.9, Where: "any", Enabled: true},
	}})
}

// A statement lists a payment to Telekom; the Telekom type must not win.
const bankStatement = `Musterbank AG
Kontoauszug 04/2025
30.04.2025 30.04.2025 Telekom Deutschland GmbH Lastschrift -49,95
15.04.2025 16.04.2025 Gehalt 2.000,00
`

func TestStatementKeywordSuppressesOtherTypes(t *testing.T) {
	doc := Doc{Text: bankStatement, StatementKeywords: domain.DefaultStatementKeywords}
	got := Classify(doc, statementTypes(), nil)
	if len(got) == 0 || got[0].TypeID != "bank" || !got[0].Confident {
		t.Fatalf("want the statement type confident first, got %+v", got)
	}
	for _, s := range got {
		if s.TypeID == "telekom" && (s.Confident || s.Score >= defaultThreshold) {
			t.Errorf("telekom must be suppressed, got %+v", s)
		}
	}
	// without the keyword list nothing is adjusted: Telekom keeps its score
	for _, sg := range Classify(Doc{Text: bankStatement}, statementTypes(), nil) {
		if sg.TypeID == "telekom" && sg.Score < defaultThreshold {
			t.Errorf("no keywords, no adjustment: %+v", sg)
		}
	}
}

func TestStatementKeywordNeedsFlaggedType(t *testing.T) {
	// no type has the flag: the keyword changes nothing
	doc := Doc{Text: telekomText + "\nKontoauszug", StatementKeywords: domain.DefaultStatementKeywords}
	got := Classify(doc, testTypes(), nil)
	if len(got) == 0 || got[0].TypeID != "telekom" || !got[0].Confident {
		t.Fatalf("got %+v", got)
	}
}

func TestStatementKeywordAbsentLeavesInvoices(t *testing.T) {
	doc := Doc{Text: telekomText, StatementKeywords: domain.DefaultStatementKeywords}
	got := Classify(doc, statementTypes(), nil)
	if len(got) == 0 || got[0].TypeID != "telekom" || !got[0].Confident {
		t.Fatalf("an invoice without keyword stays confident: %+v", got)
	}
}

func TestHasStatementKeyword(t *testing.T) {
	kws := []string{"Kontoauszug", " Account statement ", ""}
	for text, want := range map[string]bool{
		"KONTOAUSZUG Nr. 4":       true,
		"your account  statement": false, // double space is not normalized away
		"Your Account Statement":  true,
		"Rechnung":                false,
		strings.Repeat("x", 3):    false,
	} {
		if got := HasStatementKeyword(text, kws); got != want {
			t.Errorf("%q: got %v want %v", text, got, want)
		}
	}
	if HasStatementKeyword("Kontoauszug", nil) {
		t.Error("no keywords must never match")
	}
}
