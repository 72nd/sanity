// Package pdf renders PDFs and extracts text. The macOS implementation uses
// PDFKit and Vision through cgo (pdf_darwin.go); other platforms get a stub.
// It imports no UI.
package pdf

import (
	"errors"
	"image"
	"time"
)

// ErrUnsupported is returned on platforms without an implementation.
var ErrUnsupported = errors.New("pdf: unsupported platform")

// TextSource says where a page's text came from.
type TextSource string

const (
	SourceLayer  TextSource = "layer"  // embedded text layer
	SourceVision TextSource = "vision" // Vision OCR of the rendered page
)

// Meta is what the PDF itself tells about the document.
type Meta struct {
	Pages    int
	Producer string
	Creator  string
	Created  time.Time // zero when unknown
}

// PageText is the text of one page.
type PageText struct {
	Text   string
	Source TextSource
	Scan   bool // the page was classified as a scan
}

// Engine is what the rest of the app needs from the PDF layer.
type Engine interface {
	// Meta reads page count and document info.
	Meta(path string) (Meta, error)
	// Render renders a 0-based page; scale 1.0 is 72 dpi.
	Render(path string, page int, scale float64) (image.Image, error)
	// Text returns the best text of a page: the layer for born-digital pages,
	// Vision OCR for scans (scan detection per PLAN.md section 3).
	Text(path string, page int) (PageText, error)
	// Dates returns date strings found by the system data detector (second opinion).
	Dates(text string) []string
	// OCRAvailable reports whether Vision returns text (false inside the agent sandbox).
	OCRAvailable() bool
}

// Closer is implemented by engines that keep documents open (the macOS
// engine). Evict forgets one file, which callers do after filing, moving or
// trashing it; Close drops everything. Check with EvictPath instead of
// asserting by hand.
type Closer interface {
	Evict(path string)
	Close()
}

// EvictPath calls Evict when the engine keeps documents open.
func EvictPath(e Engine, path string) {
	if c, ok := e.(Closer); ok {
		c.Evict(path)
	}
}
