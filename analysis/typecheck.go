package analysis

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/72nd/sanity/domain"
)

// The values the rule and date fields accept. The LLM prompt (package prompts)
// is generated from these slices, so they cannot drift from the code.
var (
	RuleKinds   = []string{"keyword", "regex", "iban", "vatId", "customerNo", "domain", "filename"}
	RuleModes   = []string{modeShould, modeMust, modeNot}
	RuleWheres  = []string{"any", "header", "footer", "filename"}
	DateSources = []string{"auto", "text", "filename", "metadata"}
)

// Weight words of the snippet format and the numbers they stand for.
const (
	WeightWeak   = 0.2
	WeightNormal = 0.5
	WeightStrong = 0.9
)

// libraryLimit is the score from which a filed document of another type counts
// as a possible false positive.
const libraryLimit = 0.5

// redactionMarks are lower-case fragments that show a redacted value. A rule
// value that contains one was never a real identifier.
var redactionMarks = []string{"redacted", "xxxx", "***", "▇", "█", "[removed]", "[hidden]", "<removed>", "<hidden>"}

// Codes of Finding (the Code field of CheckType's findings). They are stable; the UI maps them to localized text.
//
//	Code                  Level    RuleIdx  Argument        Meaning
//	id.empty              error    -1                       id is empty
//	id.collision          error    -1       suggested id    id exists in Existing (replace or take the suggestion)
//	id.ok                 ok       -1                       id is set and unique
//	name.empty            error    -1                       name is empty
//	template.invalid      error    -1       template        does not parse or fails Validate
//	template.unsafe       error    -1       template        absolute, escapes the root or has unsafe characters
//	template.ok           ok       -1                       path template is valid
//	threshold.range       error    -1       threshold       threshold outside 0..1
//	date.source           error    -1       source          unknown date.source
//	rules.none            error    -1                       no enabled rule
//	rules.weak            warning  -1                       all enabled rules have weight <= 0.2
//	rule.kind             error    rule     kind            unknown kind
//	rule.mode             error    rule     mode            unknown mode
//	rule.where            error    rule     where           unknown where
//	rule.weight           error    rule     weight          weight outside 0..1
//	rule.empty            error    rule                     empty value
//	rule.regex            error    rule     regex           regex does not compile
//	rule.redacted         error    rule     value           value is a redaction placeholder
//	rule.ignoreToken      warning  rule     token           value contains one of the project's IgnoreTokens
//	enums.ok              ok       -1                       all kind, mode, where and source values known
//	weights.ok            ok       -1                       all weights in 0..1
//	regex.ok              ok       -1                       all regexes compile
//	redaction.ok          ok       -1                       no redaction placeholder in a rule
//	rules.ok              ok       -1                       at least one enabled rule
//	sample.weak           warning  -1       sample name     the type is not confident on the sample
//	sample.others         warning  -1       sample name     another type scores as high or higher on the sample
//	library.falsePositive warning  -1       count           filed documents of other types score > 0.5
//	library.skipped       warning  -1       count           filed documents without cached text were not checked
//
// CheckType validates a new or imported document type and dry-runs it against
// the samples and the library. It does not change the classifier or the store.
func CheckType(in CheckInput) Report {
	var r Report
	t := in.Type
	add := func(l Level, code, msg string, idx int, arg string) {
		r.Findings = append(r.Findings, Finding{Level: l, Code: code, Message: msg, RuleIdx: idx, Argument: arg})
	}
	group := func(before int, code, msg string) {
		if !hasErrorSince(r.Findings, before) {
			add(LevelOK, code, msg, -1, "")
		}
	}

	// id, name
	n := len(r.Findings)
	switch {
	case strings.TrimSpace(t.ID) == "":
		add(LevelError, "id.empty", "id is empty", -1, "")
	case idTaken(in.Existing, t.ID):
		sug := uniqueID(in.Existing, t.ID)
		add(LevelError, "id.collision", fmt.Sprintf("a type with id %q exists; replace it or use %q", t.ID, sug), -1, sug)
	}
	group(n, "id.ok", "id is set and unique")
	if strings.TrimSpace(t.Name) == "" {
		add(LevelError, "name.empty", "name is empty", -1, "")
	}

	// path template
	n = len(r.Findings)
	if tpl, err := domain.ParseTemplate(t.PathTemplate); err != nil {
		add(LevelError, "template.invalid", err.Error(), -1, t.PathTemplate)
	} else if err := tpl.Validate(); err != nil {
		code := "template.invalid"
		if errors.Is(err, domain.ErrUnsafePath) {
			code = "template.unsafe"
		}
		add(LevelError, code, err.Error(), -1, t.PathTemplate)
	}
	group(n, "template.ok", "path template is valid")

	if t.Threshold < 0 || t.Threshold > 1 {
		add(LevelError, "threshold.range", "threshold must be between 0 and 1", -1, fmt.Sprint(t.Threshold))
	}

	// enums and weights
	n = len(r.Findings)
	if t.Date.Source != "" && !slices.Contains(DateSources, t.Date.Source) {
		add(LevelError, "date.source", "unknown date source (known: "+strings.Join(DateSources, ", ")+")", -1, t.Date.Source)
	}
	for i, ru := range t.Rules {
		if !slices.Contains(RuleKinds, ru.Kind) {
			add(LevelError, "rule.kind", "unknown kind (known: "+strings.Join(RuleKinds, ", ")+")", i, ru.Kind)
		}
		if !slices.Contains(RuleModes, ru.Mode) {
			add(LevelError, "rule.mode", "unknown mode (known: "+strings.Join(RuleModes, ", ")+")", i, ru.Mode)
		}
		if !slices.Contains(RuleWheres, ru.Where) {
			add(LevelError, "rule.where", "unknown where (known: "+strings.Join(RuleWheres, ", ")+")", i, ru.Where)
		}
	}
	group(n, "enums.ok", "kinds, modes, wheres and date source are known")
	n = len(r.Findings)
	for i, ru := range t.Rules {
		if ru.Weight < 0 || ru.Weight > 1 {
			add(LevelError, "rule.weight", "weight must be between 0 and 1", i, fmt.Sprint(ru.Weight))
		}
	}
	group(n, "weights.ok", "weights are within 0 and 1")

	// values
	n = len(r.Findings)
	for i, ru := range t.Rules {
		if strings.TrimSpace(ru.Value) == "" {
			add(LevelError, "rule.empty", "rule value is empty", i, "")
			continue
		}
		if ru.Kind == "regex" {
			if _, err := regexp.Compile("(?i)" + ru.Value); err != nil {
				add(LevelError, "rule.regex", err.Error(), i, ru.Value)
			}
		}
	}
	group(n, "regex.ok", "all regular expressions compile")
	n = len(r.Findings)
	for i, ru := range t.Rules {
		low := strings.ToLower(ru.Value)
		for _, m := range redactionMarks {
			if strings.Contains(low, m) {
				add(LevelError, "rule.redacted", "the value looks like a redaction placeholder, not a real identifier", i, ru.Value)
				break
			}
		}
	}
	group(n, "redaction.ok", "no redaction placeholder in a rule value")
	for i, ru := range t.Rules {
		if tok, ok := containsIgnore(ru.Value, in.IgnoreTokens); ok {
			add(LevelWarning, "rule.ignoreToken", "the value contains one of the project's own identifiers; the learning ignores it, the rule would not", i, tok)
		}
	}

	// rules as a whole
	enabled, strong := 0, false
	for _, ru := range t.Rules {
		if !ru.Enabled {
			continue
		}
		enabled++
		if ru.Mode != modeNot && ru.Weight > WeightWeak {
			strong = true
		}
	}
	switch {
	case enabled == 0:
		add(LevelError, "rules.none", "the type needs at least one enabled rule", -1, "")
	case !strong:
		add(LevelWarning, "rules.weak", "all enabled rules have a weight of 0.2 or less (generic words); add a stronger identifier", -1, "")
	default:
		add(LevelOK, "rules.ok", "has enabled rules", -1, "")
	}

	r.dryRun(in)
	return r
}

func (r *Report) dryRun(in CheckInput) {
	t := in.Type
	types := make([]domain.DocumentType, 0, len(in.Existing)+1)
	for _, e := range in.Existing {
		if e.ID != t.ID {
			types = append(types, e)
		}
	}
	types = append(types, t)

	for _, s := range in.Samples {
		res := SampleResult{Name: s.Name}
		sugg := Classify(Doc{Text: s.Text, FileName: s.FileName}, types, in.Learned)
		best := 0.0
		for _, g := range sugg {
			if g.TypeID == t.ID {
				res.Score, res.Confident = g.Score, g.Confident
				continue
			}
			res.Others = append(res.Others, OtherScore{TypeID: g.TypeID, Score: g.Score})
			best = max(best, g.Score)
		}
		r.Samples = append(r.Samples, res)
		if !res.Confident {
			r.Findings = append(r.Findings, Finding{Level: LevelWarning, Code: "sample.weak", RuleIdx: -1, Argument: s.Name,
				Message: fmt.Sprintf("the type is not confident on sample %q (score %.2f)", s.Name, res.Score)})
		}
		if best > 0 && best >= res.Score {
			r.Findings = append(r.Findings, Finding{Level: LevelWarning, Code: "sample.others", RuleIdx: -1, Argument: s.Name,
				Message: fmt.Sprintf("another type scores %.2f on sample %q, not less than the new type", best, s.Name)})
		}
	}

	if in.Text == nil {
		return
	}
	for _, e := range in.Library {
		if e.TypeID == t.ID {
			continue
		}
		text, ok := in.Text(e.SHA256)
		if !ok || strings.TrimSpace(text) == "" {
			r.Skipped++
			continue
		}
		rr := EvalRules(t.Rules, Doc{Text: text, FileName: path.Base(e.Path)})
		if !rr.Excluded && rr.Score > libraryLimit {
			r.Library = append(r.Library, LibraryHit{EntryID: e.ID, Path: e.Path, TypeID: e.TypeID, Score: rr.Score})
		}
	}
	if len(r.Library) > 0 {
		r.Findings = append(r.Findings, Finding{Level: LevelWarning, Code: "library.falsePositive", RuleIdx: -1, Argument: fmt.Sprint(len(r.Library)),
			Message: fmt.Sprintf("%d filed documents of other types score above %.1f for the new type", len(r.Library), libraryLimit)})
	}
	if r.Skipped > 0 {
		r.Findings = append(r.Findings, Finding{Level: LevelWarning, Code: "library.skipped", RuleIdx: -1, Argument: fmt.Sprint(r.Skipped),
			Message: fmt.Sprintf("%d filed documents have no cached text and were not checked", r.Skipped)})
	}
}

func hasErrorSince(f []Finding, from int) bool {
	for _, x := range f[from:] {
		if x.Level == LevelError {
			return true
		}
	}
	return false
}

func idTaken(types []domain.DocumentType, id string) bool {
	return slices.ContainsFunc(types, func(t domain.DocumentType) bool { return t.ID == id })
}

func uniqueID(types []domain.DocumentType, id string) string {
	for n := 2; ; n++ {
		c := fmt.Sprintf("%s-%d", id, n)
		if !idTaken(types, c) {
			return c
		}
	}
}

// containsIgnore reports the first ignore token that value contains, compared
// case-insensitively and, for identifiers, without whitespace.
func containsIgnore(value string, tokens []string) (string, bool) {
	low, comp := strings.ToLower(value), compact(value)
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if strings.Contains(low, strings.ToLower(tok)) || strings.Contains(comp, compact(tok)) {
			return tok, true
		}
	}
	return "", false
}
