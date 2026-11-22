// Command ocrdump shows what the PDF layer and Vision read from a PDF page, for
// tuning the OCR settings on real documents. Vision does not work inside the
// agent sandbox, so run it yourself:
//
//	go run ./cmd/ocrdump -file testdata/langlauf.pdf
//	go run ./cmd/ocrdump -file testdata/cursor.pdf -min 0.003 -scale 4 -text
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/72nd/sanity/pdf"
)

func main() {
	file := flag.String("file", "", "PDF file")
	page := flag.Int("page", 0, "page (0-based)")
	scale := flag.Float64("scale", pdf.DefaultOCR.Scale, "render scale for OCR (1 = 72 dpi)")
	minH := flag.Float64("min", pdf.DefaultOCR.MinTextHeight, "Vision minimum text height relative to the image height (0 = Vision default)")
	showText := flag.Bool("text", false, "print the recognized text")
	sweep := flag.Bool("sweep", false, "try several minimum text heights and scales")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "usage: ocrdump -file x.pdf [-page 0] [-scale 3.5] [-min 0.005] [-text] [-sweep]")
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

	run := func(s, m float64) string {
		start := time.Now()
		out, err := pdf.OCRPage(*file, *page, pdf.OCROptions{Scale: s, MinTextHeight: m})
		fmt.Printf("  OCR scale=%.1f min=%.4f -> %5d chars, %v", s, m, len([]rune(strings.TrimSpace(out))), time.Since(start).Round(time.Millisecond))
		if err != nil {
			fmt.Printf("  error: %v", err)
		}
		fmt.Println()
		return out
	}
	if *sweep {
		for _, s := range []float64{2.5, 3.5, 5} {
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
