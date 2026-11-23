package analysis

import (
	"strings"
	"testing"

	"github.com/72nd/sanity/domain"
)

const cursorColumns = `Cursor
Invoice
Invoice number
Data of issue
Date due
56546565A-0682
April 16, 2026
April 16, 2026
Bill to
Muster GmbH
`

func TestPairColumnsCursor(t *testing.T) {
	got := PairColumns(cursorColumns)
	if !strings.HasPrefix(got, strings.TrimRight(cursorColumns, "\n")) {
		t.Fatal("the original text must stay in front, unchanged")
	}
	for _, want := range []string{PairedMarker, "Invoice number: 56546565A-0682", "Date of issue: April 16, 2026", "Due date: April 16, 2026"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if inv := FindInvoice(got, ""); len(inv) == 0 || inv[0].Value != "56546565A-0682" {
		t.Errorf("invoice: %+v", inv)
	}
	if inv := FindInvoice(cursorColumns, ""); len(inv) != 0 {
		t.Errorf("without pairing the invoice is not found: %+v", inv)
	}
	ds := FindDates(got, testNow.AddDate(1, 0, 0), "en", domain.DateConfig{})
	if len(ds) == 0 || !ds[0].Date.Equal(ymd(2026, 4, 16)) {
		t.Errorf("dates: %+v", ds)
	}
}

func TestPairColumnsAmount(t *testing.T) {
	got := PairColumns("Rechnung\nRechnungsnummer\nBetrag\nRE-2025-77\n12,50 €\n")
	if !strings.Contains(got, "Rechnungsnummer: RE-2025-77") || !strings.Contains(got, "Betrag: 12,50 €") {
		t.Errorf("got\n%s", got)
	}
}

func TestPairColumnsNegative(t *testing.T) {
	cases := map[string]string{
		"fewer labels than values": "Invoice number\n56546565A-0682\nApril 16, 2026\n",
		"unknown labels":           "Foo\nBar\nAB-1234\nMay 3, 2026\n",
		"inline values":            "Invoice number: 56546565A-0682\nDate of issue: April 16, 2026\n",
		"ambiguous run":            "Invoice number\nDate of issue\nTotal\n56546565A-0682\nApril 16, 2026\n",
		"no values":                "Invoice\nInvoice number\nDate of issue\n",
	}
	for name, text := range cases {
		if got := PairColumns(text); got != text {
			t.Errorf("%s: text must stay untouched, got\n%s", name, got)
		}
	}
}
