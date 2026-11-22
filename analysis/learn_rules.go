package analysis

import (
	"regexp"
	"strings"

	"github.com/72nd/sanity/domain"
)

var (
	learnIBAN   = regexp.MustCompile(`\b[A-Z]{2}\d{2}(?:\s?[A-Z0-9]{4}){2,7}(?:\s?[A-Z0-9]{1,4})?\b`)
	learnVAT    = regexp.MustCompile(`\bDE\d{9}\b|\bATU\d{8}\b|\bCHE-\d{3}\.\d{3}\.\d{3}\b`)
	learnCustNo = regexp.MustCompile(`(?i)(?:kunden-?(?:nummer|nr\.?)|customer\s?(?:no\.?|number|id)|vertrags-?(?:nummer|nr\.?)|account\s?(?:no\.?|number))\s*[:.]?\s*([A-Z0-9][A-Z0-9\-/.]{3,20})`)
	learnDomain = regexp.MustCompile(`(?i)\b(?:[a-z0-9][a-z0-9-]*\.)+(?:com|de|ch|at|net|org|io|eu|co|app|dev|ai|cloud|info|shop|bank|us|uk)\b`)
)

// learnFreeMail domains say nothing about the sender.
var learnFreeMail = map[string]bool{
	"gmail.com": true, "googlemail.com": true, "gmx.de": true, "gmx.net": true, "web.de": true,
	"outlook.com": true, "hotmail.com": true, "icloud.com": true, "yahoo.com": true, "t-online.de": true,
}

// LearnRules suggests distinctive identifiers found in a document for the
// "Learn rules from this document" action. IBAN, VAT ID and domain rules are
// enabled; the customer number rule is suggested but disabled. Identifiers
// matching the project's ignore list are skipped.
func LearnRules(text string, ignore []string) []domain.Rule {
	text = normForClassify(text)
	ign := make([]string, 0, len(ignore))
	for _, s := range ignore {
		if c := compact(s); c != "" {
			ign = append(ign, c)
		}
	}
	ignored := func(v string) bool {
		c := compact(v)
		for _, i := range ign {
			if c == i || (len(i) >= 6 && strings.Contains(i, c)) {
				return true
			}
		}
		return false
	}
	seen := map[string]bool{}
	var out []domain.Rule
	add := func(kind, value string, weight float64, enabled bool) {
		key := kind + "|" + strings.ToLower(value)
		if seen[key] || ignored(value) {
			return
		}
		seen[key] = true
		out = append(out, domain.Rule{Kind: kind, Value: value, Mode: modeShould, Weight: weight, Where: "any", Enabled: enabled})
	}
	for _, m := range learnIBAN.FindAllString(text, -1) {
		if v := compact(m); learnValidIBAN(v) {
			add("iban", v, 0.9, true)
		}
	}
	for _, m := range learnVAT.FindAllString(text, -1) {
		add("vatId", m, 0.9, true)
	}
	for _, m := range learnDomain.FindAllString(text, -1) {
		d := strings.ToLower(strings.TrimPrefix(strings.ToLower(m), "www."))
		if learnFreeMail[d] {
			continue
		}
		add("domain", d, 0.5, true)
	}
	for _, m := range learnCustNo.FindAllStringSubmatch(text, -1) {
		v := strings.TrimRight(m[1], ".-/")
		if strings.ContainsAny(v, "0123456789") {
			add("customerNo", v, 0.9, false)
		}
	}
	return out
}

// learnValidIBAN checks length and the mod-97 checksum of a compact IBAN.
func learnValidIBAN(s string) bool {
	if len(s) < 15 || len(s) > 34 {
		return false
	}
	r := s[4:] + s[:4]
	rem := 0
	for _, c := range r {
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
