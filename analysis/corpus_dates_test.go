package analysis

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/pdf"
)

// TestCorpusDates reports what date, invoice and amount detection finds in the
// PDFs of ../testdata. It is a report, not an assertion.
func TestCorpusDates(t *testing.T) {
	files, _ := filepath.Glob("../testdata/*.pdf")
	if len(files) == 0 {
		t.Skip("no corpus")
	}
	eng := pdf.NewEngine()
	for _, f := range files {
		name := filepath.Base(f)
		meta, err := eng.Meta(f)
		if err != nil {
			t.Logf("%s: meta: %v", name, err)
			continue
		}
		var parts []string
		skipped := false
		for p := 0; p < meta.Pages; p++ {
			pt, err := eng.Text(f, p)
			if err != nil {
				t.Logf("%s: page %d: %v", name, p, err)
				continue
			}
			if pt.Source == pdf.SourceVision && !eng.OCRAvailable() {
				skipped = true
			}
			parts = append(parts, pt.Text)
		}
		text := Normalize(strings.Join(parts, "\n"))
		if strings.TrimSpace(text) == "" {
			t.Logf("%s: no text (OCR available: %v, skipped: %v)", name, eng.OCRAvailable(), skipped)
			continue
		}
		lang := DetectLanguage(text)
		cands := FindDates(text, time.Now(), lang, domain.DateConfig{})
		t.Logf("%s: %d pages, %d chars, lang=%s, created=%s", name, meta.Pages, len(text), lang, meta.Created.Format("2006-01-02"))
		for i, c := range cands {
			if i >= 3 {
				break
			}
			t.Logf("  date %d: %s score=%.2f label=%q raw=%q period=%v", i+1, c.Date.Format("2006-01-02"), c.Score, c.Label, c.Raw, c.Period)
		}
		if len(cands) == 0 {
			fb := FallbackDate(meta.Created, name, time.Time{})
			t.Logf("  date: none; fallback %s (%s)", fb.Date.Format("2006-01-02"), fb.Label)
		}
		for i, inv := range FindInvoice(text, "") {
			if i >= 2 {
				break
			}
			t.Logf("  invoice %d: %s score=%.2f raw=%q", i+1, inv.Value, inv.Score, inv.Raw)
		}
		for i, a := range FindAmount(text) {
			if i >= 3 {
				break
			}
			t.Logf("  amount %d: %s score=%.2f raw=%q", i+1, fmtCents(a), a.Score, a.Raw)
		}
		for _, ib := range FindIBANs(text) {
			t.Logf("  iban: %s", ib.Value)
		}
		for _, v := range FindVATIDs(text) {
			t.Logf("  vat: %s", v.Value)
		}
	}
}

func fmtCents(a Amount) string {
	return fmt.Sprintf("%d.%02d %s", a.Cents/100, a.Cents%100, a.Currency)
}
