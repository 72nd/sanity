package presenter

import (
	"context"
	"errors"
	"image"
	"path/filepath"
	"strings"
	"time"

	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

// Form is the sort form of the current document.
type Form struct {
	TypeID  string
	Date    time.Time // zero = not set
	Invoice string
	Amount  string
	Text    map[string]string
}

// Sort is the presenter of the sort screen. It owns the queue position and the
// form, and files through the inbox.Filer. It is not safe for concurrent use:
// call it from the UI goroutine and run File/Undo results through fyne.Do.
type Sort struct {
	store store.Store
	queue *inbox.Queue
	filer *inbox.Filer
	pdf   pdf.Engine
	T     Translator

	// OnCorrection, when set, receives one line for every filing (what the
	// analysis suggested versus what was filed) and one for every undo. It
	// is called on the goroutine that files. The app writes it to the
	// correction log.
	OnCorrection func(inbox.Correction)

	types     []domain.DocumentType
	current   string
	form      Form
	last      *domain.LibraryEntry
	selection []string // multi-select in the queue, see sort_batch.go

	views         map[string]ViewState
	pageCount     map[string]int
	analyzer      *inbox.Analyzer
	state         *analysisState
	suggestedType string
	suggestedDate time.Time

	pv previewCache // rendered previews (preview.go)
	tm timings      // measured durations (preview.go)
}

// NewSort creates the presenter.
func NewSort(s store.Store, q *inbox.Queue, f *inbox.Filer, e pdf.Engine, t Translator) *Sort {
	if f.Origins == nil && q != nil {
		f.Origins = q // an undone filing gets its origin back
	}
	return &Sort{store: s, queue: q, filer: f, pdf: e, T: t}
}

// LoadTypes fetches the document types. Call it from a goroutine.
func (p *Sort) LoadTypes(ctx context.Context) error {
	ts, err := p.store.Types(ctx)
	if err != nil {
		return err
	}
	p.types = ts
	return nil
}

// Types returns the document types.
func (p *Sort) Types() []domain.DocumentType { return p.types }

// Items returns the queue.
func (p *Sort) Items() []inbox.Item { return p.queue.Items() }

// Current returns the open item.
func (p *Sort) Current() (inbox.Item, bool) { return p.queue.Get(p.current) }

// Open makes an item current and clears the form.
func (p *Sort) Open(id string) bool {
	if _, ok := p.queue.Get(id); !ok {
		return false
	}
	p.current, p.form, p.last = id, Form{Text: map[string]string{}}, nil
	p.markOpened(id)
	p.suggestedType, p.suggestedDate = "", time.Time{}
	p.Prefill()
	return true
}

// Step opens the next (delta > 0) or previous item; it wraps around. It opens
// the first item when none is current.
func (p *Sort) Step(delta int) bool {
	items := p.queue.Items()
	if len(items) == 0 {
		p.current = ""
		return false
	}
	idx := -1
	for i, it := range items {
		if it.ID == p.current {
			idx = i
		}
	}
	if idx < 0 {
		return p.Open(items[0].ID)
	}
	n := len(items)
	return p.Open(items[((idx+delta)%n+n)%n].ID)
}

// Import moves files or folders into the inbox folder (the origin is kept, see
// PutBack) and opens the first new document, so a file the user adds is shown
// at once. Files the watcher finds are added to the queue directly and do not
// take the current document away. It reports whether a new document was opened.
// Use ImportPaths to get the error of a move that failed.
func (p *Sort) Import(paths ...string) bool {
	opened, _ := p.ImportPaths(paths...)
	return opened
}

// ImportPaths is Import with the error: files that could not be moved are
// skipped, the others are imported. Call it from a goroutine; it moves files.
func (p *Sort) ImportPaths(paths ...string) (opened bool, err error) {
	ids, err := p.queue.Import(paths...)
	if len(ids) > 0 {
		opened = p.Open(ids[0])
	}
	return opened, err
}

// Form returns a copy of the form.
func (p *Sort) Form() Form { return p.form }

// SetType selects the type by id.
//
// Choosing a statement type replaces a date that is empty or still the
// automatic suggestion by the statement date of the analysis (the latest date
// of the month named most often).
func (p *Sort) SetType(id string) {
	p.form.TypeID = id
	t, ok := p.typeByID(id)
	if !ok || !t.IsStatement {
		return
	}
	r, have := p.result(p.current)
	if !have || r.StatementDate == nil {
		return
	}
	if p.form.Date.IsZero() || p.form.Date.Equal(p.suggestedDate) {
		p.form.Date = r.StatementDate.Date
		p.suggestedDate = p.form.Date
	}
}

// SetDate sets the document date.
func (p *Sort) SetDate(d time.Time) { p.form.Date = d }

// SetInvoice sets the invoice number.
func (p *Sort) SetInvoice(s string) { p.form.Invoice = strings.TrimSpace(s) }

// SetAmount sets the amount text.
func (p *Sort) SetAmount(s string) { p.form.Amount = strings.TrimSpace(s) }

// SetText sets a free-text placeholder value.
func (p *Sort) SetText(label, s string) {
	if p.form.Text == nil {
		p.form.Text = map[string]string{}
	}
	p.form.Text[label] = strings.TrimSpace(s)
}

func (p *Sort) selectedType() (domain.DocumentType, bool) {
	for _, t := range p.types {
		if t.ID == p.form.TypeID {
			return t, true
		}
	}
	return domain.DocumentType{}, false
}

func (p *Sort) values() domain.Values {
	cur, _ := p.Current()
	typ, _ := p.selectedType()
	return domain.Values{
		Date: p.form.Date, Type: typ.Name, Invoice: p.form.Invoice, Amount: p.form.Amount,
		Original: strings.TrimSuffix(cur.Name, filepath.Ext(cur.Name)), Text: p.form.Text,
	}
}

// TextFields lists the labels of the {text:<label>} placeholders of the
// selected type's template, in template order. The form shows one entry for
// each. Typed texts are kept per label, so switching the type back and forth
// does not lose them.
func (p *Sort) TextFields() []string {
	typ, ok := p.selectedType()
	if !ok {
		return nil
	}
	tpl, err := domain.ParseTemplate(typ.PathTemplate)
	if err != nil {
		return nil
	}
	return tpl.TextLabels()
}

// Missing lists the template placeholders that still need a value. It is
// empty when no type is selected.
func (p *Sort) Missing() []string {
	typ, ok := p.selectedType()
	if !ok {
		return nil
	}
	tpl, err := domain.ParseTemplate(typ.PathTemplate)
	if err != nil {
		return nil
	}
	return tpl.Missing(p.values())
}

// PathPreview evaluates the target path (relative, with /). ok is false while
// something is missing or the template is invalid.
func (p *Sort) PathPreview() (string, bool) {
	typ, ok := p.selectedType()
	if !ok || p.current == "" {
		return "", false
	}
	tpl, err := domain.ParseTemplate(typ.PathTemplate)
	if err != nil {
		return "", false
	}
	rel, err := tpl.Evaluate(p.values())
	return rel, err == nil
}

// CanFile reports whether Cmd+Enter may file now.
func (p *Sort) CanFile() bool {
	_, ok := p.PathPreview()
	return ok
}

// File files the current document and opens the next one. Call it from a
// goroutine; it hashes and moves the file. It returns the new entry.
func (p *Sort) File(ctx context.Context) (domain.LibraryEntry, error) {
	cur, ok := p.Current()
	typ, tok := p.selectedType()
	if !ok || !tok {
		return domain.LibraryEntry{}, errors.New(p.T("error.sort.incomplete"))
	}
	if !p.CanFile() {
		return domain.LibraryEntry{}, errors.New(p.T("error.sort.incomplete"))
	}
	sum, err := inbox.FileHash(cur.Path)
	if err != nil {
		return domain.LibraryEntry{}, err
	}
	res, has := p.result(cur.ID)
	e, err := p.filer.File(ctx, inbox.Request{Path: cur.Path, Type: typ, Values: p.values(), SHA256: sum, TextHash: res.TextHash, Text: res.Text, Overrides: p.overrides()})
	if err != nil {
		return domain.LibraryEntry{}, err
	}
	p.correction(inbox.NewCorrection(res, has, typ.ID, p.values(), sum, e.FiledAt, p.overrides()))
	p.queue.Remove(cur.ID)
	p.forgetPreview(cur.Path)
	p.current, p.form = "", Form{Text: map[string]string{}}
	p.Step(0)
	p.last = &e // after Step: opening an item clears it
	return e, nil
}

// LastPath is the relative path of the document filed last, for copying.
func (p *Sort) LastPath() (string, bool) {
	if p.last == nil {
		return "", false
	}
	return p.last.Path, true
}

// CanUndo reports whether a filing can be undone.
func (p *Sort) CanUndo() bool { return p.filer.CanUndo() }

// Undo reverts the last filing and puts the document back in the queue.
func (p *Sort) Undo(ctx context.Context) (domain.LibraryEntry, error) {
	e, err := p.filer.UndoLast(ctx)
	if err != nil {
		return domain.LibraryEntry{}, err
	}
	p.queue.Add(e.OriginalPath)
	p.last = nil
	p.undoneCorrection(e)
	return e, nil
}

// correction hands a log line to OnCorrection.
func (p *Sort) correction(c inbox.Correction) {
	if p.OnCorrection != nil {
		p.OnCorrection(c)
	}
}

// undoneCorrection logs that a filing was undone; the log keeps its lines.
func (p *Sort) undoneCorrection(e domain.LibraryEntry) {
	p.correction(inbox.Correction{At: p.filer.Now(), SHA256: e.SHA256, TypeID: e.TypeID, Undone: true})
}

// Preview renders a page (0-based, clamped) of a document at about 150 dpi and
// cuts the empty margin off, so the content fills the preview. It returns the
// number of pages. Call it from a goroutine; it may block.
func (p *Sort) Preview(path string, page int) (image.Image, int, error) {
	return p.PreviewAt(path, page, BaseScale)
}

// ViewState is how the preview of one document is shown: the zoom factor
// (1 = fit width) and the scroll offset in pixels. It lives only in memory, so
// every start begins with the fit-width view.
type ViewState struct {
	Zoom, X, Y float64
	Page       int // 0-based page of the document
}

// SaveView remembers the view of a document, for when the user returns to it.
func (p *Sort) SaveView(id string, v ViewState) {
	if id == "" {
		return
	}
	if p.views == nil {
		p.views = map[string]ViewState{}
	}
	p.views[id] = v
}

// View returns the remembered view of a document, or fit width at the top.
func (p *Sort) View(id string) ViewState {
	if v, ok := p.views[id]; ok {
		return v
	}
	return ViewState{Zoom: 1}
}

// CopyPath returns the path to put on the clipboard: the evaluated target path
// of the current document (relative to the office folder, or absolute), or the
// path of the document filed last when nothing is open.
func (p *Sort) CopyPath(abs bool) (string, bool) {
	rel, ok := p.PathPreview()
	if !ok {
		rel, ok = p.LastPath()
	}
	if !ok {
		return "", false
	}
	if abs {
		return filepath.Join(p.filer.Root, filepath.FromSlash(rel)), true
	}
	return domain.OfficeRel(rel), true
}

// RevealPath is the file to show in Finder: the current inbox file.
func (p *Sort) RevealPath() (string, bool) {
	cur, ok := p.Current()
	return cur.Path, ok
}

// Remove moves the current document to the trash and drops it from the queue.
// The view asks for confirmation first. Call it from a goroutine.
func (p *Sort) Remove() error {
	cur, ok := p.Current()
	if !ok {
		return errors.New(p.T("error.sort.nothing"))
	}
	if err := inbox.Trash(cur.Path); err != nil {
		return err
	}
	p.leave(cur) // the file is gone: the queue forgets its origin
	return nil
}

// CanPutBack reports whether the current document has an origin to go back to.
// A document that was already in the inbox has none; it can only be deleted.
func (p *Sort) CanPutBack() bool {
	_, ok := p.queue.Origin(p.current)
	return ok
}

// PutBack moves the current document back to its origin and drops it from the
// queue. For a document sent here by "sort again" the origin is its library
// path, so the library entry stays valid. It fails when there is no origin or
// the origin is occupied; the document then stays. Call it from a goroutine.
func (p *Sort) PutBack() error {
	cur, ok := p.Current()
	if !ok {
		return errors.New(p.T("error.sort.nothing"))
	}
	if _, err := p.queue.PutBack(cur.ID); err != nil {
		if errors.Is(err, inbox.ErrNoOrigin) {
			return errors.New(p.T("error.sort.noorigin"))
		}
		return err
	}
	p.leave(cur)
	return nil
}

// CanAdoptInPlace reports whether the current document came from inside the
// project folder, so AdoptInPlace may put it back and record it.
func (p *Sort) CanAdoptInPlace() bool {
	origin, ok := p.queue.Origin(p.current)
	return ok && p.insideProject(origin)
}

func (p *Sort) insideProject(abs string) bool {
	root, err := filepath.Abs(p.filer.Root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, abs)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// AdoptInPlace puts the current document back where it came from (no renaming)
// and records a library entry with the selected type and the date, invoice and
// amount of the form. The type and the date are required; the path template is
// not evaluated. It opens the next document and returns the entry. Adopting is
// not on the undo stack. Call it from a goroutine.
func (p *Sort) AdoptInPlace(ctx context.Context) (domain.LibraryEntry, error) {
	cur, ok := p.Current()
	if !ok {
		return domain.LibraryEntry{}, errors.New(p.T("error.sort.nothing"))
	}
	origin, has := p.queue.Origin(cur.ID)
	if !has || !p.insideProject(origin) {
		return domain.LibraryEntry{}, errors.New(p.T("error.sort.noorigin"))
	}
	typ, tok := p.selectedType()
	if !tok || p.form.Date.IsZero() {
		return domain.LibraryEntry{}, errors.New(p.T("error.sort.adoptincomplete"))
	}
	sum, err := inbox.FileHash(cur.Path)
	if err != nil {
		return domain.LibraryEntry{}, err
	}
	res, hasRes := p.result(cur.ID)
	e, err := p.filer.Adopt(ctx, inbox.AdoptRequest{Path: cur.Path, Origin: origin, Type: typ, Values: p.values(), SHA256: sum, TextHash: res.TextHash, Text: res.Text, Overrides: p.overrides()})
	if err != nil {
		return domain.LibraryEntry{}, err
	}
	p.correction(inbox.NewCorrection(res, hasRes, typ.ID, p.values(), sum, e.FiledAt, p.overrides()))
	p.leave(cur)
	p.last = &e // after leave: opening an item clears it
	return e, nil
}

// leave drops a document that left the inbox and opens the next one.
func (p *Sort) leave(cur inbox.Item) {
	p.queue.Remove(cur.ID)
	p.forgetPreview(cur.Path)
	p.current, p.form = "", Form{Text: map[string]string{}}
	p.Step(0)
}

// Count is the number of documents in the queue.
func (p *Sort) Count() int { return p.queue.Len() }

// Jump opens the first (first=true) or last document of the queue.
func (p *Sort) Jump(first bool) bool {
	items := p.queue.Items()
	if len(items) == 0 {
		return false
	}
	if first {
		return p.Open(items[0].ID)
	}
	return p.Open(items[len(items)-1].ID)
}

// SetPageCount remembers how many pages a document has (known after Preview).
func (p *Sort) SetPageCount(id string, n int) {
	if p.pageCount == nil {
		p.pageCount = map[string]int{}
	}
	p.pageCount[id] = n
}

// Page returns the shown page (0-based) and the page count of a document; the
// count is 0 until the first preview arrived.
func (p *Sort) Page(id string) (page, total int) {
	return p.View(id).Page, p.pageCount[id]
}

// PageStep moves the current document by delta pages (clamped) and reports
// whether the page changed. The scroll position returns to the top; the zoom
// stays. Save the view before calling it so the zoom is current.
func (p *Sort) PageStep(delta int) bool {
	id := p.current
	page, total := p.Page(id)
	if id == "" || total < 2 {
		return false
	}
	next := min(max(page+delta, 0), total-1)
	if next == page {
		return false
	}
	v := p.View(id)
	v.Page, v.X, v.Y = next, 0, 0
	p.SaveView(id, v)
	return true
}
