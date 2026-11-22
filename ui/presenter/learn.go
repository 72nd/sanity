package presenter

import (
	"context"
	"errors"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/domain"
)

// RuleSuggestion is a rule the user may add to the selected type, learned from
// the current document.
type RuleSuggestion struct {
	Rule    domain.Rule
	Label   string // localized kind and value
	Checked bool   // ticked by default
}

// SuggestRules proposes distinctive identifiers of the current document (IBAN,
// VAT ID, domain, customer number) as rules of the selected type. Rules the
// type already has are left out, and so are the project's own identifiers.
// Call it from a goroutine.
func (p *Sort) SuggestRules(ctx context.Context) ([]RuleSuggestion, error) {
	typ, ok := p.selectedType()
	if !ok {
		return nil, errors.New(p.T("error.learn.type"))
	}
	res, ok := p.result(p.current)
	if !ok {
		return nil, errors.New(p.T("error.learn.analysis"))
	}
	pr, err := p.store.Project(ctx)
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, r := range typ.Rules {
		have[r.Kind+"\x00"+r.Value] = true
	}
	var out []RuleSuggestion
	for _, r := range analysis.LearnRules(res.Text, pr.IgnoreTokens) {
		if have[r.Kind+"\x00"+r.Value] {
			continue
		}
		out = append(out, RuleSuggestion{
			Rule:    r,
			Label:   p.T("sort.learn.kind."+r.Kind) + ": " + r.Value,
			Checked: r.Enabled,
		})
	}
	return out, nil
}

// LearnRules appends the chosen rules to the selected type and saves it. Call
// it from a goroutine.
func (p *Sort) LearnRules(ctx context.Context, rules []domain.Rule) error {
	typ, ok := p.selectedType()
	if !ok {
		return errors.New(p.T("error.learn.type"))
	}
	if len(rules) == 0 {
		return nil
	}
	for _, r := range rules {
		r.Enabled = true
		typ.Rules = append(typ.Rules, r)
	}
	if err := p.store.SaveType(ctx, typ); err != nil {
		return err
	}
	return p.LoadTypes(ctx)
}
