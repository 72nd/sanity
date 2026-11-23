package inbox

import (
	"context"
	"path/filepath"
	"strings"
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

const kontistStatement = `Kontist
Kontoauszug 04/2025
Erstellt am 31.10.2025
Saldo 1.234,56 EUR
30.04.2025 30.04.2025 Telekom Deutschland GmbH -49,95
15.04.2025 16.04.2025 Gehalt 2.000,00
Solaris Bank`

func TestAnalyzeStatementDateAndType(t *testing.T) {
	a := &Analyzer{
		PDF:   pdf.NewFake(kontistStatement),
		Store: store.NewFake(),
		Now:   func() time.Time { return time.Date(2025, 11, 22, 0, 0, 0, 0, time.UTC) },
	}
	r := a.Analyze(context.Background(), writePDF(t, t.TempDir(), "scan.pdf"))
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if len(r.Suggestions) == 0 || r.Suggestions[0].TypeID != "kontist-statement" || !r.Suggestions[0].Confident {
		t.Fatalf("suggestions %+v", r.Suggestions)
	}
	for _, s := range r.Suggestions {
		if s.TypeID == "telekom" && s.Confident {
			t.Errorf("telekom must not be confident on a statement: %+v", s)
		}
	}
	want := time.Date(2025, 4, 30, 0, 0, 0, 0, time.UTC)
	if len(r.Dates) == 0 || !r.Dates[0].Date.Equal(want) || r.Dates[0].Score < 0.8 {
		t.Fatalf("dates %+v", r.Dates)
	}
	if r.StatementDate == nil || !r.StatementDate.Date.Equal(want) {
		t.Errorf("statement date %+v", r.StatementDate)
	}
	seen := map[time.Time]bool{}
	for _, d := range r.Dates {
		if seen[d.Date] {
			t.Errorf("date %v listed twice", d.Date)
		}
		seen[d.Date] = true
	}
}

const cursorColumnsText = `Cursor
Invoice
Invoice number
Data of issue
Date due
56546565A-0682
April 16, 2026
April 16, 2026
Total 20,00 EUR`

func TestAnalyzePairsColumns(t *testing.T) {
	a := &Analyzer{
		PDF:   pdf.NewFake(cursorColumnsText),
		Store: store.NewFake(),
		Now:   func() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) },
	}
	r := a.Analyze(context.Background(), writePDF(t, t.TempDir(), "cursor.pdf"))
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if len(r.Invoices) == 0 || r.Invoices[0].Value != "56546565A-0682" {
		t.Errorf("invoices %+v", r.Invoices)
	}
	if len(r.Dates) == 0 || !r.Dates[0].Date.Equal(time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("dates %+v", r.Dates)
	}
	if strings.Contains(r.Text, "paired columns") {
		t.Error("the stored text must stay the extracted original")
	}
}

func TestAnalyzeAmountLabelsFromType(t *testing.T) {
	st := store.NewFake()
	ctx := context.Background()
	pr, _ := st.Project(ctx)
	ty, _ := pr.TypeByID("vodafone-invoice")
	ty.Amount.PreferredLabels = []string{"Betrag"}
	if err := st.SaveType(ctx, ty); err != nil {
		t.Fatal(err)
	}
	text := "Vodafone GmbH\nvodafone.de\nPosition 100,00 EUR\nBetrag 8,00 EUR\nGesamt 108,00 EUR\n"
	a := &Analyzer{PDF: pdf.NewFake(text), Store: st, Now: func() time.Time { return time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC) }}
	r := a.Analyze(ctx, writePDF(t, t.TempDir(), "v.pdf"))
	if r.Err != nil || len(r.Suggestions) == 0 || r.Suggestions[0].TypeID != "vodafone-invoice" {
		t.Fatalf("err %v suggestions %+v", r.Err, r.Suggestions)
	}
	if len(r.Amounts) == 0 || r.Amounts[0].Cents != 800 {
		t.Errorf("amounts %+v", r.Amounts)
	}
}
