package analysis

import "testing"

func TestDetectLanguage(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Rechnung Nr. 123\nDatum: 12.03.2025\nVielen Dank für Ihre Bestellung. Der Betrag ist fällig.", "de"},
		{"Invoice #123\nDate: March 12, 2025\nThank you for your order. The amount is due on receipt.", "en"},
		{"", "en"},
		{"12345 67890", "en"},
	}
	for _, c := range cases {
		if got := DetectLanguage(c.in); got != c.want {
			t.Errorf("DetectLanguage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
