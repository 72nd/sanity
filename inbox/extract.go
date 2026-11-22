package inbox

import (
	"context"
	"strings"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/pdf"
)

// Extracted is the normalized text of a PDF.
type Extracted struct {
	Text   string
	Scan   bool           // a page came from OCR or was classified as a scan
	Source pdf.TextSource // vision when any page needed OCR, else layer
	Meta   pdf.Meta
}

// ExtractText reads the first pages of a PDF and returns the normalized text.
// It is the one text pipeline shared by Analyzer and Learner. A meta error is
// returned; page errors skip the page.
func ExtractText(ctx context.Context, eng pdf.Engine, path string) (Extracted, error) {
	var x Extracted
	meta, err := eng.Meta(path)
	if err != nil {
		return x, err
	}
	x.Meta = meta
	x.Source = pdf.SourceLayer
	var parts []string
	for p := 0; p < min(meta.Pages, maxAnalysedPages); p++ {
		if err := ctx.Err(); err != nil {
			return x, err
		}
		pt, err := eng.Text(path, p)
		if err != nil {
			continue
		}
		if pt.Source == pdf.SourceVision {
			x.Source = pdf.SourceVision
		}
		x.Scan = x.Scan || pt.Scan || pt.Source == pdf.SourceVision
		parts = append(parts, pt.Text)
	}
	x.Text = analysis.Normalize(strings.Join(parts, "\n"))
	return x, nil
}
