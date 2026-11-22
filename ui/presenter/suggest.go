package presenter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
)

// prefillDateScore is the date score from which the date field is prefilled.
const prefillDateScore = 0.8

// TypeChoice is one entry of the type picker.
type TypeChoice struct {
	ID        string
	Name      string
	Score     float64 // -1 when the type was not suggested
	Confident bool
}

// DateChoice is one detected date candidate.
type DateChoice struct {
	Date     time.Time
	Label    string
	Fallback bool
}

// analysisState holds the results of the background analysis. Workers write
// it, the UI goroutine reads it, so it has its own lock.
type analysisState struct {
	mu       sync.Mutex
	results  map[string]inbox.Result
	inflight map[string]bool
}

// SetAnalyzer enables background analysis and suggestions.
func (p *Sort) SetAnalyzer(a *inbox.Analyzer) {
	p.analyzer = a
	p.state = &analysisState{results: map[string]inbox.Result{}, inflight: map[string]bool{}}
}

// Analyse starts the analysis of every queued document that has none yet, with
// at most two documents at a time. done is called from a worker goroutine
// after each document; the view moves to the UI goroutine (fyne.Do) there.
func (p *Sort) Analyse(ctx context.Context, done func(id string)) {
	if p.analyzer == nil {
		return
	}
	sem := make(chan struct{}, 2)
	for _, it := range p.queue.Items() {
		p.state.mu.Lock()
		_, have := p.state.results[it.ID]
		busy := p.state.inflight[it.ID]
		if !have && !busy {
			p.state.inflight[it.ID] = true
		}
		p.state.mu.Unlock()
		if have || busy {
			continue
		}
		go func(it inbox.Item) {
			sem <- struct{}{}
			defer func() { <-sem }()
			p.queue.SetStatus(it.ID, inbox.StatusAnalysing, "")
			t0 := time.Now()
			r := p.analyzer.Analyze(ctx, it.Path)
			p.recordTiming(Timing{Kind: TimingAnalysis, Name: it.Path, D: time.Since(t0)})
			p.state.mu.Lock()
			p.state.results[it.ID] = r
			delete(p.state.inflight, it.ID)
			p.state.mu.Unlock()
			if r.Err != nil {
				p.queue.SetStatus(it.ID, inbox.StatusFailed, r.Err.Error())
			} else {
				p.queue.SetStatus(it.ID, inbox.StatusReady, "")
			}
			if done != nil {
				done(it.ID)
			}
		}(it)
	}
}

func (p *Sort) result(id string) (inbox.Result, bool) {
	if p.state == nil {
		return inbox.Result{}, false
	}
	p.state.mu.Lock()
	defer p.state.mu.Unlock()
	r, ok := p.state.results[id]
	return r, ok && r.Err == nil
}

// Analysed reports whether the current document has an analysis result.
func (p *Sort) Analysed() bool {
	_, ok := p.result(p.current)
	return ok
}

// Prefill fills the empty form fields from the analysis of the current
// document: the type when the top suggestion is confident, the date when the
// best candidate is strong, the invoice number and the amount. It never
// overwrites what the user entered. Call it after Open and when the analysis
// of the current document arrives.
func (p *Sort) Prefill() {
	r, ok := p.result(p.current)
	if !ok {
		return
	}
	if p.form.TypeID == "" && len(r.Suggestions) > 0 && r.Suggestions[0].Confident {
		p.form.TypeID = r.Suggestions[0].TypeID
		p.suggestedType = p.form.TypeID
	}
	if p.form.Date.IsZero() && len(r.Dates) > 0 && !r.Dates[0].Fallback && r.Dates[0].Score >= prefillDateScore {
		p.form.Date = r.Dates[0].Date
		p.suggestedDate = p.form.Date
	}
	if p.form.Invoice == "" && len(r.Invoices) > 0 {
		p.form.Invoice = r.Invoices[0].Value
	}
	if p.form.Amount == "" && len(r.Amounts) > 0 {
		a := r.Amounts[0]
		p.form.Amount = fmt.Sprintf("%d.%02d", a.Cents/100, a.Cents%100)
		if a.Currency != "" {
			p.form.Amount += " " + a.Currency
		}
	}
}

// TypeChoices lists all types for the picker: suggested ones with their score,
// the rest unscored. The picker ranks them.
func (p *Sort) TypeChoices() []TypeChoice { return p.typeChoicesFor(p.current) }

func (p *Sort) typeChoicesFor(id string) []TypeChoice {
	scores := map[string]float64{}
	conf := map[string]bool{}
	if r, ok := p.result(id); ok {
		for _, s := range r.Suggestions {
			scores[s.TypeID] = s.Score
			conf[s.TypeID] = s.Confident
		}
	}
	out := make([]TypeChoice, 0, len(p.types))
	for _, t := range p.types {
		sc, ok := scores[t.ID]
		if !ok {
			sc = -1
		}
		out = append(out, TypeChoice{ID: t.ID, Name: t.Name, Score: sc, Confident: conf[t.ID]})
	}
	return out
}

// DateChoices lists up to three detected dates (best first); a fallback date
// is listed last and marked.
func (p *Sort) DateChoices() []DateChoice {
	r, ok := p.result(p.current)
	if !ok {
		return nil
	}
	var out []DateChoice
	for _, d := range r.Dates {
		if len(out) == 3 {
			break
		}
		out = append(out, DateChoice{Date: d.Date, Label: d.Label, Fallback: d.Fallback})
	}
	return out
}

// Duplicate reports an existing library entry that looks like the current
// document. It does not block filing.
func (p *Sort) Duplicate() (domain.LibraryEntry, string, bool) {
	r, ok := p.result(p.current)
	if !ok || r.Duplicate == nil {
		return domain.LibraryEntry{}, "", false
	}
	return *r.Duplicate, r.DupReason, true
}

// DuplicateReasonText translates the reason that analysis.FindDuplicate gives
// (an English phrase). An unknown reason is returned as it is.
func (p *Sort) DuplicateReasonText(reason string) string {
	switch reason {
	case "same file (SHA-256)":
		return p.T("sort.duplicate.reason.file")
	case "same text":
		return p.T("sort.duplicate.reason.text")
	case "same invoice number":
		return p.T("sort.duplicate.reason.invoice")
	}
	return reason
}

// overrides lists the suggestions the user changed, for later tuning.
func (p *Sort) overrides() []string {
	var o []string
	if p.suggestedType != "" && p.form.TypeID != p.suggestedType {
		o = append(o, "type")
	}
	if !p.suggestedDate.IsZero() && !p.form.Date.Equal(p.suggestedDate) {
		o = append(o, "date")
	}
	return o
}
