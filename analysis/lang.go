package analysis

import (
	"regexp"
	"strings"
)

var langWords = regexp.MustCompile(`[\p{L}]+`)

var (
	deStop = setOf("der", "die", "das", "und", "ist", "nicht", "mit", "für", "fuer", "von", "vom", "zum", "zur",
		"rechnung", "datum", "betrag", "bitte", "wir", "sie", "ihre", "ihr", "ihnen", "auf", "den", "dem", "des",
		"ein", "eine", "einen", "bei", "bis", "nach", "gesamt", "summe", "kunde", "kundennummer", "zahlung",
		"mwst", "ust", "vielen", "dank", "sehr", "geehrte", "grüße", "gruesse", "freundlichen", "im", "am", "zu")
	enStop = setOf("the", "and", "is", "not", "with", "for", "from", "to", "of", "invoice", "date", "amount",
		"please", "we", "you", "your", "on", "in", "at", "by", "total", "due", "payment", "thank", "customer",
		"account", "order", "tax", "this", "that", "are", "was", "will", "be", "paid", "billing", "receipt")
)

func setOf(w ...string) map[string]bool {
	m := make(map[string]bool, len(w))
	for _, s := range w {
		m[s] = true
	}
	return m
}

// DetectLanguage returns "de" or "en" by counting stop words. Ties and empty
// text give "en".
func DetectLanguage(text string) string {
	de, en := 0, 0
	for _, w := range langWords.FindAllString(strings.ToLower(text), -1) {
		if deStop[w] {
			de++
		}
		if enStop[w] {
			en++
		}
	}
	if de > en {
		return "de"
	}
	return "en"
}
