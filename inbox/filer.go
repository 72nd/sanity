package inbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/store"
)

// Request describes one document to file.
type Request struct {
	Path      string // current absolute path of the PDF
	Type      domain.DocumentType
	Values    domain.Values
	SHA256    string
	TextHash  string
	Overrides []string
	Text      string // normalized analysis text, optional; passed to OnFiled
	// Batch groups filings made in one go (see NewBatch); UndoBatch undoes
	// them together. Empty for a single filing.
	Batch string
}

type filed struct {
	move  Move
	entry domain.LibraryEntry
	batch string
}

// Filer files documents (always a move) and undoes the last filings.
type Filer struct {
	Root  string
	Store store.Store
	Now   func() time.Time
	NewID func() string

	// OnFiled runs after a successful filing (learning hook); OnUndone after
	// an undo. Both are optional.
	OnFiled  func(entry domain.LibraryEntry, text string)
	OnUndone func(entry domain.LibraryEntry)

	mu   sync.Mutex
	undo []filed
	seq  int

	batchSeq int
}

// NewFiler creates a filer for the project root.
func NewFiler(root string, s store.Store) *Filer {
	return &Filer{Root: root, Store: s, Now: time.Now}
}

// File evaluates the type's path template, moves the PDF there (adding _2, _3
// on collisions) and records the library entry. A missing template value is an
// error; the sort form requires those fields first.
func (f *Filer) File(ctx context.Context, r Request) (domain.LibraryEntry, error) {
	tpl, err := domain.ParseTemplate(r.Type.PathTemplate)
	if err != nil {
		return domain.LibraryEntry{}, err
	}
	rel, err := tpl.Evaluate(r.Values)
	if err != nil {
		return domain.LibraryEntry{}, err
	}
	rel = domain.UniquePath(rel, func(p string) bool {
		_, err := os.Lstat(filepath.Join(f.Root, filepath.FromSlash(p)))
		return err == nil
	})
	mv, err := MoveFile(r.Path, filepath.Join(f.Root, filepath.FromSlash(rel)))
	if err != nil {
		return domain.LibraryEntry{}, err
	}
	now := f.Now()
	e := domain.LibraryEntry{
		ID: f.nextID(now), TypeID: r.Type.ID, Path: rel, OriginalPath: r.Path,
		Date: r.Values.Date, Invoice: r.Values.Invoice, Amount: r.Values.Amount,
		SHA256: r.SHA256, TextHash: r.TextHash, FiledAt: now, Overrides: r.Overrides,
	}
	if err := f.Store.AddEntry(ctx, e); err != nil {
		if uerr := Undo(mv); uerr != nil {
			err = errors.Join(err, uerr)
		}
		return domain.LibraryEntry{}, err
	}
	f.mu.Lock()
	f.undo = append(f.undo, filed{mv, e, r.Batch})
	f.mu.Unlock()
	if f.OnFiled != nil {
		f.OnFiled(e, r.Text)
	}
	return e, nil
}

// CanUndo reports whether a filing can be undone.
func (f *Filer) CanUndo() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.undo) > 0
}

// UndoLast moves the last filed document back to its original path and removes
// its library entry. It returns the entry that was undone.
func (f *Filer) UndoLast(ctx context.Context) (domain.LibraryEntry, error) {
	f.mu.Lock()
	if len(f.undo) == 0 {
		f.mu.Unlock()
		return domain.LibraryEntry{}, errors.New("inbox: nothing to undo")
	}
	last := f.undo[len(f.undo)-1]
	f.mu.Unlock()
	if err := Undo(last.move); err != nil {
		return domain.LibraryEntry{}, err
	}
	f.mu.Lock()
	f.undo = f.undo[:len(f.undo)-1]
	f.mu.Unlock()
	if err := f.Store.RemoveEntry(ctx, last.entry.ID); err != nil {
		return domain.LibraryEntry{}, err
	}
	if f.OnUndone != nil {
		f.OnUndone(last.entry)
	}
	return last.entry, nil
}

// NewBatch returns a fresh batch id for Request.Batch.
func (f *Filer) NewBatch() string {
	f.mu.Lock()
	f.batchSeq++
	n := f.batchSeq
	f.mu.Unlock()
	return "batch-" + itoa(n)
}

// LastBatchSize is the number of filings at the end of the undo stack that
// belong to the same batch as the last one; 0 when the last filing was not
// part of a batch.
func (f *Filer) LastBatchSize() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := len(f.undo)
	if n == 0 || f.undo[n-1].batch == "" {
		return 0
	}
	id, c := f.undo[n-1].batch, 0
	for i := n - 1; i >= 0 && f.undo[i].batch == id; i-- {
		c++
	}
	return c
}

// UndoBatch undoes every filing of the last batch in one step (newest first).
// When the last filing is not part of a batch it undoes just that one. It
// stops at the first error and returns the entries undone so far.
func (f *Filer) UndoBatch(ctx context.Context) ([]domain.LibraryEntry, error) {
	n := f.LastBatchSize()
	if n == 0 {
		n = 1
	}
	var out []domain.LibraryEntry
	for i := 0; i < n; i++ {
		e, err := f.UndoLast(ctx)
		if err != nil {
			return out, err
		}
		out = append(out, e)
	}
	return out, nil
}

func (f *Filer) nextID(now time.Time) string {
	if f.NewID != nil {
		return f.NewID()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	return now.Format("20060102T150405") + "-" + itoa(f.seq)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
