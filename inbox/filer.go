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
}

type filed struct {
	move  Move
	entry domain.LibraryEntry
}

// Filer files documents (always a move) and undoes the last filings.
type Filer struct {
	Root  string
	Store store.Store
	Now   func() time.Time
	NewID func() string

	mu   sync.Mutex
	undo []filed
	seq  int
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
	f.undo = append(f.undo, filed{mv, e})
	f.mu.Unlock()
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
	return last.entry, nil
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
