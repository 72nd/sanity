package analysis

import (
	"sort"
	"strings"
	"unicode"
)

// Fingerprint is the letterhead fingerprint of a document: token set plus
// bigrams of the first 15 and last 5 non-empty lines. Lines containing one of
// the ignore strings (own IBAN, VAT ID, address lines) are dropped, and so are
// single-token ignore entries wherever they occur. Pure-number tokens are
// dropped as well (dates, amounts, page numbers). The result is sorted and unique.
func Fingerprint(text string, ignore []string) []string {
	lines := nonEmptyLines(text)
	var sel []string
	if len(lines) <= headerLines+footerLines {
		sel = lines
	} else {
		sel = append(append(sel, lines[:headerLines]...), lines[len(lines)-footerLines:]...)
	}
	ign := ignoreSet(ignore)
	set := map[string]struct{}{}
	for _, l := range sel {
		if lineIgnored(l, ignore) {
			continue
		}
		toks := tokenize(l)
		kept := toks[:0:0]
		for _, t := range toks {
			if isNumeric(t) {
				continue
			}
			if _, bad := ign[t]; bad {
				continue
			}
			kept = append(kept, t)
		}
		for i, t := range kept {
			set[t] = struct{}{}
			if i+1 < len(kept) {
				set[t+" "+kept[i+1]] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ignoreSet holds the single-token ignore entries.
func ignoreSet(ignore []string) map[string]struct{} {
	m := map[string]struct{}{}
	for _, ig := range ignore {
		if t := tokenize(ig); len(t) == 1 {
			m[t[0]] = struct{}{}
		}
	}
	return m
}

func lineIgnored(line string, ignore []string) bool {
	lc := alnum(line)
	for _, ig := range ignore {
		ic := alnum(ig)
		if len(ic) >= 4 && strings.Contains(lc, ic) {
			return true
		}
	}
	return false
}

// alnum lower-cases and keeps letters and digits only.
func alnum(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(normForClassify(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(normForClassify(s)), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func isNumeric(t string) bool {
	for _, r := range t {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return t != ""
}

// Jaccard is |a∩b| / |a∪b| of two sorted unique sets.
func Jaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	i, j, inter := 0, 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			inter++
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}

// FingerprintIndex keeps the fingerprints of filed documents per type.
type FingerprintIndex struct {
	byType map[string][][]string
}

// Add stores the fingerprint of a filed document.
func (x *FingerprintIndex) Add(typeID string, fp []string) {
	if x.byType == nil {
		x.byType = map[string][][]string{}
	}
	x.byType[typeID] = append(x.byType[typeID], fp)
}

// Score returns, per type, the maximum similarity over its filed documents.
func (x *FingerprintIndex) Score(fp []string) map[string]float64 {
	out := map[string]float64{}
	for id, fps := range x.byType {
		best := 0.0
		for _, f := range fps {
			if s := Jaccard(fp, f); s > best {
				best = s
			}
		}
		out[id] = best
	}
	return out
}
