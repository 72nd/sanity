package presenter

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
)

// BatchItem is the plan for one selected document: the evaluated target path,
// or why the document is blocked.
type BatchItem struct {
	ID      string
	Name    string
	Path    string   // relative target path (with /); empty when blocked
	Missing []string // template placeholders without a value
	Err     string   // other reason (no type, invalid template); empty when none
}

// Blocked reports whether the document cannot be filed yet.
func (b BatchItem) Blocked() bool { return b.Path == "" }

// BatchFailure is a document that FileBatch could not file.
type BatchFailure struct {
	ID   string
	Name string
	Err  error
}

// SetSelection remembers the selected queue documents (multi-select). Ids that
// are not in the queue are dropped.
func (p *Sort) SetSelection(ids []string) {
	p.selection = p.selection[:0:0]
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	for _, it := range p.queue.Items() { // queue order keeps plans stable
		if want[it.ID] {
			p.selection = append(p.selection, it.ID)
		}
	}
}

// Selection returns the selected document ids in queue order, without ids that
// left the queue.
func (p *Sort) Selection() []string {
	var out []string
	for _, id := range p.selection {
		if _, ok := p.queue.Get(id); ok {
			out = append(out, id)
		}
	}
	return out
}

// BatchActive reports whether a batch (more than one document) is selected.
func (p *Sort) BatchActive() bool { return len(p.Selection()) > 1 }

// batchValues are the template values of one document from its own analysis
// (the form is not used): the same prefill rules as Prefill.
func (p *Sort) batchValues(it inbox.Item, typ domain.DocumentType) domain.Values {
	v := domain.Values{Type: typ.Name, Original: strings.TrimSuffix(it.Name, filepath.Ext(it.Name))}
	r, ok := p.result(it.ID)
	if !ok {
		return v
	}
	if len(r.Dates) > 0 && !r.Dates[0].Fallback && r.Dates[0].Score >= prefillDateScore {
		v.Date = r.Dates[0].Date
	}
	if len(r.Invoices) > 0 {
		v.Invoice = r.Invoices[0].Value
	}
	if len(r.Amounts) > 0 {
		a := r.Amounts[0]
		v.Amount = fmt.Sprintf("%d.%02d", a.Cents/100, a.Cents%100)
		if a.Currency != "" {
			v.Amount += " " + a.Currency
		}
	}
	return v
}

func (p *Sort) typeByID(id string) (domain.DocumentType, bool) {
	for _, t := range p.types {
		if t.ID == id {
			return t, true
		}
	}
	return domain.DocumentType{}, false
}

// BatchPlan evaluates the target path of every selected document for the type.
// Paths of documents in the same batch that would collide are not made unique
// here; the filer adds _2, _3 when it files them.
func (p *Sort) BatchPlan(typeID string) []BatchItem {
	typ, tok := p.typeByID(typeID)
	var tpl domain.Template
	var terr error
	if tok {
		tpl, terr = domain.ParseTemplate(typ.PathTemplate)
	}
	var out []BatchItem
	for _, id := range p.Selection() {
		it, _ := p.queue.Get(id)
		b := BatchItem{ID: id, Name: it.Name}
		switch {
		case !tok:
			b.Err = p.T("error.sort.incomplete")
		case terr != nil:
			b.Err = terr.Error()
		default:
			v := p.batchValues(it, typ)
			if m := tpl.Missing(v); len(m) > 0 {
				b.Missing = m
			} else if rel, err := tpl.Evaluate(v); err != nil {
				b.Err = err.Error()
			} else {
				b.Path = rel
			}
		}
		out = append(out, b)
	}
	return out
}

// CanBatch reports whether a batch is selected and every document of it has
// all template values for the type.
func (p *Sort) CanBatch(typeID string) bool {
	plan := p.BatchPlan(typeID)
	if len(plan) < 2 {
		return false
	}
	for _, b := range plan {
		if b.Blocked() {
			return false
		}
	}
	return true
}

// BatchTypeChoices lists the types for the batch picker, ranked by the
// suggestions of the first selected document.
func (p *Sort) BatchTypeChoices() []TypeChoice {
	sel := p.Selection()
	if len(sel) == 0 {
		return p.typeChoicesFor("")
	}
	return p.typeChoicesFor(sel[0])
}

// FileBatch files every ready selected document with the type through the
// filer, one by one. It continues after a failure and reports each; blocked
// documents are reported as failures too and stay in the queue. Every filing
// is on the undo stack (Undo undoes them one by one, UndoBatch together) and
// carries one batch id. Filed documents leave the queue and the next remaining
// document opens. Call it from a goroutine.
func (p *Sort) FileBatch(ctx context.Context, typeID string) (filed []domain.LibraryEntry, failed []BatchFailure) {
	typ, ok := p.typeByID(typeID)
	if !ok {
		for _, id := range p.Selection() {
			it, _ := p.queue.Get(id)
			failed = append(failed, BatchFailure{ID: id, Name: it.Name, Err: errors.New(p.T("error.sort.incomplete"))})
		}
		return nil, failed
	}
	batch := p.filer.NewBatch()
	var left []string
	for _, b := range p.BatchPlan(typeID) {
		if b.Blocked() {
			reason := b.Err
			if len(b.Missing) > 0 {
				reason = p.T("sort.hint.missing", "Fields", strings.Join(b.Missing, ", "))
			}
			failed = append(failed, BatchFailure{ID: b.ID, Name: b.Name, Err: errors.New(reason)})
			left = append(left, b.ID)
			continue
		}
		it, _ := p.queue.Get(b.ID)
		sum, err := inbox.FileHash(it.Path)
		if err != nil {
			failed = append(failed, BatchFailure{ID: b.ID, Name: b.Name, Err: err})
			left = append(left, b.ID)
			continue
		}
		res, _ := p.result(it.ID)
		var over []string
		if len(res.Suggestions) > 0 && res.Suggestions[0].Confident && res.Suggestions[0].TypeID != typeID {
			over = []string{"type"}
		}
		vals := p.batchValues(it, typ)
		e, err := p.filer.File(ctx, inbox.Request{
			Path: it.Path, Type: typ, Values: vals, SHA256: sum,
			TextHash: res.TextHash, Text: res.Text, Overrides: over, Batch: batch,
		})
		if err != nil {
			failed = append(failed, BatchFailure{ID: b.ID, Name: b.Name, Err: err})
			left = append(left, b.ID)
			continue
		}
		_, has := p.result(it.ID)
		p.correction(inbox.NewCorrection(res, has, typ.ID, vals, sum, e.FiledAt, over))
		filed = append(filed, e)
		p.queue.Remove(b.ID)
	}
	p.selection = left
	if _, ok := p.queue.Get(p.current); !ok {
		p.current, p.form = "", Form{Text: map[string]string{}}
		p.Step(0)
	}
	if n := len(filed); n > 0 {
		last := filed[n-1]
		p.last = &last // after Step: opening an item clears it
	}
	return filed, failed
}

// LastBatchSize is the number of filings the next UndoBatch reverts (0 when
// the last filing was a single one).
func (p *Sort) LastBatchSize() int { return p.filer.LastBatchSize() }

// UndoBatch reverts the last batch in one step (or the last single filing) and
// puts the documents back in the queue.
func (p *Sort) UndoBatch(ctx context.Context) ([]domain.LibraryEntry, error) {
	es, err := p.filer.UndoBatch(ctx)
	for _, e := range es {
		p.queue.Add(e.OriginalPath)
		p.undoneCorrection(e)
	}
	if len(es) > 0 {
		p.last = nil
	}
	return es, err
}
