package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/72nd/sanity/domain"
)

// JSON is a Store that persists one project to <root>/office/sanity.json.
// Every mutation rewrites the whole file atomically.
type JSON struct {
	mu   sync.Mutex
	path string
	m    memory
}

var _ Store = (*JSON)(nil)

// OpenJSON opens the project at root. A missing sanity.json is created with a
// default project named after the root folder.
func OpenJSON(root string) (*JSON, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(abs, "office", "sanity.json")

	var p domain.Project
	created := false
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		p = domain.Project{
			Name:         filepath.Base(abs),
			Inbox:        domain.DefaultInbox,
			IgnoreTokens: []string{},
			Types:        []domain.DocumentType{},
			Library:      []domain.LibraryEntry{},
		}
		created = true
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	p.Root = abs
	if p.Inbox == "" {
		p.Inbox = domain.DefaultInbox
	}

	j := &JSON{path: path, m: memory{p: p}}
	if created {
		if err := j.save(); err != nil {
			return nil, err
		}
	}
	return j, nil
}

// Project implements Store.
func (j *JSON) Project(ctx context.Context) (domain.Project, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.m.snapshot(), nil
}

// Library implements Store.
func (j *JSON) Library(ctx context.Context) ([]domain.LibraryEntry, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return cloneLibrary(j.m.p.Library), nil
}

// Types implements Store.
func (j *JSON) Types(ctx context.Context) ([]domain.DocumentType, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return cloneTypes(j.m.p.Types), nil
}

// SaveType implements Store.
func (j *JSON) SaveType(ctx context.Context, t domain.DocumentType) error {
	return j.mutate(func(m *memory) error {
		m.saveType(t)
		return nil
	})
}

// DeleteType implements Store.
func (j *JSON) DeleteType(ctx context.Context, id string) error {
	return j.mutate(func(m *memory) error {
		return m.deleteType(id)
	})
}

// AddEntry implements Store.
func (j *JSON) AddEntry(ctx context.Context, e domain.LibraryEntry) error {
	return j.mutate(func(m *memory) error {
		return m.addEntry(e)
	})
}

// RemoveEntry implements Store.
func (j *JSON) RemoveEntry(ctx context.Context, id string) error {
	return j.mutate(func(m *memory) error {
		return m.removeEntry(id)
	})
}

// mutate applies fn and persists the result. If fn or the write fails, the
// in-memory project is restored to the state before the call.
func (j *JSON) mutate(fn func(m *memory) error) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	prev := j.m.snapshot()
	if err := fn(&j.m); err != nil {
		j.m.p = prev
		return err
	}
	if err := j.save(); err != nil {
		j.m.p = prev
		return err
	}
	return nil
}

// save writes the project as indented JSON. The caller holds j.mu.
func (j *JSON) save() error {
	data, err := json.MarshalIndent(j.m.p, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(j.path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(j.path, data, 0o644)
}

// writeFileAtomic writes data to a temp file in the same directory and renames
// it over path, so readers see either the old or the new file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	done := false
	defer func() {
		if !done {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	done = true
	return nil
}
