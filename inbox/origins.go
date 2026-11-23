package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/72nd/sanity/domain"
)

// OriginsFile is the name of the file in the inbox folder that remembers where
// each imported document came from. It starts with a dot, so the queue never
// lists it.
const OriginsFile = ".origins.json"

// ErrNoOrigin is returned by PutBack for a document that has no known origin
// (it was already in the inbox folder). Such a document can only be deleted.
var ErrNoOrigin = errors.New("inbox: document has no origin")

// ErrOutsideProject is returned by Adopt when the origin is not below the project root.
var ErrOutsideProject = errors.New("inbox: origin is outside the project")

type originsDoc struct {
	Version int               `json:"version"`
	Origins map[string]string `json:"origins"` // file path relative to the inbox folder (with /) -> absolute origin
}

// OpenQueue creates a queue on the inbox folder dir: it loads the origins and
// reads the documents that are already in the folder (the inbox survives a
// restart). It creates the folder if missing.
func OpenQueue(dir string, onChange func()) (*Queue, error) {
	q := NewQueue(onChange)
	if err := q.SetDir(dir); err != nil {
		return nil, err
	}
	q.Add(q.Dir())
	return q, nil
}

// SetDir makes dir the inbox folder: Import moves documents there and the
// origins are kept in dir/.origins.json. It loads the saved origins (records of
// files that are gone are dropped) but does not read the folder; Add does. Watch
// calls SetDir itself.
func (q *Queue) SetDir(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.dir, q.origins = abs, map[string]string{}
	data, err := os.ReadFile(filepath.Join(abs, OriginsFile))
	if err != nil {
		return nil // none yet
	}
	var doc originsDoc
	if json.Unmarshal(data, &doc) != nil { // damaged: start empty, the next save repairs it
		return nil
	}
	dirty := false
	for k, v := range doc.Origins {
		if _, err := os.Stat(filepath.Join(abs, filepath.FromSlash(k))); err != nil || v == "" {
			dirty = true
			continue
		}
		q.origins[k] = v
	}
	if dirty {
		return q.saveOriginsLocked()
	}
	return nil
}

// Dir is the inbox folder ("" when none was set).
func (q *Queue) Dir() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.dir
}

// Origin returns where the item came from.
func (q *Queue) Origin(id string) (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if i := q.indexLocked(id); i >= 0 && q.items[i].Origin != "" {
		return q.items[i].Origin, true
	}
	return "", false
}

// SetOrigin records origin for the file at path (inside the inbox folder) and
// saves it; a queued item gets the origin too. It is used to restore the origin
// after an undone filing.
func (q *Queue) SetOrigin(path, origin string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	q.mu.Lock()
	k, ok := q.keyLocked(abs)
	if !ok {
		q.mu.Unlock()
		return nil // not in the inbox folder: nothing to remember
	}
	q.origins[k] = origin
	if i := q.indexLocked(abs); i >= 0 {
		q.items[i].Origin = origin
	}
	err = q.saveOriginsLocked()
	q.mu.Unlock()
	q.changed()
	return err
}

// Import moves files and folders (recursively, PDFs only, hidden entries
// skipped) into the inbox folder and queues them, remembering each origin. A
// name that is taken gets a suffix (_2, _3, ...). Files already in the inbox
// folder are only queued. Without an inbox folder (see SetDir) it behaves like
// Add. It returns the ids of the new items; a file that cannot be moved is
// skipped and reported in the error.
func (q *Queue) Import(paths ...string) ([]string, error) {
	found := collectPDFs(paths)
	var ids []string
	var errs error
	q.mu.Lock()
	for _, p := range found {
		abs, err := filepath.Abs(p)
		if err != nil || q.indexLocked(abs) >= 0 {
			continue
		}
		if q.dir == "" || q.inDirLocked(abs) {
			q.items = append(q.items, q.newItemLocked(abs))
			ids = append(ids, abs)
			continue
		}
		rel := domain.UniquePath(filepath.Base(abs), func(n string) bool {
			_, err := os.Lstat(filepath.Join(q.dir, n))
			return err == nil
		})
		dst := filepath.Join(q.dir, rel)
		if _, err := MoveFile(abs, dst); err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		q.origins[rel] = abs
		q.items = append(q.items, q.newItemLocked(dst))
		ids = append(ids, dst)
	}
	if len(ids) > 0 && q.dir != "" {
		errs = errors.Join(errs, q.saveOriginsLocked())
	}
	q.mu.Unlock()
	if len(ids) > 0 {
		q.changed()
	}
	return ids, errs
}

// PutBack moves the item back to its origin and drops it from the queue. It
// returns the path the file went to. An item without origin gives ErrNoOrigin;
// an origin that is occupied is an error and nothing changes.
func (q *Queue) PutBack(id string) (string, error) {
	q.mu.Lock()
	i := q.indexLocked(id)
	if i < 0 {
		q.mu.Unlock()
		return "", fmt.Errorf("inbox: unknown item %s", id)
	}
	it := q.items[i]
	if it.Origin == "" {
		q.mu.Unlock()
		return "", ErrNoOrigin
	}
	if _, err := MoveFile(it.Path, it.Origin); err != nil {
		q.mu.Unlock()
		return "", err
	}
	q.items = append(q.items[:i], q.items[i+1:]...)
	q.forgetOriginLocked(it.Path)
	q.mu.Unlock()
	q.changed()
	return it.Origin, nil
}

// newItemLocked creates an item for the file abs; the origin comes from the saved records.
func (q *Queue) newItemLocked(abs string) *Item {
	return &Item{ID: abs, Path: abs, Name: filepath.Base(abs), AddedAt: q.now(), Origin: q.originLocked(abs)}
}

func (q *Queue) inDirLocked(abs string) bool {
	_, ok := q.keyLocked(abs)
	return ok
}

// keyLocked is the record key of a file inside the inbox folder.
func (q *Queue) keyLocked(abs string) (string, bool) {
	if q.dir == "" {
		return "", false
	}
	rel, err := filepath.Rel(q.dir, abs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func (q *Queue) originLocked(abs string) string {
	if k, ok := q.keyLocked(abs); ok {
		return q.origins[k]
	}
	return ""
}

// forgetOriginLocked drops the record of path (when it has one) and saves.
func (q *Queue) forgetOriginLocked(path string) {
	if k, ok := q.keyLocked(path); ok {
		if _, has := q.origins[k]; has {
			delete(q.origins, k)
			_ = q.saveOriginsLocked()
		}
	}
}

// saveOriginsLocked writes the records atomically; an empty set removes the file.
func (q *Queue) saveOriginsLocked() error {
	file := filepath.Join(q.dir, OriginsFile)
	if len(q.origins) == 0 {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(originsDoc{Version: 1, Origins: q.origins}, "", "  ")
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// AdoptRequest describes a document that goes back to its place and only gets
// a library entry.
type AdoptRequest struct {
	Path      string // current absolute path (in the inbox)
	Origin    string // absolute path it goes back to; below the project root
	Type      domain.DocumentType
	Values    domain.Values
	SHA256    string
	TextHash  string
	Overrides []string
	Text      string
}

// Adopt moves the document back to its origin (no renaming) and records a
// library entry for it. Unlike File it is not on the undo stack. When the entry
// cannot be stored the file goes back to the inbox.
func (f *Filer) Adopt(ctx context.Context, r AdoptRequest) (domain.LibraryEntry, error) {
	root, err := filepath.Abs(f.Root)
	if err != nil {
		return domain.LibraryEntry{}, err
	}
	origin, err := filepath.Abs(r.Origin)
	if err != nil {
		return domain.LibraryEntry{}, err
	}
	rel, err := filepath.Rel(root, origin)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return domain.LibraryEntry{}, ErrOutsideProject
	}
	mv, err := MoveFile(r.Path, origin)
	if err != nil {
		return domain.LibraryEntry{}, err
	}
	now := f.Now()
	e := domain.LibraryEntry{
		ID: f.nextID(now), TypeID: r.Type.ID, Path: filepath.ToSlash(rel), OriginalPath: r.Path,
		Date: r.Values.Date, Invoice: r.Values.Invoice, Amount: r.Values.Amount,
		SHA256: r.SHA256, TextHash: r.TextHash, FiledAt: now, Overrides: r.Overrides,
	}
	if err := f.Store.AddEntry(ctx, e); err != nil {
		if uerr := Undo(mv); uerr != nil {
			err = errors.Join(err, uerr)
		}
		return domain.LibraryEntry{}, err
	}
	if f.OnFiled != nil {
		f.OnFiled(e, r.Text)
	}
	return e, nil
}
