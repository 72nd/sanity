package analysis

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"nbsp", "Gesamt  49,95 EUR", "Gesamt 49,95 EUR"},
		{"ocr O in date", "Datum: 12.O3.2O25", "Datum: 12.03.2025"},
		{"ocr l in amount", "Summe 1l9,50", "Summe 119,50"},
		{"words stay", "Olli Illner lol 2025", "Olli Illner lol 2025"},
		{"liters stay", "0,5l Flasche", "0,5l Flasche"},
		{"hyphenation", "Rechnungs-\nnummer 12345", "Rechnungsnummer 12345"},
		{"hyphen before capital stays", "Mehrwert-\nSteuer", "Mehrwert-\nSteuer"},
		{"collapse", "a   b\t\tc  \n\n\n\n d", "a b c\n\nd"},
		{"crlf", "a\r\nb", "a\nb"},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("%s: Normalize(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestTextHash(t *testing.T) {
	a := TextHash(Normalize("Rechnung  Nr. 1\n\nBetrag 5"))
	b := TextHash(Normalize("rechnung nr. 1 betrag 5"))
	if a != b || len(a) != 64 {
		t.Errorf("hash should ignore case and whitespace: %s %s", a, b)
	}
	if a == TextHash("anders") {
		t.Error("different text must hash differently")
	}
}
