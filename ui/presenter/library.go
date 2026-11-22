// Package presenter holds the view logic of sanity. It imports no UI package,
// so everything here is tested headless.
package presenter

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/store"
)

// Translator resolves a message id (with key/value template data). Pass
// i18n.T in production and a stub in tests.
type Translator func(id string, kv ...any) string

// Row is one library line as the grid shows it.
type Row struct {
	Entry    domain.LibraryEntry
	TypeName string
}

// Library is the presenter of the library page.
type Library struct {
	store  store.Store
	T      Translator
	rows   []Row
	loaded bool
	at     time.Time
}

// NewLibrary creates the presenter.
func NewLibrary(s store.Store, t Translator) *Library { return &Library{store: s, T: t} }

// Loaded reports whether data has been fetched.
func (p *Library) Loaded() bool { return p.loaded }

// Load fetches the library. Call it from a goroutine; it may block. The page
// calls it on every visit, so documents filed meanwhile show up.
func (p *Library) Load(ctx context.Context) error {
	pr, err := p.store.Project(ctx)
	if err != nil {
		return err
	}
	rows := make([]Row, 0, len(pr.Library))
	for _, e := range pr.Library {
		r := Row{Entry: e, TypeName: e.TypeID}
		if t, ok := pr.TypeByID(e.TypeID); ok {
			r.TypeName = t.Name
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Entry.Date.After(rows[j].Entry.Date) })
	p.rows, p.loaded, p.at = rows, true, time.Now()
	return nil
}

// freshFor is how long loaded data is shown without asking the store again.
const freshFor = 2 * time.Second

// Fresh reports whether the data was loaded a moment ago, so a page that is
// shown again right away needs no reload.
func (p *Library) Fresh() bool { return p.loaded && time.Since(p.at) < freshFor }

// Total is the number of filed documents.
func (p *Library) Total() int { return len(p.rows) }

// Visible returns the rows whose path, type, invoice or amount contain query.
func (p *Library) Visible(query string) []Row {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return p.rows
	}
	var out []Row
	for _, r := range p.rows {
		hay := strings.ToLower(r.Entry.Path + " " + r.TypeName + " " + r.Entry.Invoice + " " + r.Entry.Amount)
		if strings.Contains(hay, q) {
			out = append(out, r)
		}
	}
	return out
}

// DateText formats a document date for the grid.
func (p *Library) DateText(t time.Time) string { return t.Format("02.01.2006") }

// FiledText formats the filing time for the grid.
func (p *Library) FiledText(t time.Time) string { return t.Format("02.01.2006 15:04") }

// StatusItems builds the right side of the status bar.
func (p *Library) StatusItems(shown, selected int) []string {
	items := []string{p.T("status.rows", "Shown", strconv.Itoa(shown), "Total", strconv.Itoa(p.Total()))}
	if selected > 1 {
		items = append(items, p.T("status.selected", "Count", strconv.Itoa(selected)))
	}
	return items
}
