package store

import (
	"fmt"
	"slices"

	"github.com/72nd/sanity/domain"
)

// memory holds one project in plain Go values. It takes no locks; the
// callers (Fake, JSON) serialize access.
type memory struct {
	p domain.Project
}

// snapshot returns a deep copy of the project, so callers cannot alias the
// stored slices.
func (m *memory) snapshot() domain.Project {
	p := m.p
	p.IgnoreTokens = slices.Clone(p.IgnoreTokens)
	p.Types = cloneTypes(m.p.Types)
	p.Library = cloneLibrary(m.p.Library)
	return p
}

func (m *memory) saveType(t domain.DocumentType) {
	t = cloneType(t)
	for i := range m.p.Types {
		if m.p.Types[i].ID == t.ID {
			m.p.Types[i] = t
			return
		}
	}
	m.p.Types = append(m.p.Types, t)
}

func (m *memory) deleteType(id string) error {
	i := slices.IndexFunc(m.p.Types, func(t domain.DocumentType) bool { return t.ID == id })
	if i < 0 {
		return fmt.Errorf("document type %q not found", id)
	}
	m.p.Types = slices.Delete(m.p.Types, i, i+1)
	return nil
}

func (m *memory) addEntry(e domain.LibraryEntry) error {
	if m.indexOf(e.ID) >= 0 {
		return fmt.Errorf("library entry %q already exists", e.ID)
	}
	m.p.Library = append(m.p.Library, cloneEntry(e))
	return nil
}

func (m *memory) removeEntry(id string) error {
	i := m.indexOf(id)
	if i < 0 {
		return fmt.Errorf("library entry %q not found", id)
	}
	m.p.Library = slices.Delete(m.p.Library, i, i+1)
	return nil
}

func (m *memory) indexOf(id string) int {
	return slices.IndexFunc(m.p.Library, func(e domain.LibraryEntry) bool { return e.ID == id })
}

func cloneTypes(in []domain.DocumentType) []domain.DocumentType {
	out := make([]domain.DocumentType, 0, len(in))
	for _, t := range in {
		out = append(out, cloneType(t))
	}
	return out
}

func cloneType(t domain.DocumentType) domain.DocumentType {
	t.Rules = slices.Clone(t.Rules)
	t.Date.PreferredLabels = slices.Clone(t.Date.PreferredLabels)
	t.Date.IgnoreLabels = slices.Clone(t.Date.IgnoreLabels)
	return t
}

func cloneLibrary(in []domain.LibraryEntry) []domain.LibraryEntry {
	out := make([]domain.LibraryEntry, 0, len(in))
	for _, e := range in {
		out = append(out, cloneEntry(e))
	}
	return out
}

func cloneEntry(e domain.LibraryEntry) domain.LibraryEntry {
	e.Overrides = slices.Clone(e.Overrides)
	return e
}
