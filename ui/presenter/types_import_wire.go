package presenter

import (
	"context"
	"path/filepath"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/prompts"
)

// defaultImportFuncs wires the real analysis, prompt and PDF pieces.
func defaultImportFuncs() ImportFuncs {
	var eng pdf.Engine // created on first use; the engine holds open documents
	return ImportFuncs{
		Parse:    analysis.ParseSnippet,
		Check:    analysis.CheckType,
		TypeJSON: analysis.TypeJSON,
		Prompt:   prompts.TypeSetup,
		Extract: func(ctx context.Context, path string) (string, error) {
			if eng == nil {
				eng = pdf.NewEngine()
			}
			x, err := inbox.ExtractText(ctx, eng, path)
			return x.Text, err
		},
		LibraryText: func(root string) analysis.LibraryText {
			c := inbox.OpenCache(inbox.CachePath(filepath.Clean(root)))
			return func(sha string) (string, bool) {
				e, ok := c.Get(sha)
				return e.Text, ok && e.Text != ""
			}
		},
	}
}
