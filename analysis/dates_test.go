package analysis

import (
	"testing"
	"time"

	"github.com/72nd/sanity/domain"
)

var testNow = time.Date(2025, 11, 22, 12, 0, 0, 0, time.UTC)

func ymd(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func dates(text, lang string) []DateCandidate {
	return FindDates(text, testNow, lang, domain.DateConfig{})
}

func has(c []DateCandidate, d time.Time) bool {
	for _, x := range c {
		if x.Date.Equal(d) {
			return true
		}
	}
	return false
}

func TestFindDatesFormats(t *testing.T) {
	cases := []struct {
		name, text, lang string
		want             time.Time
	}{
		{"dmy dots", "Datum: 12.03.2025", "de", ymd(2025, 3, 12)},
		{"dmy short year", "Rechnungsdatum 12.03.25", "de", ymd(2025, 3, 12)},
		{"dmy single digits", "Zürich, 5.3.2025", "de", ymd(2025, 3, 5)},
		{"iso", "Date: 2025-03-12", "en", ymd(2025, 3, 12)},
		{"iso time", "created 2025-03-12T10:00:00", "en", ymd(2025, 3, 12)},
		{"de month name", "Berlin, 12. März 2025", "de", ymd(2025, 3, 12)},
		{"de maerz", "12 Maerz 2025", "de", ymd(2025, 3, 12)},
		{"de abbreviated", "12. Mär 2025", "de", ymd(2025, 3, 12)},
		{"de dez", "Datum 24. Dez. 2024", "de", ymd(2024, 12, 24)},
		{"de januar", "1. Januar 2025", "de", ymd(2025, 1, 1)},
		{"en month day", "Invoice date: March 12, 2025", "en", ymd(2025, 3, 12)},
		{"en abbreviated", "Paid Mar 12 2025", "en", ymd(2025, 3, 12)},
		{"en day month", "12 March 2025", "en", ymd(2025, 3, 12)},
		{"en ordinal", "March 3rd, 2025", "en", ymd(2025, 3, 3)},
		{"slash mdy default", "Date 03/12/2025", "en", ymd(2025, 3, 12)},
		{"slash dmy by lang", "Datum 03/12/2025", "de", ymd(2025, 12, 3)},
		{"slash dmy by day>12", "Date 25/03/2025", "en", ymd(2025, 3, 25)},
		{"slash mdy by day>12", "Date 03/25/2025", "de", ymd(2025, 3, 25)},
		{"swiss", "Bern, den 12.3.2025", "de", ymd(2025, 3, 12)},
	}
	for _, c := range cases {
		got := dates(c.text, c.lang)
		if len(got) == 0 || !got[0].Date.Equal(c.want) {
			t.Errorf("%s: %q -> %v, want top %s", c.name, c.text, got, c.want.Format("2006-01-02"))
		}
	}
}

func TestFindDatesRejects(t *testing.T) {
	for _, text := range []string{
		"Version 1.2.3", "IP 192.168.100.200", "Betrag 1.234,56", "Nr 112.03.2025", "31.02.2025", "12.03.20256",
		"Art 2025-03-123",
	} {
		if got := dates(text, "de"); len(got) != 0 {
			t.Errorf("%q should have no date, got %v", text, got)
		}
	}
}

func TestFindDatesLabels(t *testing.T) {
	text := "Rechnungsdatum: 12.03.2025\nFälligkeit: 26.03.2025\nLieferdatum 10.03.2025\n"
	got := dates(text, "de")
	if len(got) != 3 || !got[0].Date.Equal(ymd(2025, 3, 12)) {
		t.Fatalf("got %v", got)
	}
	if got[0].Score <= got[1].Score+0.3 {
		t.Errorf("invoice date should clearly beat the others: %v", got)
	}
	en := dates("Payment due date: Mar 26, 2025\nInvoice date: Mar 12, 2025", "en")
	if !en[0].Date.Equal(ymd(2025, 3, 12)) {
		t.Errorf("english labels: %v", en)
	}
	kto := dates("Kontoauszug vom 31.10.2025\nzahlbar bis 14.11.2025", "de")
	if !kto[0].Date.Equal(ymd(2025, 10, 31)) {
		t.Errorf("kontoauszug: %v", kto)
	}
}

func TestFindDatesTable(t *testing.T) {
	text := "Rechnungsdatum   Fälligkeit\n12.03.2025   26.03.2025\n"
	got := dates(text, "de")
	if !got[0].Date.Equal(ymd(2025, 3, 12)) || got[0].Label == "" {
		t.Errorf("header-over-value table: %v", got)
	}
}

func TestFindDatesConfig(t *testing.T) {
	text := "Buchungsdatum 01.03.2025\nValuta 05.03.2025\n"
	cfg := domain.DateConfig{PreferredLabels: []string{"Valuta"}}
	got := FindDates(text, testNow, "de", cfg)
	if !got[0].Date.Equal(ymd(2025, 3, 5)) {
		t.Errorf("preferred label: %v", got)
	}
	cfg = domain.DateConfig{IgnoreLabels: []string{"Buchungsdatum"}}
	got = FindDates("Buchungsdatum 01.03.2025\nBelegdatum 05.03.2025", testNow, "de", cfg)
	if !got[0].Date.Equal(ymd(2025, 3, 5)) || got[1].Score > 0.3 {
		t.Errorf("ignore label: %v", got)
	}
}

func TestFindDatesPlausibility(t *testing.T) {
	got := dates("Datum 12.03.2031\nDatum 12.03.2025", "de")
	if !got[0].Date.Equal(ymd(2025, 3, 12)) || got[1].Score >= got[0].Score-0.3 {
		t.Errorf("future date: %v", got)
	}
	cfg := domain.DateConfig{MaxAgeYears: 5}
	got = FindDates("Datum 12.03.2010\nDatum 12.03.2025", testNow, "de", cfg)
	if !got[0].Date.Equal(ymd(2025, 3, 12)) || got[1].Score >= got[0].Score-0.3 {
		t.Errorf("too old: %v", got)
	}
}

func TestFindDatesRepetition(t *testing.T) {
	got := dates("Rechnung\n12.03.2025\nSeite 1\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n12.03.2025\n13.03.2025", "de")
	if !got[0].Date.Equal(ymd(2025, 3, 12)) || len(got) != 2 {
		t.Errorf("repeated date should win and be deduplicated: %v", got)
	}
}

func TestFindDatesPeriod(t *testing.T) {
	text := "Rechnungsdatum 05.04.2025\nAbrechnungszeitraum 01.03.2025 - 31.03.2025\n"
	got := dates(text, "de")
	if !got[0].Date.Equal(ymd(2025, 4, 5)) {
		t.Fatalf("got %v", got)
	}
	for _, c := range got[1:] {
		if !c.Period || c.Score > 0.2 {
			t.Errorf("period date should be a separate low-score value: %+v", c)
		}
	}
	got = dates("Leistungszeitraum: 01.03.2025 bis 31.03.2025", "de")
	if len(got) != 2 || !got[0].Period {
		t.Errorf("bis range: %v", got)
	}
}

func TestFindDatesOCRNoise(t *testing.T) {
	got := dates(Normalize("Rechnungsdatum: 12.O3.2O25"), "de")
	if len(got) == 0 || !got[0].Date.Equal(ymd(2025, 3, 12)) {
		t.Errorf("normalised OCR text: %v", got)
	}
}

func TestFallbackDate(t *testing.T) {
	created := time.Date(2025, 3, 1, 8, 30, 0, 0, time.UTC)
	mtime := time.Date(2025, 5, 5, 8, 30, 0, 0, time.UTC)
	f := FallbackDate(created, "rechnung_2025-02-10.pdf", mtime)
	if !f.Fallback || !f.Date.Equal(ymd(2025, 3, 1)) || f.Score >= 0.5 {
		t.Errorf("created: %+v", f)
	}
	f = FallbackDate(time.Time{}, "scan_20250210_1.pdf", mtime)
	if !f.Fallback || !f.Date.Equal(ymd(2025, 2, 10)) {
		t.Errorf("file name: %+v", f)
	}
	f = FallbackDate(time.Time{}, "10.02.2025 Telekom.pdf", mtime)
	if !f.Date.Equal(ymd(2025, 2, 10)) {
		t.Errorf("dmy file name: %+v", f)
	}
	f = FallbackDate(time.Time{}, "scan.pdf", mtime)
	if !f.Fallback || !f.Date.Equal(ymd(2025, 5, 5)) {
		t.Errorf("mtime: %+v", f)
	}
	if f := FallbackDate(time.Time{}, "x.pdf", time.Time{}); !f.Date.IsZero() || !f.Fallback {
		t.Errorf("nothing: %+v", f)
	}
}

func TestFindDatesPrintDate(t *testing.T) {
	got := dates("Belegsdatum 24. Juli 2025\nDruckdatum: 05.11.2025", "de")
	if !got[0].Date.Equal(ymd(2025, 7, 24)) || got[1].Score > 0.3 {
		t.Errorf("%v", got)
	}
}
