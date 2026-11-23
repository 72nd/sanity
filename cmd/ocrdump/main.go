// Command ocrdump shows what the PDF layer and Vision read from a PDF page, for
// tuning the OCR settings on real documents. Vision does not work inside the
// agent sandbox, so run it yourself:
//
//	go run ./cmd/ocrdump -file testdata/langlauf.pdf
//	go run ./cmd/ocrdump -file testdata/cursor.pdf -min 0.003 -scale 4 -text
//	go run ./cmd/ocrdump -file testdata/apple-store-software.pdf -ref testdata/src/apple_2026-01-24.pdf -sweep
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/72nd/sanity/pdf"
)

func main() {
	file := flag.String("file", "", "PDF file")
	page := flag.Int("page", 0, "page (0-based)")
	scale := flag.Float64("scale", pdf.DefaultOCR.Scale, "render scale for OCR (1 = 72 dpi)")
	minH := flag.Float64("min", pdf.DefaultOCR.MinTextHeight, "Vision minimum text height relative to the image height (0 = Vision default)")
	showText := flag.Bool("text", false, "print the recognized text")
	sweep := flag.Bool("sweep", false, "try several minimum text heights and scales")
	ref := flag.String("ref", "", "clean original of the same page: prints how many of its words the OCR found (word recall) and sweeps more scales")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "usage: ocrdump -file x.pdf [-page 0] [-scale 1] [-min 0.005] [-text] [-sweep]")
		os.Exit(2)
	}
	e := pdf.NewEngine()
	meta, err := e.Meta(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "meta:", err)
		os.Exit(1)
	}
	fmt.Printf("%s: %d pages, producer %q, creator %q\n", *file, meta.Pages, meta.Producer, meta.Creator)
	fmt.Printf("OCR available: %v\n", e.OCRAvailable())

	pt, err := e.Text(*file, *page)
	if err != nil {
		fmt.Fprintln(os.Stderr, "text:", err)
		os.Exit(1)
	}
	fmt.Printf("engine.Text: source=%s scan=%v chars=%d\n", pt.Source, pt.Scan, len([]rune(strings.TrimSpace(pt.Text))))

	var refWords map[string]bool
	if *ref != "" {
		rt, err := e.Text(*ref, *page)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ref:", err)
			os.Exit(1)
		}
		refWords = words(rt.Text)
		fmt.Printf("reference: %d distinct words\n", len(refWords))
	}
	run := func(s, m float64) string {
		start := time.Now()
		out, err := pdf.OCRPage(*file, *page, pdf.OCROptions{Scale: s, MinTextHeight: m})
		fmt.Printf("  OCR scale=%.1f min=%.4f -> %5d chars, %v", s, m, len([]rune(strings.TrimSpace(out))), time.Since(start).Round(time.Millisecond))
		if refWords != nil {
			fmt.Printf("  word recall %3.0f%%", recall(refWords, words(out))*100)
		}
		if err != nil {
			fmt.Printf("  error: %v", err)
		}
		fmt.Println()
		return out
	}
	if *sweep && refWords != nil {
		for _, s := range []float64{0.6, 0.8, 1, 1.2, 1.4, 1.6, 2, 2.5} {
			run(s, *minH)
		}
		return
	}
	if *sweep {
		for _, s := range []float64{0.6, 0.75, 1, 1.25} {
			for _, m := range []float64{0, 0.01, 0.005, 0.003, 0.0015} {
				run(s, m)
			}
		}
		return
	}
	out := run(*scale, *minH)
	if *showText {
		fmt.Println("---- recognized text ----")
		fmt.Println(out)
		fmt.Println("---- text of engine.Text ----")
		fmt.Println(pt.Text)
	}
}

// words returns the distinct lower-case words of at least 3 letters or digits.
func words(text string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len([]rune(w)) >= 3 {
			out[w] = true
		}
	}
	return out
}

// recall is the share of the reference words that the OCR found.
func recall(ref, got map[string]bool) float64 {
	if len(ref) == 0 {
		return 0
	}
	n := 0
	for w := range ref {
		if got[w] {
			n++
		}
	}
	return float64(n) / float64(len(ref))
}
