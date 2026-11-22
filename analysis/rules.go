// Package analysis recognises what a document is: type, date and fields.
// Pure Go, standard library only, no UI.
package analysis

import (
	"regexp"
	"strings"

	"github.com/72nd/sanity/domain"
)

// Rule modes and scopes as stored in domain.Rule.
const (
	modeShould = "should"
	modeMust   = "must"
	modeNot    = "not"

	headerLines = 15
	footerLines = 5
)

// Doc is the input of the classifier.
type Doc struct {
	Text     string
	FileName string   // base name, may be empty
	Header   []string // optional; derived from Text (first 15 non-empty lines) when nil
	Footer   []string // optional; derived from Text (last 5 non-empty lines) when nil
}

// scopes holds the lower-cased texts a rule can look at.
type scopes struct {
	any, header, footer, filename string
	// compact variants: upper-case without whitespace, for identifiers.
	anyC, headerC, footerC, filenameC string
}

func newScopes(d Doc) scopes {
	header, footer := d.Header, d.Footer
	lines := nonEmptyLines(d.Text)
	if header == nil {
		header = firstN(lines, headerLines)
	}
	if footer == nil {
		footer = lastN(lines, footerLines)
	}
	mk := func(s string) (string, string) {
		s = normForClassify(s)
		return strings.ToLower(s), compact(s)
	}
	var s scopes
	s.any, s.anyC = mk(d.Text)
	s.header, s.headerC = mk(strings.Join(header, "\n"))
	s.footer, s.footerC = mk(strings.Join(footer, "\n"))
	s.filename, s.filenameC = mk(d.FileName)
	return s
}

func (s scopes) pick(where string) (plain, comp string) {
	switch where {
	case "header":
		return s.header, s.headerC
	case "footer":
		return s.footer, s.footerC
	case "filename":
		return s.filename, s.filenameC
	}
	return s.any, s.anyC
}

// ruleMatches reports whether one rule matches the document scopes.
func ruleMatches(r domain.Rule, s scopes) bool {
	v := strings.TrimSpace(r.Value)
	if v == "" {
		return false
	}
	where := r.Where
	if r.Kind == "filename" {
		where = "filename"
	}
	plain, comp := s.pick(where)
	switch r.Kind {
	case "keyword", "filename", "customerNo", "domain":
		return strings.Contains(plain, strings.ToLower(normForClassify(v)))
	case "iban", "vatId":
		return strings.Contains(comp, compact(v))
	case "regex":
		re, err := regexp.Compile("(?i)" + v)
		if err != nil {
			return false
		}
		if where == "filename" {
			return re.MatchString(s.filename)
		}
		return re.MatchString(rawScope(s, where))
	}
	return false
}

// rawScope is the plain text for regex rules (lower-cased, which is fine with (?i)).
func rawScope(s scopes, where string) string {
	p, _ := s.pick(where)
	return p
}

// RuleResult is the outcome of evaluating one type's rules.
type RuleResult struct {
	Score    float64
	Excluded bool // a matching not rule
}

// EvalRules evaluates rules against a document.
// Score is 1 - Π(1-w) over matching should (and must) rules. All enabled must
// rules have to match, otherwise the score is 0. A matching not rule excludes
// the type. A must rule that matches also contributes its weight.
func EvalRules(rules []domain.Rule, d Doc) RuleResult {
	return evalScopes(rules, newScopes(d))
}

func evalScopes(rules []domain.Rule, s scopes) RuleResult {
	miss := 1.0
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		m := ruleMatches(r, s)
		switch r.Mode {
		case modeNot:
			if m {
				return RuleResult{Excluded: true}
			}
		case modeMust:
			if !m {
				return RuleResult{}
			}
			miss *= 1 - clamp01(r.Weight)
		default: // should
			if m {
				miss *= 1 - clamp01(r.Weight)
			}
		}
	}
	return RuleResult{Score: 1 - miss}
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// noisyOR combines independent evidence.
func noisyOR(a, b float64) float64 { return 1 - (1-clamp01(a))*(1-clamp01(b)) }

func nonEmptyLines(text string) []string {
	var out []string
	for _, l := range strings.Split(normForClassify(text), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func firstN(l []string, n int) []string {
	if len(l) > n {
		return l[:n]
	}
	return l
}

func lastN(l []string, n int) []string {
	if len(l) > n {
		return l[len(l)-n:]
	}
	return l
}

// compact upper-cases and removes all whitespace and NBSP-like spaces.
func compact(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(normForClassify(s)) {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// normForClassify fixes typographic spaces and soft hyphens; it keeps line breaks.
func normForClassify(s string) string {
	return classifyReplacer.Replace(s)
}

var classifyReplacer = strings.NewReplacer(
	" ", " ", " ", " ", " ", " ", "​", "", "­", "", "\r\n", "\n", "\r", "\n",
)
