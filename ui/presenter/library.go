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

	// amount caches AmountValue(Entry.Amount) for sorting and filtering;
	// parsed tells whether it was filled (rows built outside Load are not).
	amount            float64
	hasAmount, parsed bool
}

// Amount is the numeric amount of the row (see AmountValue), cached by Load.
func (r Row) Amount() (float64, bool) {
	if r.parsed {
		return r.amount, r.hasAmount
	}
	return AmountValue(r.Entry.Amount)
}

// Library is the presenter of the library page.
type Library struct {
	store  store.Store
	T      Translator
	rows   []Row
	loaded bool
	at     time.Time
	root   string // project root, for absolute paths

	// Query and SelKeys keep the quick filter text and the selected entry ids
	// across visits (the page is rebuilt on every show).
	Query   string
	SelKeys []string
	// ImportFn takes the absolute paths of documents that go back to the sort
	// queue (Resort). The view sets it; nil means nothing happens.
	ImportFn func(paths ...string)
	// OnRemoved is called for every library entry that was removed, so the
	// learned index can forget the document.
	OnRemoved func(e domain.LibraryEntry)
	// InboxCount returns the number of documents waiting in the inbox.
	InboxCount func() int
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
		r := Row{Entry: e, TypeName: e.TypeID, parsed: true}
		r.amount, r.hasAmount = AmountValue(e.Amount)
		if t, ok := pr.TypeByID(e.TypeID); ok {
			r.TypeName = t.Name
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Entry.Date.After(rows[j].Entry.Date) })
	p.rows, p.loaded, p.at, p.root = rows, true, time.Now(), pr.Root
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
		if matches(r, q) {
			out = append(out, r)
		}
	}
	return out
}

// Focus prepares the page to show one filed entry: the quick filter becomes
// the entry's path, so only it is visible, and the entry is selected. It works
// from the loaded rows; an entry that is not loaded yet is selected without
// the filter (the page reloads on the next visit). Show the page afterwards.
func (p *Library) Focus(entryID string) {
	for _, r := range p.rows {
		if r.Entry.ID == entryID {
			p.FocusEntry(r.Entry)
			return
		}
	}
	p.Query, p.SelKeys = "", []string{entryID}
}

// FocusEntry is Focus for an entry the caller already has (it may have been
// filed after the last load).
func (p *Library) FocusEntry(e domain.LibraryEntry) {
	p.Query, p.SelKeys = e.Path, []string{e.ID}
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
