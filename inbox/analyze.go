package inbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

// maxAnalysedPages caps the pages read per document. Receipts put what we
// need (letterhead, date, number, total) on the first pages.
const maxAnalysedPages = 4

// Result is the analysis of one inbox document.
type Result struct {
	SHA256      string
	TextHash    string
	Lang        string
	Scan        bool // at least one page came from OCR or was classified as a scan
	Suggestions []analysis.Suggestion
	Dates       []analysis.DateCandidate
	Invoices    []analysis.Found
	Amounts     []analysis.Amount
	Duplicate   *domain.LibraryEntry
	DupReason   string
	Err         error
}

// Analyzer runs the recognition pipeline of PLAN.md section 3 on a PDF.
type Analyzer struct {
	PDF     pdf.Engine
	Store   store.Store
	Learned analysis.LearnedIndex // may be nil
	Now     func() time.Time
}

// Analyze reads the document and returns suggestions. It blocks (hashing,
// rendering, OCR), so call it from a goroutine.
func (a *Analyzer) Analyze(ctx context.Context, path string) Result {
	var r Result
	sum, err := FileHash(path)
	if err != nil {
		r.Err = err
		return r
	}
	r.SHA256 = sum
	meta, err := a.PDF.Meta(path)
	if err != nil {
		r.Err = err
		return r
	}
	var parts []string
	for p := 0; p < min(meta.Pages, maxAnalysedPages); p++ {
		if ctx.Err() != nil {
			r.Err = ctx.Err()
			return r
		}
		pt, err := a.PDF.Text(path, p)
		if err != nil {
			continue
		}
		r.Scan = r.Scan || pt.Scan || pt.Source == pdf.SourceVision
		parts = append(parts, pt.Text)
	}
	text := analysis.Normalize(strings.Join(parts, "\n"))
	r.TextHash = analysis.TextHash(text)
	r.Lang = analysis.DetectLanguage(text)
	now := time.Now
	if a.Now != nil {
		now = a.Now
	}
	pr, err := a.Store.Project(ctx)
	if err != nil {
		r.Err = err
		return r
	}
	name := filepath.Base(path)
	doc := analysis.Doc{Text: text, FileName: name}
	r.Suggestions = analysis.Classify(doc, pr.Types, a.Learned)

	var cfg domain.DateConfig
	if len(r.Suggestions) > 0 {
		if t, ok := pr.TypeByID(r.Suggestions[0].TypeID); ok {
			cfg = t.Date
		}
	}
	r.Dates = analysis.FindDates(text, now(), r.Lang, cfg)
	var mtime time.Time
	if st, err := os.Stat(path); err == nil {
		mtime = st.ModTime()
	}
	if fb := analysis.FallbackDate(meta.Created, name, mtime); !fb.Date.IsZero() {
		r.Dates = append(r.Dates, fb)
	}
	r.Invoices = analysis.FindInvoice(text, "")
	r.Amounts = analysis.FindAmount(text)

	typeID, invoice := "", ""
	if len(r.Suggestions) > 0 {
		typeID = r.Suggestions[0].TypeID
	}
	if len(r.Invoices) > 0 {
		invoice = r.Invoices[0].Value
	}
	if e, why, ok := analysis.FindDuplicate(pr.Library, sum, r.TextHash, typeID, invoice); ok {
		r.Duplicate, r.DupReason = &e, why
	}
	return r
}
