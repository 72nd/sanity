package analysis

import (
	"slices"
	"sort"
	"strings"

	"github.com/72nd/sanity/domain"
)

const (
	defaultThreshold = 0.8
	minLead          = 0.15
	minFiledForFull  = 3
	learnedCap       = 0.7

	// statementSuppress multiplies the score of a type without
	// DocumentType.IsStatement when the document contains a statement keyword.
	// A statement lists payments to many companies, so the name of one of them
	// must not decide the type; 0.2 keeps even a 0.9 rule below every threshold.
	statementSuppress = 0.2
	// statementBoost is combined (noisy OR) with the score of a statement type
	// that already has some evidence when the document contains a keyword. A
	// type without any evidence is not suggested because of it.
	statementBoost = 0.4
)

// HasStatementKeyword reports whether the text contains one of the keywords,
// case-insensitively and on the normalized text.
func HasStatementKeyword(text string, keywords []string) bool {
	plain := strings.ToLower(normForClassify(text))
	for _, k := range keywords {
		k = strings.ToLower(strings.TrimSpace(normForClassify(k)))
		if k != "" && strings.Contains(plain, k) {
			return true
		}
	}
	return false
}

// LearnedIndex provides the learned score per type.
type LearnedIndex interface {
	// Score returns the learned score per type id for a document text.
	Score(text string) map[string]float64
	// Filed returns how many documents of the type are indexed.
	Filed(typeID string) int
}

// Index is the standard LearnedIndex: letterhead fingerprints plus TF-IDF kNN.
// The learned score of a type is the larger of both.
type Index struct {
	Ignore []string // project's own identifiers
	fp     FingerprintIndex
	corpus Corpus
	count  map[string]int
}

// NewIndex returns an empty index.
func NewIndex(ignore []string) *Index { return &Index{Ignore: ignore, count: map[string]int{}} }

// Add indexes the text of one filed document.
func (x *Index) Add(typeID, text string) {
	x.fp.Add(typeID, Fingerprint(text, x.Ignore))
	x.corpus.Add(typeID, text)
	x.count[typeID]++
}

// Filed implements LearnedIndex.
func (x *Index) Filed(typeID string) int { return x.count[typeID] }

// Score implements LearnedIndex.
func (x *Index) Score(text string) map[string]float64 {
	out := x.fp.Score(Fingerprint(text, x.Ignore))
	for id, s := range x.corpus.Score(text) {
		if s > out[id] {
			out[id] = s
		}
	}
	return out
}

// Suggestion is one candidate type for a document.
type Suggestion struct {
	TypeID       string
	Score        float64
	RuleScore    float64
	LearnedScore float64
	Confident    bool
}

// Classify ranks the types for a document, best first. Types excluded by a
// not rule or with score 0 are left out. learned may be nil.
func Classify(doc Doc, types []domain.DocumentType, learned LearnedIndex) []Suggestion {
	sc := newScopes(doc)
	var lscores map[string]float64
	if learned != nil {
		lscores = learned.Score(doc.Text)
	}
	// The statement adjustment applies only when a type carries the flag, so a
	// project without statement types classifies as before.
	statement := slices.ContainsFunc(types, func(t domain.DocumentType) bool { return t.IsStatement }) &&
		HasStatementKeyword(doc.Text, doc.StatementKeywords)
	var out []Suggestion
	for _, t := range types {
		rr := evalScopes(t.Rules, sc)
		if rr.Excluded {
			continue
		}
		var ls float64
		if learned != nil && t.UseLearned {
			ls = clamp01(lscores[t.ID])
			if learned.Filed(t.ID) < minFiledForFull && ls > learnedCap {
				ls = learnedCap
			}
		}
		s := noisyOR(rr.Score, ls)
		switch {
		case statement && !t.IsStatement:
			s *= statementSuppress
		case statement && s > 0:
			s = noisyOR(s, statementBoost)
		}
		if s <= 0 {
			continue
		}
		out = append(out, Suggestion{TypeID: t.ID, Score: s, RuleScore: rr.Score, LearnedScore: ls})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	thr := map[string]float64{}
	for _, t := range types {
		thr[t.ID] = t.Threshold
	}
	for i := range out {
		th := thr[out[i].TypeID]
		if th == 0 {
			th = defaultThreshold
		}
		runner := 0.0
		if i == 0 && len(out) > 1 {
			runner = out[1].Score
		} else if i > 0 {
			runner = out[0].Score
		}
		out[i].Confident = out[i].Score >= th && out[i].Score-runner >= minLead-1e-9
	}
	return out
}
