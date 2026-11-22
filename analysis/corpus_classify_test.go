package analysis_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

// TestCorpusClassifyReport logs the suggestions for the sample PDFs. No assertions.
func TestCorpusClassifyReport(t *testing.T) {
	files, _ := filepath.Glob("../testdata/*.pdf")
	if len(files) == 0 {
		t.Skip("no sample pdfs")
	}
	types, err := store.NewFake().Types(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	eng := pdf.NewEngine()
	for _, f := range files {
		meta, err := eng.Meta(f)
		if err != nil {
			t.Logf("%s: meta: %v", filepath.Base(f), err)
			continue
		}
		var sb strings.Builder
		for p := 0; p < meta.Pages && p < 3; p++ {
			pt, err := eng.Text(f, p)
			if err != nil {
				break
			}
			sb.WriteString(pt.Text + "\n")
		}
		text := sb.String()
		if strings.TrimSpace(text) == "" {
			t.Logf("%s: no text, skipped", filepath.Base(f))
			continue
		}
		sug := analysis.Classify(analysis.Doc{Text: text, FileName: filepath.Base(f)}, types, nil)
		if len(sug) == 0 {
			t.Logf("%s: no suggestion", filepath.Base(f))
		}
		for _, s := range sug {
			t.Logf("%s: %-18s score %.2f rules %.2f confident %v", filepath.Base(f), s.TypeID, s.Score, s.RuleScore, s.Confident)
		}
		_ = os.Stdout
	}
}
