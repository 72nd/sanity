//go:build darwin

package pdf

import (
	"image"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var corpus = map[string]int{ // file -> expected page count (verified below against Meta)
	"apple-store-software.pdf": 0, "cursor.pdf": 0, "hetzner.pdf": 0, "kontist.pdf": 0,
	"postfinance.pdf": 0, "raiffeisen.pdf": 0, "vodafone_2025-11.pdf": 0,
}

func testPDFs(t *testing.T) []string {
	t.Helper()
	files, _ := filepath.Glob("../testdata/*.pdf")
	if len(files) == 0 {
		t.Skip("no test PDFs")
	}
	return files
}

func TestMetaAndRender(t *testing.T) {
	e := NewEngine()
	for _, f := range testPDFs(t) {
		t.Run(filepath.Base(f), func(t *testing.T) {
			m, err := e.Meta(f)
			if err != nil {
				t.Fatal(err)
			}
			if m.Pages < 1 {
				t.Fatalf("pages = %d", m.Pages)
			}
			t.Logf("pages=%d producer=%q creator=%q created=%v", m.Pages, m.Producer, m.Creator, m.Created)
			start := time.Now()
			img, err := e.Render(f, 0, 1)
			d := time.Since(start)
			if err != nil {
				t.Fatal(err)
			}
			b := img.Bounds()
			if b.Dx() < 400 || b.Dx() > 900 || b.Dy() < 400 || b.Dy() > 1300 {
				t.Fatalf("odd page size %v", b)
			}
			if _, ok := img.(*image.RGBA); !ok {
				t.Fatalf("not RGBA: %T", img)
			}
			img2, err := e.Render(f, 0, 2)
			if err != nil {
				t.Fatal(err)
			}
			if img2.Bounds().Dx() < 2*b.Dx()-2 {
				t.Fatalf("scale 2 gives %v, scale 1 %v", img2.Bounds(), b)
			}
			// not blank (some non-white pixel)
			rgba := img.(*image.RGBA)
			dark := 0
			for i := 0; i < len(rgba.Pix); i += 4 {
				if rgba.Pix[i] < 250 {
					dark++
				}
			}
			if dark == 0 {
				t.Fatal("rendered page is blank")
			}
			t.Logf("render 72dpi page 0: %v", d)
			start = time.Now()
			if _, err := e.Render(f, 0, DefaultOCR.Scale); err != nil {
				t.Fatal(err)
			}
			t.Logf("render at OCR scale %.1f: %v", DefaultOCR.Scale, time.Since(start))
			if _, err := e.Render(f, m.Pages, 1); err == nil {
				t.Fatal("expected error for page out of range")
			}
		})
	}
	if _, err := e.Meta("../testdata/does-not-exist.pdf"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestTextAndScanClassification(t *testing.T) {
	e := NewEngine()
	ocr := e.OCRAvailable()
	t.Logf("OCRAvailable=%v", ocr)
	for _, f := range testPDFs(t) {
		t.Run(filepath.Base(f), func(t *testing.T) {
			m, err := e.Meta(f)
			if err != nil {
				t.Fatal(err)
			}
			for p := 0; p < m.Pages; p++ {
				start := time.Now()
				pt, err := e.Text(f, p)
				d := time.Since(start)
				if err != nil {
					t.Fatal(err)
				}
				n := len(strings.TrimSpace(pt.Text))
				t.Logf("page %d: scan=%v source=%s chars=%d time=%v", p, pt.Scan, pt.Source, n, d)
				if !pt.Scan && (pt.Source != SourceLayer || n < minLayerChars) {
					t.Fatalf("born-digital page without layer text: %+v", pt)
				}
				if pt.Scan && !ocr && pt.Source != SourceLayer {
					t.Fatal("vision source without OCR")
				}
			}
		})
	}
}

func TestBornDigitalLayer(t *testing.T) {
	e := NewEngine()
	found := 0
	for _, f := range testPDFs(t) {
		m, _ := e.Meta(f)
		pt, err := e.Text(f, 0)
		if err != nil {
			t.Fatal(err)
		}
		if pt.Scan {
			continue
		}
		found++
		if pt.Source != SourceLayer || len(pt.Text) < 100 {
			t.Errorf("%s: %+v", f, m)
		}
	}
	if found == 0 {
		t.Fatal("no born-digital PDF in the corpus")
	}
}

func TestOCRScan(t *testing.T) {
	e := NewEngine()
	if !e.OCRAvailable() {
		t.Skip("Vision returns no text here (agent sandbox)")
	}
	n := 0
	for _, f := range testPDFs(t) {
		pt, err := e.Text(f, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !pt.Scan {
			continue
		}
		n++
		if pt.Source != SourceVision {
			t.Errorf("%s: scan page not OCRed (%s)", f, pt.Source)
		}
	}
	if n == 0 {
		t.Skip("no scan in the corpus")
	}
}

func TestDates(t *testing.T) {
	e := NewEngine()
	d := e.Dates("Rechnung vom 12. März 2025, Lieferung 12.03.25")
	if len(d) == 0 {
		t.Fatal("no dates found")
	}
	t.Log(d)
	if got := e.Dates("nothing here"); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

// The page must fill the whole bitmap at every scale: CGPDFPageGetDrawingTransform
// alone only scales down, which left a 72 dpi page centred in a white canvas.
func TestRenderContentScalesWithScale(t *testing.T) {
	e := NewEngine()
	for _, f := range testPDFs(t) {
		t.Run(filepath.Base(f), func(t *testing.T) {
			base, err := e.Render(f, 0, 1)
			if err != nil {
				t.Fatal(err)
			}
			cb := CropWhite(base).Bounds()
			for _, s := range []float64{2, 4, 8} {
				img, err := e.Render(f, 0, s)
				if err != nil {
					t.Fatal(err)
				}
				c := CropWhite(img).Bounds()
				if got, want := float64(c.Dx()), float64(cb.Dx())*s; got < want*0.9 || got > want*1.1 {
					t.Fatalf("scale %v: content %d px wide, want about %.0f (page not scaled up)", s, c.Dx(), want)
				}
				if got, want := float64(c.Min.X), float64(cb.Min.X)*s; got < want*0.9-4 || got > want*1.1+4 {
					t.Fatalf("scale %v: content starts at x=%d, want about %.0f", s, c.Min.X, want)
				}
			}
		})
	}
}
