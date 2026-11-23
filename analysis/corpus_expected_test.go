package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/pdf"
)

type expectedDoc struct {
	Date     string  `json:"date"`
	Invoice  *string `json:"invoice"`
	Amount   string  `json:"amount"`
	Currency string  `json:"currency"`

	// Type settings the document would be filed under (optional).
	Statement    bool     `json:"statement"`
	TypeCurrency string   `json:"typeCurrency"`
	AmountLabels []string `json:"amountLabels"`
}

// TestCorpusExpected compares the top date, invoice and amount candidate with
// the hand-checked values in ../testdata/expected.json and logs the hit rate.
// It is a report, not an assertion: it skips without the (private) corpus and
// without Vision, since scans have no text then.
func TestCorpusExpected(t *testing.T) {
	raw, err := os.ReadFile("../testdata/expected.json")
	if err != nil {
		t.Skip("no expected.json")
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	eng := pdf.NewEngine()
	var hit, total int
	for name, msg := range all {
		if strings.HasPrefix(name, "_") {
			continue
		}
		var want expectedDoc
		if err := json.Unmarshal(msg, &want); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		f := filepath.Join("../testdata", name)
		meta, err := eng.Meta(f)
		if err != nil {
			continue // sample not present locally
		}
		var parts []string
		for p := 0; p < meta.Pages; p++ {
			if pt, err := eng.Text(f, p); err == nil {
				parts = append(parts, pt.Text)
			}
		}
		text := PairColumns(FixLabels(Normalize(strings.Join(parts, "\n"))))
		if strings.TrimSpace(text) == "" {
			t.Logf("%s: no text (OCR available: %v)", name, eng.OCRAvailable())
			continue
		}
		check := func(field, got, exp string) {
			total++
			if got == exp {
				hit++
				return
			}
			t.Logf("MISS %s %s: got %q, want %q", name, field, got, exp)
		}
		var date, amount, invoice string
		cands := FindDates(text, time.Now(), DetectLanguage(text), domain.DateConfig{})
		if len(cands) > 0 {
			date = cands[0].Date.Format("2006-01-02")
		}
		if sc, ok := StatementDate(cands); ok && want.Statement {
			date = sc.Date.Format("2006-01-02")
		}
		if a := FindAmountFor(text, domain.AmountConfig{PreferredLabels: want.AmountLabels}, want.TypeCurrency); len(a) > 0 {
			amount = fmtCents(a[0])
		}
		if inv := FindInvoice(text, ""); len(inv) > 0 {
			invoice = inv[0].Value
		}
		check("date", date, want.Date)
		check("amount", amount, want.Amount+" "+want.Currency)
		if want.Invoice != nil {
			check("invoice", invoice, *want.Invoice)
		}
	}
	if total == 0 {
		t.Skip("no corpus samples")
	}
	t.Logf("hit rate: %d/%d", hit, total)
}
