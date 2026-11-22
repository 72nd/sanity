package inbox

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

// TestLearnCorpusReport learns every corpus PDF as its own type and reports
// how a second pass ranks them. It only logs; it fails on nothing but I/O.
func TestLearnCorpusReport(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "testdata", "*.pdf"))
	if len(files) == 0 {
		t.Skip("no corpus")
	}
	eng := pdf.NewEngine()
	l := NewLearner(t.TempDir(), store.NewFake(), eng)
	texts := map[string]string{}
	ctx := context.Background()
	for _, f := range files {
		ex, err := ExtractText(ctx, eng, f)
		if err != nil || strings.TrimSpace(ex.Text) == "" {
			t.Logf("skip %s (err=%v, empty text, ocr=%v)", filepath.Base(f), err, eng.OCRAvailable())
			continue
		}
		id := filepath.Base(f)
		texts[id] = ex.Text
		_ = l.Learn(id, id, ex.Text)
	}
	own := 0
	for id, text := range texts {
		sc := l.Index().Score(text)
		best, bs := "", -1.0
		for k, v := range sc {
			if v > bs {
				best, bs = k, v
			}
		}
		if best == id {
			own++
		}
		t.Logf("%-28s own=%.2f best=%s (%.2f)", id, sc[id], best, bs)
	}
	t.Logf("%d of %d rank their own type first", own, len(texts))
}
