package analysis

import (
	"regexp"
	"strings"
	"time"
	"unicode"
)

// PairedMarker starts the synthetic block that PairColumns appends.
const PairedMarker = "[paired columns]"

const maxLabelLen = 40

var (
	reInvoiceLabelLine = regexp.MustCompile(`(?i)^(?:rechnung(?:s)?[ -]*(?:nummer|nr\.?|no\.?)|invoice[ ]*(?:no\.?|number|nr\.?|#)|` +
		`inv\.?[ ]*(?:no\.?|#)|beleg-?(?:nr\.?|nummer)|re-?nr\.?|receipt[ ]*(?:no\.?|number|#)|rechnung[ ]*#)$`)
	reMoneyLine = regexp.MustCompile(`^(?:€|EUR|CHF|USD|\$|£)?[ ]?-?\d[\d.,'’ ]*[.,]\d{2}[ ]?(?:€|EUR|CHF|USD|\$|£)?$`)
	reIDLine    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9\-/.#_]{3,30}$`)
)

// canonLabels rewrites OCR variants and word orders of date labels to the
// wording FindDates knows.
var canonLabels = []struct {
	re   *regexp.Regexp
	name string
}{
	{regexp.MustCompile(`(?i)^dat[ae][ ]+of[ ]+issue$`), "Date of issue"},
	{regexp.MustCompile(`(?i)^date[ ]+due$`), "Due date"},
	{regexp.MustCompile(`(?i)^payment[ ]+date[ ]+due$`), "Due date"},
}

// PairColumns resolves label/value columns. OCR and some PDFs emit the labels
// and the values as separate blocks:
//
//	Invoice
//	Invoice number
//	Date of issue
//	56546565A-0682
//	April 16, 2026
//
// When a block of m value lines (a date, a money value or an identifier with a
// digit) directly follows at least m label-only lines, the k-th of the last m
// label lines is paired with the k-th value. This only happens when at least
// one of those labels is a known invoice number, date or amount label, and
// the labels before the last m (a heading) are no known label themselves.
//
// The original text is returned unchanged with a synthetic block appended: the
// marker line PairedMarker and one "Label: value" line per pair. Offsets into
// the original text stay valid. Without a pair the text is returned as is.
func PairColumns(text string) string {
	lines := strings.Split(text, "\n")
	var pairs []string
	for i := 0; i < len(lines); {
		if !isValueLine(lines[i]) {
			i++
			continue
		}
		j := i
		for j < len(lines) && isValueLine(lines[j]) {
			j++
		}
		pairs = append(pairs, pairBlock(lines, i, j)...)
		i = j
	}
	if len(pairs) == 0 {
		return text
	}
	return strings.TrimRight(text, "\n") + "\n\n" + PairedMarker + "\n" + strings.Join(pairs, "\n") + "\n"
}

// pairBlock pairs the value lines [from,to) with the label lines before them.
func pairBlock(lines []string, from, to int) []string {
	m := to - from
	start := from
	for start > 0 && isLabelOnly(lines[start-1]) {
		start--
	}
	run := from - start // label-only lines directly before the values
	if run < m {
		return nil // fewer labels than values: not a clean columns layout
	}
	labels := lines[from-m : from]
	known := false
	for _, l := range labels {
		known = known || isKnownLabel(l)
	}
	if !known {
		return nil
	}
	for _, l := range lines[start : from-m] {
		if isKnownLabel(l) { // a further label: the run is ambiguous
			return nil
		}
	}
	out := make([]string, 0, m)
	for k, l := range labels {
		out = append(out, canonLabel(l)+": "+strings.TrimSpace(lines[from+k]))
	}
	return out
}

func cleanLabel(s string) string {
	return strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), ":"))
}

func canonLabel(s string) string {
	s = cleanLabel(s)
	for _, c := range canonLabels {
		if c.re.MatchString(s) {
			return c.name
		}
	}
	return s
}

// isLabelOnly is a short line of text without digits that is no value.
func isLabelOnly(s string) bool {
	s = cleanLabel(s)
	if s == "" || len(s) > maxLabelLen || isValueLine(s) {
		return false
	}
	hasLetter := false
	for _, r := range s {
		if unicode.IsDigit(r) {
			return false
		}
		hasLetter = hasLetter || unicode.IsLetter(r)
	}
	return hasLetter
}

// isKnownLabel reports whether the whole line is an invoice number, date or
// amount label.
func isKnownLabel(s string) bool {
	s = canonLabel(s)
	if s == "" || len(s) > maxLabelLen {
		return false
	}
	if reInvoiceLabelLine.MatchString(s) {
		return true
	}
	covers := func(loc []int) bool { return loc != nil && loc[0] == 0 && loc[1] == len(s) }
	for _, l := range dateLabels {
		if l.weight != 0 && covers(l.re.FindStringIndex(s)) {
			return true
		}
	}
	for _, l := range amountLabels[:2] {
		if covers(l.re.FindStringIndex(s)) {
			return true
		}
	}
	return false
}

// isValueLine reports whether the line is only a date, a money value or an
// identifier that contains a digit.
func isValueLine(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 40 {
		return false
	}
	if reMoneyLine.MatchString(s) {
		return true
	}
	if raws := findRawDates(s, time.Now(), ""); len(raws) == 1 && raws[0].start <= 1 && len(s)-raws[0].end <= 1 {
		return true
	}
	return reIDLine.MatchString(s) && strings.ContainsAny(s, "0123456789")
}
