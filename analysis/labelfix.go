package analysis

import (
	"strings"
	"unicode"
)

// fixableLabels are the labels whose OCR misreadings FixLabels repairs. They
// are the ones date and invoice detection hang on.
var fixableLabels = []string{
	"Invoice date", "Invoice number", "Date of issue", "Due date",
	"Rechnungsdatum", "Rechnungsnummer", "Belegdatum", "Rechnungsnr.",
}

// FixLabels replaces short lines that are an OCR misreading of a known label
// ("INVOIGE DAIE" for "Invoice date") by the label itself, so that the label
// regexes of the extractors match. A line counts as a misreading when it has
// no digits and differs from the label by at most a sixth of its length in
// edits. Lines that equal a label, and everything else, stay as they are.
// Offsets of the result differ from the input: use it for extraction only.
func FixLabels(text string) string {
	lines := strings.Split(text, "\n")
	changed := false
	for i, l := range lines {
		t := strings.TrimSpace(strings.TrimRight(l, ":"))
		if n := len([]rune(t)); n < 6 || n > 20 || strings.IndexFunc(t, unicode.IsDigit) >= 0 {
			continue
		}
		lt := strings.ToLower(t)
		for _, lab := range fixableLabels {
			ll := strings.ToLower(lab)
			if lt == ll {
				break
			}
			if editDistance(lt, ll) <= len([]rune(ll))/6 {
				lines[i], changed = lab, true
				break
			}
		}
	}
	if !changed {
		return text
	}
	return strings.Join(lines, "\n")
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
