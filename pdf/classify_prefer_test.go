package pdf

import (
	"strings"
	"testing"
)

func TestPreferOCR(t *testing.T) {
	good := strings.Repeat("Rechnung Nr. 4711 vom 12.03.2025 Betrag 49,95 EUR ", 6)
	garbage := strings.Repeat("|| ~~ l1 .. ;; ", 30)
	short := "Apple"
	cases := []struct {
		name       string
		layer, ocr string
		want       bool
	}{
		{"empty OCR never wins", good, "", false},
		{"no layer: OCR wins", "", short, true},
		{"good layer, tiny OCR (missed the small print)", good, short, false},
		{"good layer, comparable OCR", good, good[:len(good)*4/5], true},
		{"garbage layer, OCR of any decent size", garbage, good[:len(garbage)/2+1], true},
		{"garbage layer, tiny OCR", garbage, short, false},
	}
	for _, c := range cases {
		if got := PreferOCR(c.layer, c.ocr); got != c.want {
			t.Errorf("%s: PreferOCR = %v, want %v (low quality layer: %v)", c.name, got, c.want, LowQuality(c.layer))
		}
	}
}
