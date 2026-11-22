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
	// DeleteType removes a document type by id. An unknown id is an error;
	// library entries of the type are kept.
	DeleteType(ctx context.Context, id string) error
	// SaveProject replaces the project name, the inbox folder (relative to the
	// root) and the own identifiers the learning ignores.
	SaveProject(ctx context.Context, name, inbox string, ignore []string) error
	// AddEntry appends a library entry.
	AddEntry(ctx context.Context, e domain.LibraryEntry) error
	// RemoveEntry removes a library entry by id.
	RemoveEntry(ctx context.Context, id string) error
}
