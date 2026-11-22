package analysis

import "testing"

func TestFindInvoice(t *testing.T) {
	cases := []struct{ name, text, override, want string }{
		{"de", "Rechnungsnummer: R-88123\nDatum 12.03.2025", "", "R-88123"},
		{"de nr dot", "Rechnung Nr. 2025-0042 vom 12.03.2025", "", "2025-0042"},
		{"de re-nr", "RE-Nr.: 100234", "", "100234"},
		{"beleg", "Beleg-Nr. 99887766", "", "99887766"},
		{"en number", "Invoice number: INV-2025-0099", "", "INV-2025-0099"},
		{"en hash", "Invoice #A1B2C3D4", "", "A1B2C3D4"},
		{"en no", "Invoice No. 55012", "", "55012"},
		{"trailing dot", "Rechnungsnummer 123456.", "", "123456"},
		{"override", "Ref XY-77 Rechnungsnummer 1234567", `Ref ([A-Z]{2}-\d+)`, "XY-77"},
		{"bad override falls back", "Rechnungsnummer 1234567", `(`, "1234567"},
	}
	for _, c := range cases {
		got := FindInvoice(c.text, c.override)
		if len(got) == 0 || got[0].Value != c.want {
			t.Errorf("%s: got %v want %s", c.name, got, c.want)
		}
	}
	if got := FindInvoice("Rechnungsnummer Datum\nKundennummer", ""); len(got) != 0 {
		t.Errorf("word without digits is no number: %v", got)
	}
}

func TestFindAmount(t *testing.T) {
	cases := []struct {
		name, text string
		cents      int64
		cur        string
	}{
		{"de", "Zwischensumme 1.000,00 EUR\nMwSt 19% 190,00 EUR\nGesamtbetrag 1.190,00 EUR", 119000, "EUR"},
		{"de euro sign", "Summe: 49,95 €", 4995, "EUR"},
		{"de zu zahlen", "Netto 41,98\nMwSt 7,97\nzu zahlen 49,95", 4995, ""},
		{"ch", "Total CHF 1'234.50\nMWST 8.1% 92.50", 123450, "CHF"},
		{"ch typographic", "Rechnungsbetrag CHF 1’234.50", 123450, "CHF"},
		{"us", "Subtotal $1,200.00\nTax $96.00\nTotal $1,296.00", 129600, "USD"},
		{"us amount due", "Amount due: $49.95", 4995, "USD"},
		{"balance due", "Balance due USD 12.50", 1250, "USD"},
		{"negative", "Total -12,50 EUR", -1250, "EUR"},
	}
	for _, c := range cases {
		got := FindAmount(c.text)
		if len(got) == 0 || got[0].Cents != c.cents || got[0].Currency != c.cur {
			t.Errorf("%s: got %+v want %d %s", c.name, got, c.cents, c.cur)
		}
	}
	for _, text := range []string{"Datum 12.03.2025", "MwSt 19,00 %", "Tel 089 1234567", "Version 1.2.3"} {
		if got := FindAmount(text); len(got) != 0 && text != "MwSt 19,00 %" {
			t.Errorf("%q: unexpected %+v", text, got)
		}
	}
	if got := FindAmount("MwSt 19,00 %"); len(got) != 0 {
		t.Errorf("percent is no amount: %+v", got)
	}
}

func TestParseMoney(t *testing.T) {
	for in, want := range map[string]int64{"1.234,56": 123456, "1'234.56": 123456, "1,234.56": 123456, "12,50": 1250, "0.99": 99, "1.234.567,89": 123456789} {
		if got, ok := parseMoney(in); !ok || got != want {
			t.Errorf("%s: %d %v", in, got, ok)
		}
	}
}

func TestIBAN(t *testing.T) {
	valid := []string{"DE89 3704 0044 0532 0130 00", "CH93 0076 2011 6238 5295 7", "AT61 1904 3002 3457 3201", "GB82 WEST 1234 5698 7654 32", "DE89370400440532013000"}
	for _, v := range valid {
		if !IBANValid(v) {
			t.Errorf("%s should be valid", v)
		}
		got := FindIBANs("IBAN: " + v + " BIC: COBADEFF")
		if len(got) != 1 {
			t.Errorf("%s: found %v", v, got)
		}
	}
	for _, v := range []string{"DE89 3704 0044 0532 0130 01", "DE00 0000 0000 0000 0000 00", "XX"} {
		if IBANValid(v) {
			t.Errorf("%s should be invalid", v)
		}
	}
	got := FindIBANs("Konto DE89 3704 0044 0532 0130 00 COBA\nzweite: CH9300762011623852957")
	if len(got) != 2 || got[0].Value != "DE89370400440532013000" || got[1].Value != "CH9300762011623852957" {
		t.Errorf("trailing word and unspaced: %v", got)
	}
	if got := FindIBANs("DE89 3704 0044 0532 0130 01"); len(got) != 0 {
		t.Errorf("bad checksum: %v", got)
	}
}

func TestVATIDs(t *testing.T) {
	cases := []struct{ text, want string }{
		{"USt-IdNr.: DE123456789", "DE123456789"},
		{"UID ATU12345678", "ATU12345678"},
		{"MWST-Nr. CHE-123.456.789 MWST", "CHE-123.456.789"},
		{"UID: CHE123456789 TVA", "CHE-123.456.789"},
		{"VAT NL123456789B01", "NL123456789B01"},
	}
	for _, c := range cases {
		got := FindVATIDs(c.text)
		if len(got) != 1 || got[0].Value != c.want {
			t.Errorf("%q: %v", c.text, got)
		}
	}
	if got := FindVATIDs("Konto DE89370400440532013000 und XDE123456789 und DE1234567890"); len(got) != 0 {
		t.Errorf("false positives: %v", got)
	}
}

func TestAmountLeadingZeros(t *testing.T) {
	if got := FindAmount("DE 000007.00"); len(got) != 0 {
		t.Errorf("zero padded code is no amount: %v", got)
	}
}
