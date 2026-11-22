package pdf

import (
	"strings"
	"testing"
)

func TestClassifyScan(t *testing.T) {
	good := strings.Repeat("Rechnung Nummer 2025-01 vom 12.03.2025 Gesamtbetrag 119,00 EUR. ", 3)
	garbage := strings.Repeat("~|` ^ \\ { } < > ¦ § ¬ ° ", 8)
	singles := strings.Repeat("a b c d e f g h ", 10)
	tests := []struct {
		name string
		in   PageInfo
		scan bool
		why  ScanReason
	}{
		{"born digital", PageInfo{Text: good, Producer: "Skia/PDF"}, false, ReasonNone},
		{"empty", PageInfo{Text: ""}, true, ReasonNoText},
		{"short", PageInfo{Text: "  Seite 1  \n"}, true, ReasonNoText},
		{"ocrmypdf", PageInfo{Text: good, Producer: "OCRmyPDF 15.0"}, true, ReasonProducer},
		{"tesseract creator", PageInfo{Text: good, Creator: "tesseract 5"}, true, ReasonProducer},
		{"paperless", PageInfo{Text: good, Producer: "Paperless-ngx"}, true, ReasonProducer},
		{"full page image", PageInfo{Text: good, PageW: 595, PageH: 842, ImageW: 2480, ImageH: 3508}, true, ReasonImage},
		{"small logo", PageInfo{Text: good, PageW: 595, PageH: 842, ImageW: 300, ImageH: 100}, false, ReasonNone},
		{"low res image", PageInfo{Text: good, PageW: 595, PageH: 842, ImageW: 400, ImageH: 566}, false, ReasonNone},
		{"wide banner", PageInfo{Text: good, PageW: 595, PageH: 842, ImageW: 2480, ImageH: 600}, false, ReasonNone},
		{"garbage", PageInfo{Text: garbage}, true, ReasonQuality},
		{"single tokens", PageInfo{Text: singles}, true, ReasonQuality},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scan, why := ClassifyScan(tt.in)
			if scan != tt.scan || why != tt.why {
				t.Fatalf("got (%v, %q), want (%v, %q)", scan, why, tt.scan, tt.why)
			}
		})
	}
}

func TestLowQuality(t *testing.T) {
	if LowQuality("short ~~~") {
		t.Fatal("short text must not be judged")
	}
	if LowQuality(strings.Repeat("Müller & Söhne GmbH, Straße 5 – 10115 Berlin. ", 4)) {
		t.Fatal("umlauts and punctuation are fine")
	}
}
