package analysis

import "testing"

const statementText = `Kontoauszug
Erstellt am 31.10.2025
Buchungstag Valuta Text Betrag
30.04.2025 30.04.2025 Gehalt 1.000,00
15.04.2025 16.04.2025 Miete -500,00
`

func TestStatementDateMostMentionedMonth(t *testing.T) {
	cands := dates(statementText, "de")
	got, ok := StatementDate(cands)
	if !ok || !got.Date.Equal(ymd(2025, 4, 30)) || got.Label != StatementLabel {
		t.Fatalf("got %+v ok=%v, want 2025-04-30", got, ok)
	}
	if got.Score < 0.85 {
		t.Errorf("score %v: a statement date must be prefill-worthy", got.Score)
	}
}

func TestStatementDateCountsOccurrences(t *testing.T) {
	// April: 30.04. twice (booking, valuta) = 2; October once, so April wins
	// although only one distinct date per day is kept in the candidates.
	text := "Kontoauszug\n30.04.2025 30.04.2025 Gehalt 1,00\n31.10.2025\n"
	got, ok := StatementDate(dates(text, "de"))
	if !ok || !got.Date.Equal(ymd(2025, 4, 30)) {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestStatementDateTieTakesLaterMonth(t *testing.T) {
	text := "03.03.2025 Kauf\n28.03.2025 Kauf\n05.04.2025 Kauf\n12.04.2025 Kauf\n"
	got, ok := StatementDate(dates(text, "de"))
	if !ok || !got.Date.Equal(ymd(2025, 4, 12)) {
		t.Fatalf("got %+v ok=%v, want 2025-04-12 (later month, latest date in it)", got, ok)
	}
}

func TestStatementDateIgnoresFallbackAndEmpty(t *testing.T) {
	if _, ok := StatementDate(nil); ok {
		t.Error("no candidates must give ok=false")
	}
	fb := []DateCandidate{{Date: ymd(2025, 5, 1), Fallback: true}}
	if _, ok := StatementDate(fb); ok {
		t.Error("a fallback date alone is no statement date")
	}
}
