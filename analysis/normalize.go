// Package analysis turns document text into suggestions (dates, fields, types).
// It is pure Go and imports no UI.
package analysis

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var (
	hyphenBreak  = regexp.MustCompile(`(\p{L})[-\x{00AD}\x{2010}] *\n *(\p{Ll})`)
	softHyphen   = strings.NewReplacer("\u00AD", "")
	blanks       = regexp.MustCompile(`[ \t]+`)
	manyNewlines = regexp.MustCompile(`\n{3,}`)
	numericToken = regexp.MustCompile(`[0-9][0-9OolI.,/'’:\-]*[0-9]`)
)

// Normalize cleans extracted text for analysis: odd spaces become blanks,
// line-end hyphenation is joined, OCR letter/digit confusions inside numeric
// tokens are repaired (O/o to 0, l/I to 1) and blanks are collapsed.
func Normalize(text string) string {
	r := strings.NewReplacer(
		"\r\n", "\n", "\r", "\n", "\f", "\n", "\v", "\n",
		"\u00A0", " ", "\u2007", " ", "\u202F", " ", "\u2009", " ", "\u2002", " ",
		"\u2003", " ", "\u2004", " ", "\u2005", " ", "\u2006", " ", "\u200A", " ",
		"\u3000", " ", "\u200B", "", "\u200C", "", "\u200D", "", "\uFEFF", "",
		"\u2212", "-", "\u2011", "-",
	)
	text = r.Replace(text)
	text = hyphenBreak.ReplaceAllString(text, "$1$2")
	text = softHyphen.Replace(text)
	text = numericToken.ReplaceAllStringFunc(text, repairNumeric)
	text = blanks.ReplaceAllString(text, " ")
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	text = strings.Join(lines, "\n")
	text = manyNewlines.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

// repairNumeric fixes a token that is mostly digits and separators. A token is
// only touched when it has at least two real digits and at least one of the
// confusable letters, so words such as "Olli" or "Ill" stay untouched.
func repairNumeric(tok string) string {
	digits, letters := 0, 0
	for _, c := range tok {
		switch {
		case c >= '0' && c <= '9':
			digits++
		case c == 'O' || c == 'o' || c == 'l' || c == 'I':
			letters++
		}
	}
	if digits < 2 || letters == 0 || letters > digits {
		return tok
	}
	return strings.NewReplacer("O", "0", "o", "0", "l", "1", "I", "1").Replace(tok)
}

// TextHash returns a hex hash of the normalised text that ignores case and
// whitespace differences, so re-OCR noise in spacing does not change it.
func TextHash(norm string) string {
	s := strings.ToLower(strings.Join(strings.Fields(norm), " "))
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
