package inbox

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

const telekomText = `Telekom Deutschland GmbH
Rechnung
Rechnungsdatum 12.03.2025
Rechnungsnummer R-88123
Gesamtbetrag 49,95 EUR
IBAN DE02 1203 0000 0000 2020 51`

func TestAnalyzeFindsFields(t *testing.T) {
	dir := t.TempDir()
	path := writePDF(t, dir, "scan_004.pdf")
	a := &Analyzer{
		PDF:   pdf.NewFake(telekomText),
		Store: store.NewFake(),
		Now:   func() time.Time { return time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC) },
	}
	r := a.Analyze(context.Background(), path)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if r.SHA256 == "" || r.TextHash == "" {
		t.Error("hashes missing")
	}
	if len(r.Dates) == 0 || !r.Dates[0].Date.Equal(time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("dates %+v", r.Dates)
	}
	if len(r.Invoices) == 0 || r.Invoices[0].Value != "R-88123" {
		t.Errorf("invoices %+v", r.Invoices)
	}
	if len(r.Amounts) == 0 || r.Amounts[0].Cents != 4995 {
		t.Errorf("amounts %+v", r.Amounts)
	}
}

func TestAnalyzeMissingFile(t *testing.T) {
	a := &Analyzer{PDF: pdf.NewFake(), Store: store.NewFake()}
	if r := a.Analyze(context.Background(), filepath.Join(t.TempDir(), "nope.pdf")); r.Err == nil {
		t.Error("expected error")
	}
}
