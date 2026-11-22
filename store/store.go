// Package store defines how sanity reaches project data. Presenters depend on
// the Store interface only. No UI imports.
package store

import (
	"context"

	"github.com/72nd/sanity/domain"
)

// Store loads and saves one project.
type Store interface {
	// Project returns the current project.
	Project(ctx context.Context) (domain.Project, error)
	// Library returns all filed documents.
	Library(ctx context.Context) ([]domain.LibraryEntry, error)
	// Types returns the document types.
	Types(ctx context.Context) ([]domain.DocumentType, error)
	// SaveType inserts or replaces a document type by id.
	SaveType(ctx context.Context, t domain.DocumentType) error
	// AddEntry appends a library entry.
	AddEntry(ctx context.Context, e domain.LibraryEntry) error
	// RemoveEntry removes a library entry by id.
	RemoveEntry(ctx context.Context, id string) error
}
