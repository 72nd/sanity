package analysis

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/72nd/sanity/domain"
)

// DateCandidate is one date found in a document.
type DateCandidate struct {
	Date     time.Time // UTC midnight
	Label    string    // the label that applies to it ("" when none)
	Score    float64   // 0..1
	Offset   int       // byte offset of the match in the text
	Raw      string    // the matched text
	Period   bool      // part of a date range such as a billing period
	Fallback bool      // not from the text; never a confident suggestion
}

const dateWindow = 40

type rawDate struct {
	start, end int
	y, m, d    int
	penalty    float64 // alternative readings of ambiguous dates
	twoDigitY  bool
}

var (
	reDMY   = regexp.MustCompile(`(\d{1,2})\.[ ]?(\d{1,2})\.[ ]?(\d{4}|\d{2})`)
	reISO   = regexp.MustCompile(`(\d{4})([-/.])(\d{2})([-/.])(\d{2})`)
	reSlash = regexp.MustCompile(`(\d{1,2})/(\d{1,2})/(\d{4}|\d{2})`)

	monthAlt = `january|januar|jan|february|februar|feb|march|märz|maerz|mär|mar|april|apr|may|mai|june|juni|jun|` +
		`july|juli|jul|august|aug|september|sept|sep|october|oktober|okt|oct|november|nov|december|dezember|dez|dec`
	reDayMonth = regexp.MustCompile(`(?i)(\d{1,2})(?:st|nd|rd|th)?\.?[ ]*(?:of[ ]+)?(` + monthAlt + `)\.?,?[ ]*(\d{4})`)
	reMonthDay = regexp.MustCompile(`(?i)(` + monthAlt + `)\.?[ ]+(\d{1,2})(?:st|nd|rd|th)?,?[ ]+(\d{4})`)
)

var monthNum = map[string]int{
	"jan": 1, "january": 1, "januar": 1, "feb": 2, "february": 2, "februar": 2,
	"mar": 3, "march": 3, "mär": 3, "märz": 3, "maerz": 3, "apr": 4, "april": 4,
	"may": 5, "mai": 5, "jun": 6, "june": 6, "juni": 6, "jul": 7, "july": 7, "juli": 7,
	"aug": 8, "august": 8, "sep": 9, "sept": 9, "september": 9,
	"oct": 10, "okt": 10, "october": 10, "oktober": 10, "nov": 11, "november": 11,
	"dec": 12, "dez": 12, "december": 12, "dezember": 12,
}

func isDigit(b byte) bool  { return b >= '0' && b <= '9' }
func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80 }

func byteAt(s string, i int) byte {
	if i < 0 || i >= len(s) {
		return 0
	}
	return s[i]
}

// numBoundaryOK checks that the match [s,e) is not part of a longer number.
func numBoundaryOK(text string, s, e int) bool {
	p, n := byteAt(text, s-1), byteAt(text, e)
	if isDigit(p) || isDigit(n) {
		return false
	}
	if (p == '.' || p == ',') && isDigit(byteAt(text, s-2)) {
		return false
	}
	if (n == '.' || n == ',' || n == '/') && isDigit(byteAt(text, e+1)) {
		return false
	}
	if p == '/' && isDigit(byteAt(text, s-2)) {
		return false
	}
	return true
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func validYMD(y, m, d int) bool {
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return false
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	return t.Year() == y && int(t.Month()) == m && t.Day() == d
}

func expandYear(s string, now time.Time) int {
	y := atoi(s)
	if len(s) == 2 {
		if 2000+y > now.Year()+1 {
			return 1900 + y
		}
		return 2000 + y
	}
	return y
}

func findRawDates(text string, now time.Time, lang string) []rawDate {
	var out []rawDate
	add := func(s, e, y, m, d int, pen float64, two bool) {
		if validYMD(y, m, d) {
			out = append(out, rawDate{start: s, end: e, y: y, m: m, d: d, penalty: pen, twoDigitY: two})
		}
	}
	for _, m := range reDMY.FindAllStringSubmatchIndex(text, -1) {
		if !numBoundaryOK(text, m[0], m[1]) {
			continue
		}
		ys := text[m[6]:m[7]]
		add(m[0], m[1], expandYear(ys, now), atoi(text[m[4]:m[5]]), atoi(text[m[2]:m[3]]), 0, len(ys) == 2)
	}
	for _, m := range reISO.FindAllStringSubmatchIndex(text, -1) {
		s, e := m[0], m[1]
		if p := byteAt(text, s-1); isDigit(p) || p == '-' || p == '/' && isDigit(byteAt(text, s-2)) {
			continue
		}
		if n := byteAt(text, e); isDigit(n) || n == '-' && isDigit(byteAt(text, e+1)) {
			continue
		}
		if text[m[4]:m[5]] != text[m[8]:m[9]] {
			continue
		}
		add(s, e, atoi(text[m[2]:m[3]]), atoi(text[m[6]:m[7]]), atoi(text[m[10]:m[11]]), 0, false)
	}
	for _, m := range reSlash.FindAllStringSubmatchIndex(text, -1) {
		if !numBoundaryOK(text, m[0], m[1]) {
			continue
		}
		a, b := atoi(text[m[2]:m[3]]), atoi(text[m[4]:m[5]])
		ys := text[m[6]:m[7]]
		y := expandYear(ys, now)
		two := len(ys) == 2
		switch {
		case a > 12:
			add(m[0], m[1], y, b, a, 0, two)
		case b > 12:
			add(m[0], m[1], y, a, b, 0, two)
		case lang == "de":
			add(m[0], m[1], y, b, a, 0, two)
			if a != b {
				add(m[0], m[1], y, a, b, 0.5, two)
			}
		default:
			add(m[0], m[1], y, a, b, 0, two)
			if a != b {
				add(m[0], m[1], y, b, a, 0.5, two)
			}
		}
	}
	for _, m := range reDayMonth.FindAllStringSubmatchIndex(text, -1) {
		s, e := m[0], m[1]
		if isDigit(byteAt(text, s-1)) || isDigit(byteAt(text, e)) {
			continue
		}
		add(s, e, atoi(text[m[6]:m[7]]), monthNum[strings.ToLower(text[m[4]:m[5]])], atoi(text[m[2]:m[3]]), 0, false)
	}
	for _, m := range reMonthDay.FindAllStringSubmatchIndex(text, -1) {
		s, e := m[0], m[1]
		if isLetter(byteAt(text, s-1)) || isDigit(byteAt(text, e)) {
			continue
		}
		add(s, e, atoi(text[m[6]:m[7]]), monthNum[strings.ToLower(text[m[2]:m[3]])], atoi(text[m[4]:m[5]]), 0, false)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out
}

type dateLabel struct {
	re     *regexp.Regexp
	weight float64
	name   string
	period bool
}

func dl(pattern string, w float64, name string, period bool) dateLabel {
	return dateLabel{regexp.MustCompile(`(?i)\b(?:` + pattern + `)`), w, name, period}
}

var dateLabels = []dateLabel{
	dl(`rechnungs-?datum|rechnungsdatum|belegdatum|invoice[ ]+date|date[ ]+of[ ]+(?:issue|invoice)|issue[ ]+date|`+
		`datum[ ]+der[ ]+rechnung|ausstellungsdatum|ausgestellt[ ]+am|kontoauszug[ ]+(?:vom|per)|statement[ ]+date|`+
		`rechnung[ ]+vom|belegs?datum|invoice[ ]+of|bill(?:ing)?[ ]+date|receipt[ ]+date|date[ ]+issued|belegdatum|beleg[ ]+vom`, 0.5, "invoice date", false),
	dl(`datum|date|dated|vom`, 0.12, "date", false),
	dl(`f(?:ä|ae)llig(?:keits?(?:datum)?)?(?:[ ]+am)?|zahlbar[ ]+(?:bis|innert|innerhalb)(?:[ ]+zum)?|zahlungsziel|zahlungsfrist|`+
		`due[ ]+date|due[ ]+on|payment[ ]+due|pay[ ]+by|due[ ]+by|`+
		`lieferdatum|liefertermin|delivery[ ]+date|leistungsdatum|service[ ]+date|`+
		`druckdatum|print(?:ed)?[ ]+date|gedruckt[ ]+am|g(?:ü|ue)ltig[ ]+bis|valid[ ]+(?:until|through|to)|expires?(?:[ ]+on)?|geburtsdatum|date[ ]+of[ ]+birth`, -0.5, "due/delivery date", false),
	dl(`leistungszeitraum|abrechnungszeitraum|abrechnungsperiode|rechnungszeitraum|billing[ ]+period|service[ ]+period|period|zeitraum`, -0.5, "period", true),
}

func lineStart(text string, i int) int {
	return strings.LastIndexByte(text[:i], '\n') + 1
}

// labelNear returns the closest label ending before position start within the
// window, or ("", 0, false).
func labelNear(window string) (dateLabel, string, bool) {
	best := dateLabel{}
	bestText := ""
	bestEnd, bestLen := -1, 0
	for _, l := range dateLabels {
		for _, m := range l.re.FindAllStringIndex(window, -1) {
			ln := m[1] - m[0]
			if m[1] > bestEnd || m[1] == bestEnd && ln > bestLen {
				best, bestText, bestEnd, bestLen = l, window[m[0]:m[1]], m[1], ln
			}
		}
	}
	return best, bestText, bestEnd >= 0
}

func cutWindow(text string, start int) string {
	lo := start - dateWindow
	if lo < 0 {
		lo = 0
	}
	for lo < start && !isRuneStart(text[lo]) {
		lo++
	}
	w := text[lo:start]
	// same or previous line only
	if i := strings.LastIndexByte(w, '\n'); i >= 0 {
		if j := strings.LastIndexByte(w[:i], '\n'); j >= 0 {
			w = w[j+1:]
		}
	}
	return w
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

var periodSep = regexp.MustCompile(`^[ ]*(?:-|–|—|bis|to|until|through|/)[ ]*$`)

// FindDates finds dates in text, sorted by descending score. lang ("de" or
// "en") decides how ambiguous slash dates are read; cfg tunes labels and the
// plausibility window.
func FindDates(text string, now time.Time, lang string, cfg domain.DateConfig) []DateCandidate {
	raws := findRawDates(text, now, lang)
	if len(raws) == 0 {
		return nil
	}
	// period detection: two dates joined by a dash or "bis"
	period := make([]bool, len(raws))
	for i := 0; i+1 < len(raws); i++ {
		a, b := raws[i], raws[i+1]
		if a.end <= b.start && periodSep.MatchString(text[a.end:b.start]) {
			period[i], period[i+1] = true, true
		}
	}
	// dates per line, for header-over-value tables
	lines := map[int]*lineInfo{}
	for i, r := range raws {
		ls := lineStart(text, r.start)
		if lines[ls] == nil {
			lines[ls] = &lineInfo{}
		}
		lines[ls].idx = append(lines[ls].idx, i)
	}

	total := float64(len(text))
	counts := map[string]int{}
	for i, r := range raws {
		if !period[i] && r.penalty == 0 {
			counts[key(r)]++
		}
	}
	maxAge := cfg.MaxAgeYears
	if maxAge <= 0 {
		maxAge = 15
	}
	var all []DateCandidate
	for i, r := range raws {
		label, weight, isPeriod := labelFor(text, raws, i, lines, cfg)
		isPeriod = isPeriod || period[i]
		score := 0.3 + weight
		if !r.twoDigitY {
			score += 0.05
		} else {
			score -= 0.05
		}
		if total > 0 {
			switch pos := float64(r.start) / total; {
			case pos < 0.3:
				score += 0.1
			case pos > 0.7:
				score -= 0.03
			}
		}
		if n := counts[key(r)]; n > 1 && !isPeriod {
			bonus := 0.05 * float64(n-1)
			if bonus > 0.15 {
				bonus = 0.15
			}
			score += bonus
		}
		d := time.Date(r.y, time.Month(r.m), r.d, 0, 0, 0, 0, time.UTC)
		if d.After(now.AddDate(0, 0, 1)) {
			score -= 0.5
		}
		if d.Before(now.AddDate(-maxAge, 0, 0)) {
			score -= 0.5
		}
		score -= r.penalty
		if isPeriod && score > 0.2 {
			score = 0.2
		}
		all = append(all, DateCandidate{Date: d, Label: label, Score: dClamp(score), Offset: r.start,
			Raw: text[r.start:r.end], Period: isPeriod})
	}
	// keep the best occurrence per date
	best := map[time.Time]int{}
	var out []DateCandidate
	for _, c := range all {
		if j, ok := best[c.Date]; ok {
			if c.Score > out[j].Score {
				out[j] = c
			}
			continue
		}
		best[c.Date] = len(out)
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Offset < out[j].Offset
	})
	return out
}

func key(r rawDate) string { return strconv.Itoa(r.y*10000 + r.m*100 + r.d) }

func dClamp(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

type lineInfo struct{ idx []int }

// labelFor finds the label for raws[i]: first in the window before the date,
// then, for header-over-value tables, in the previous line. It returns the
// label text, the score weight and whether the label marks a period.
func labelFor(text string, raws []rawDate, i int, lines map[int]*lineInfo, cfg domain.DateConfig) (string, float64, bool) {
	r := raws[i]
	win := cutWindow(text, r.start)
	if l, name, ok := labelNear(win); ok {
		return name, l.weight + configWeight(win, cfg), l.period
	}
	if w := configWeight(win, cfg); w != 0 {
		return "", w, false
	}
	// table: "Rechnungsdatum   Fälligkeit\n12.03.2025   26.03.2025"
	ls := lineStart(text, r.start)
	li := lines[ls]
	if li == nil || ls == 0 {
		return "", 0, false
	}
	prevStart := lineStart(text, ls-1)
	prev := text[prevStart : ls-1]
	var found []struct {
		l    dateLabel
		name string
		pos  int
	}
	for _, l := range dateLabels {
		for _, m := range l.re.FindAllStringIndex(prev, -1) {
			found = append(found, struct {
				l    dateLabel
				name string
				pos  int
			}{l, prev[m[0]:m[1]], m[0]})
		}
	}
	sort.Slice(found, func(a, b int) bool { return found[a].pos < found[b].pos })
	// drop generic "date" labels swallowed by a longer label at the same place
	var uniq []struct {
		l    dateLabel
		name string
		pos  int
	}
	for _, f := range found {
		if n := len(uniq); n > 0 && f.pos < uniq[n-1].pos+len(uniq[n-1].name) {
			continue
		}
		uniq = append(uniq, f)
	}
	k := -1
	for n, idx := range li.idx {
		if idx == i {
			k = n
		}
	}
	if k >= 0 && len(uniq) == len(li.idx) {
		f := uniq[k]
		return f.name, f.l.weight + configWeight(f.name, cfg), f.l.period
	}
	return "", 0, false
}

// configWeight applies the type's preferred and ignored labels.
func configWeight(window string, cfg domain.DateConfig) float64 {
	lw := strings.ToLower(window)
	w := 0.0
	for _, p := range cfg.PreferredLabels {
		if p != "" && strings.Contains(lw, strings.ToLower(p)) {
			w += 0.4
			break
		}
	}
	for _, p := range cfg.IgnoreLabels {
		if p != "" && strings.Contains(lw, strings.ToLower(p)) {
			w -= 0.6
			break
		}
	}
	return w
}

var (
	reFileISO = regexp.MustCompile(`(\d{4})[-_.](\d{2})[-_.](\d{2})`)
	reFile8   = regexp.MustCompile(`(?:^|\D)(\d{4})(\d{2})(\d{2})(?:\D|$)`)
	reFileDMY = regexp.MustCompile(`(\d{2})[-_.](\d{2})[-_.](\d{4})`)
)

// FallbackDate picks a date that is not from the document text: the PDF
// creation date, then a date in the file name, then the file mtime. The result
// is always marked Fallback and scores low, so it is never a confident
// suggestion. A zero Date means nothing was available.
func FallbackDate(pdfCreated time.Time, fileName string, mtime time.Time) DateCandidate {
	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }
	if !pdfCreated.IsZero() && pdfCreated.Year() > 1990 {
		return DateCandidate{Date: day(pdfCreated), Label: "pdf created", Score: 0.3, Offset: -1, Fallback: true}
	}
	if d, raw, ok := dateInFileName(fileName); ok {
		return DateCandidate{Date: d, Label: "file name", Score: 0.25, Offset: -1, Raw: raw, Fallback: true}
	}
	if !mtime.IsZero() {
		return DateCandidate{Date: day(mtime), Label: "modified", Score: 0.1, Offset: -1, Fallback: true}
	}
	return DateCandidate{Fallback: true}
}

func dateInFileName(name string) (time.Time, string, bool) {
	try := func(re *regexp.Regexp, order [3]int) (time.Time, string, bool) {
		for _, m := range re.FindAllStringSubmatch(name, -1) {
			y, mo, d := atoi(m[order[0]]), atoi(m[order[1]]), atoi(m[order[2]])
			if y >= 1990 && y <= 2100 && validYMD(y, mo, d) {
				return time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC), strings.TrimSpace(m[0]), true
			}
		}
		return time.Time{}, "", false
	}
	if d, r, ok := try(reFileISO, [3]int{1, 2, 3}); ok {
		return d, r, ok
	}
	if d, r, ok := try(reFile8, [3]int{1, 2, 3}); ok {
		return d, r, ok
	}
	return try(reFileDMY, [3]int{3, 2, 1})
}
