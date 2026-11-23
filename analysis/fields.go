package analysis

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/72nd/sanity/domain"
)

// Found is a string value found in text (invoice number, IBAN, VAT ID).
type Found struct {
	Value  string
	Raw    string
	Score  float64
	Offset int
}

// Amount is a money value found in text.
type Amount struct {
	Cents    int64
	Currency string // EUR, CHF, USD, GBP or "" when unknown
	Raw      string
	Score    float64
	Offset   int
}

var reInvoice = regexp.MustCompile(`(?i)\b(?:rechnung(?:s)?[ -]*(?:nummer|nr\.?|no\.?)|invoice[ ]*(?:no\.?|number|nr\.?|#)|inv\.?[ ]*(?:no\.?|#)|` +
	`beleg-?(?:nr\.?|nummer)|re-?nr\.?|receipt[ ]*(?:no\.?|number|#)|facture[ ]*n°|rechnung[ ]*#)[ ]*[:.#]?[ ]*([A-Z0-9][A-Z0-9\-/.]{3,24})`)

// FindInvoice returns invoice number candidates, best first. override is an
// optional per-type regex; its first group (or the whole match) is tried first.
func FindInvoice(text string, override string) []Found {
	var out []Found
	seen := map[string]bool{}
	add := func(val, raw string, off int, score float64) {
		val = strings.TrimRight(val, ".-/")
		if len(val) < 3 || seen[val] {
			return
		}
		seen[val] = true
		out = append(out, Found{Value: val, Raw: raw, Score: score, Offset: off})
	}
	if override != "" {
		if re, err := regexp.Compile(override); err == nil {
			for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
				s, e := m[0], m[1]
				if len(m) >= 4 && m[2] >= 0 {
					s, e = m[2], m[3]
				}
				add(text[s:e], text[m[0]:m[1]], s, 0.95)
			}
		}
	}
	for _, m := range reInvoice.FindAllStringSubmatchIndex(text, -1) {
		val := text[m[2]:m[3]]
		if !strings.ContainsAny(val, "0123456789") {
			continue
		}
		score := 0.8
		if strings.Contains(text[m[0]:m[2]], "\n") {
			score -= 0.1
		}
		if len(strings.Trim(val, ".-/")) < 5 {
			score -= 0.1
		}
		add(val, text[m[0]:m[1]], m[2], score)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

var (
	// Grouping marks: . , ' ’ and the narrow spaces some statements use
	// (NBSP, thin space, narrow NBSP). A plain space is no grouping mark: in
	// table text "3 100,00" is a quantity next to a price.
	reMoney     = regexp.MustCompile(`-?\d{1,3}(?:[.,'’\x{a0}\x{2009}\x{202f}]\d{3})*[.,]\d{2}|-?\d+[.,]\d{2}`)
	reCurAfter  = regexp.MustCompile(`(?i)^[ ]{0,2}(€|EUR\b|CHF\b|USD\b|\$|GBP\b|£|Fr\.|SFr\.)`)
	reCurBefore = regexp.MustCompile(`(?i)(€|\bEUR|\bCHF|\bUSD|\$|\bGBP|£|\bSFr\.|\bFr\.)[ ]{0,2}$`)
)

var amountLabels = []struct {
	re     *regexp.Regexp
	weight float64
}{
	{regexp.MustCompile(`(?i)\b(?:gesamt(?:betrag|summe)?|total(?:[ ]+(?:due|amount|price|chf|eur|usd))?|grand[ ]+total|summe|rechnungsbetrag|zu[ ]+zahlen|zahlbetrag|` +
		`endbetrag|endsumme|amount[ ]+(?:due|payable|paid)|balance[ ]+due|total[ ]+due|betrag[ ]+(?:brutto|inkl)|brutto|zahlungsbetrag|` +
		`gesamtpreis|to[ ]+pay|payable|charged|abgebucht|lastschrift)`), 0.45},
	{regexp.MustCompile(`(?i)\b(?:betrag|amount|preis|price)\b`), 0.15},
	{regexp.MustCompile(`(?i)\b(?:zwischensumme|sub-?total|netto|net|mwst|ust|vat|tax|steuer|mehrwertsteuer|rabatt|discount|versand|shipping|porto|skonto)`), -0.3},
}

// preferredAmountWeight is added to a value whose label window contains one
// of the type's preferred labels. It outweighs the generic "total" labels
// (0.45) and the largest-value bonus (0.1) together, so a type that says
// "Betrag" gets the amount next to "Betrag" even when a larger one is around.
const preferredAmountWeight = 0.6

// FindAmount returns money candidates, best first. The best one is usually the
// grand total: it carries a total label and tends to be the largest value.
func FindAmount(text string) []Amount { return FindAmountFor(text, domain.AmountConfig{}, "") }

// FindAmountFor is FindAmount with the type's preferred labels: a value whose
// label window (the same and the previous line before it) contains a preferred
// label, case-insensitively, scores preferredAmountWeight higher. currency
// (an ISO code, may be empty) is the type's default for values that carry none;
// it applies after the currency symbols of the document and before a code
// named somewhere in the text.
func FindAmountFor(text string, cfg domain.AmountConfig, currency string) []Amount {
	var all []Amount
	for _, m := range reMoney.FindAllStringIndex(text, -1) {
		s, e := m[0], m[1]
		if !numBoundaryOK(text, s, e) {
			continue
		}
		raw := text[s:e]
		cents, ok := parseMoney(raw)
		if !ok {
			continue
		}
		rest := text[e:]
		if strings.HasPrefix(strings.TrimLeft(rest, " "), "%") {
			continue
		}
		cur := ""
		score := 0.2
		if c := reCurAfter.FindStringSubmatch(rest); c != nil {
			cur = curCode(c[1])
		} else if c := reCurBefore.FindStringSubmatch(text[:s]); c != nil {
			cur = curCode(c[1])
		}
		if cur != "" {
			score += 0.15
		}
		win := text[maxInt(0, s-dateWindow):s]
		if i := strings.LastIndexByte(win, '\n'); i >= 0 {
			if j := strings.LastIndexByte(win[:i], '\n'); j >= 0 {
				win = win[j+1:]
			}
		}
		bestEnd, w := -1, 0.0
		for _, l := range amountLabels {
			for _, lm := range l.re.FindAllStringIndex(win, -1) {
				if lm[1] > bestEnd {
					bestEnd, w = lm[1], l.weight
				}
			}
		}
		score += w
		if labelMatches(labelWindow(text, s), cfg.PreferredLabels) {
			score += preferredAmountWeight
		}
		all = append(all, Amount{Cents: cents, Currency: cur, Raw: strings.TrimSpace(text[s:e]), Score: score, Offset: s})
	}
	if len(all) == 0 {
		return nil
	}
	// document currency for amounts without one
	freq := map[string]int{}
	for _, a := range all {
		if a.Currency != "" {
			freq[a.Currency]++
		}
	}
	docCur, n := "", 0
	for c, k := range freq {
		if k > n || k == n && c < docCur {
			docCur, n = c, k
		}
	}
	if docCur == "" {
		docCur = strings.ToUpper(strings.TrimSpace(currency))
	}
	if docCur == "" { // no symbol next to a value: a code anywhere in the text
		docCur = textCurrency(text)
	}
	var maxCents int64
	for _, a := range all {
		if a.Cents > maxCents {
			maxCents = a.Cents
		}
	}
	best := map[string]int{}
	var out []Amount
	for _, a := range all {
		if a.Currency == "" {
			a.Currency = docCur
		}
		if a.Cents == maxCents {
			a.Score += 0.1
		}
		if a.Cents == 0 {
			a.Score -= 0.5 // "Amount Due 0.00" of a paid invoice is not the amount
		}
		k := a.Currency + strconv.FormatInt(a.Cents, 10)
		if j, ok := best[k]; ok {
			if a.Score > out[j].Score {
				a.Score = a.Score + 0.03
				out[j] = a
			} else {
				out[j].Score += 0.03
			}
			continue
		}
		best[k] = len(out)
		out = append(out, a)
	}
	for i := range out {
		out[i].Score = dClamp(out[i].Score)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Cents > out[j].Cents
	})
	return out
}

// labelWindow is the text a preferred label may stand in for the value at s:
// the line before it, and the previous line too when that is a heading, that
// is, holds no money value of its own (then it belongs to another row).
func labelWindow(text string, s int) string {
	ls := lineStart(text, s)
	win := text[ls:s]
	if ls == 0 {
		return win
	}
	ps := lineStart(text, ls-1)
	if prev := text[ps : ls-1]; !reMoney.MatchString(prev) {
		win = prev + "\n" + win
	}
	return win
}

// labelMatches reports whether window contains one of the labels,
// case-insensitively; empty labels are ignored.
func labelMatches(window string, labels []string) bool {
	lw := strings.ToLower(window)
	for _, l := range labels {
		if l = strings.TrimSpace(l); l != "" && strings.Contains(lw, strings.ToLower(l)) {
			return true
		}
	}
	return false
}

var reCurCode = regexp.MustCompile(`\b(EUR|CHF|USD|GBP)\b`)

// textCurrency returns the currency code named most often in text, or "".
func textCurrency(text string) string {
	freq := map[string]int{}
	for _, m := range reCurCode.FindAllString(text, -1) {
		freq[m]++
	}
	best, n := "", 0
	for c, k := range freq {
		if k > n || k == n && c < best {
			best, n = c, k
		}
	}
	return best
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func curCode(s string) string {
	switch strings.ToUpper(s) {
	case "€", "EUR":
		return "EUR"
	case "$", "USD":
		return "USD"
	case "CHF", "FR.", "SFR.":
		return "CHF"
	case "£", "GBP":
		return "GBP"
	}
	return ""
}

// parseMoney reads 1.234,56 / 1'234.56 / 1,234.56 / 12,50 / 12.50 as cents.
// The last separator is the decimal mark; the others are grouping marks.
func parseMoney(raw string) (int64, bool) {
	neg := strings.HasPrefix(raw, "-")
	raw = strings.TrimPrefix(raw, "-")
	i := strings.LastIndexAny(raw, ".,")
	if i < 0 || len(raw)-i-1 != 2 {
		return 0, false
	}
	intPart := raw[:i]
	if len(intPart) > 1 && intPart[0] == '0' && isDigit(intPart[1]) {
		return 0, false
	}
	for _, g := range []string{".", ",", "'", "’", " ", "\u00a0", "\u2009", "\u202f"} {
		intPart = strings.ReplaceAll(intPart, g, "")
	}
	v, err := strconv.ParseInt(intPart+raw[i+1:], 10, 64)
	if err != nil {
		return 0, false
	}
	if neg {
		v = -v
	}
	return v, true
}

var ibanLen = map[string]int{
	"DE": 22, "CH": 21, "LI": 21, "AT": 20, "FR": 27, "GB": 22, "IT": 27, "ES": 24, "NL": 18, "BE": 16,
	"LU": 20, "PT": 25, "IE": 22, "DK": 18, "SE": 24, "NO": 15, "FI": 18, "PL": 28, "CZ": 24, "HU": 28,
	"GR": 27, "SK": 24, "SI": 19, "HR": 21, "LT": 20, "LV": 21, "EE": 20, "MT": 31, "CY": 28, "BG": 22, "RO": 24,
}

var reIBANFind = regexp.MustCompile(`[A-Z]{2}\d{2}(?:[ ]?[A-Z0-9]{4}){2,7}(?:[ ]?[A-Z0-9]{1,4})?`)

// IBANValid checks length (known countries) and the mod 97 checksum.
func IBANValid(iban string) bool {
	iban = strings.ToUpper(strings.ReplaceAll(iban, " ", ""))
	if len(iban) < 15 || len(iban) > 34 {
		return false
	}
	if n, ok := ibanLen[iban[:2]]; ok && n != len(iban) {
		return false
	}
	rem := 0
	for _, c := range iban[4:] + iban[:4] {
		switch {
		case c >= '0' && c <= '9':
			rem = (rem*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			rem = (rem*100 + int(c-'A') + 10) % 97
		default:
			return false
		}
	}
	return rem == 1
}

// FindIBANs returns the valid IBANs in text (compact form in Value).
func FindIBANs(text string) []Found {
	var out []Found
	seen := map[string]bool{}
	for _, m := range reIBANFind.FindAllStringIndex(text, -1) {
		if isAlnum(byteAt(text, m[0]-1)) {
			continue
		}
		raw := text[m[0]:m[1]]
		// positions of non-space bytes, to cut back from the end
		compact := make([]int, 0, len(raw))
		for i := 0; i < len(raw); i++ {
			if raw[i] != ' ' {
				compact = append(compact, i)
			}
		}
		for n := len(compact); n >= 15; n-- {
			cand := raw[:compact[n-1]+1]
			if IBANValid(cand) {
				v := strings.ReplaceAll(cand, " ", "")
				if !seen[v] {
					seen[v] = true
					out = append(out, Found{Value: v, Raw: cand, Score: 1, Offset: m[0]})
				}
				break
			}
		}
	}
	return out
}

func isAlnum(b byte) bool { return isDigit(b) || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }

var reVATFind = regexp.MustCompile(`DE\d{9}|ATU\d{8}|CHE[- ]?\d{3}[. ]?\d{3}[. ]?\d{3}(?:[ ]?(?:MWST|TVA|IVA))?|NL\d{9}B\d{2}|FR[A-Z0-9]{2}\d{9}|IT\d{11}|GB\d{9}|ESX\d{7}[A-Z]|BE0\d{9}`)

// FindVATIDs returns VAT IDs. CHE numbers are normalised to CHE-123.456.789.
func FindVATIDs(text string) []Found {
	var out []Found
	seen := map[string]bool{}
	for _, m := range reVATFind.FindAllStringIndex(text, -1) {
		if isAlnum(byteAt(text, m[0]-1)) || isDigit(byteAt(text, m[1])) {
			continue
		}
		raw := text[m[0]:m[1]]
		v := raw
		if strings.HasPrefix(raw, "CHE") {
			d := strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, raw)
			v = "CHE-" + d[0:3] + "." + d[3:6] + "." + d[6:9]
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, Found{Value: v, Raw: raw, Score: 1, Offset: m[0]})
	}
	return out
}
