package inbox

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

func acmeText(n string) string {
	return "Acme Hosting AG\nPostfach 12\n8000 Zuerich\nRechnung " + n + "\nServer Miete Monat\nTotal CHF 120.00\nwww.acmehosting.example"
}

func acmeScore(t *testing.T, l *Learner, text string) float64 {
	t.Helper()
	return l.Index().Score(analysis.Normalize(text))["acme"]
}

func TestLearnCapsUntilThreeFiled(t *testing.T) {
	l := NewLearner(t.TempDir(), store.NewFake(), pdf.NewFake())
	if err := l.Learn("acme", "sha1", analysis.Normalize(acmeText("1001"))); err != nil {
		t.Fatal(err)
	}
	s := acmeScore(t, l, acmeText("2002"))
	if s <= 0 {
		t.Fatalf("learned score %v, want > 0", s)
	}
	pr, _ := store.NewFake().Project(context.Background())
	types := append(pr.Types, domain.DocumentType{ID: "acme", Name: "Acme", UseLearned: true})
	sug := analysis.Classify(analysis.Doc{Text: analysis.Normalize(acmeText("2002")), FileName: "x.pdf"}, types, l.Index())
	if len(sug) == 0 || sug[0].TypeID != "acme" || sug[0].LearnedScore > 0.7+1e-9 || sug[0].Confident {
		t.Fatalf("after 1 filed: %+v", sug)
	}
	for i, sha := range []string{"sha2", "sha3"} {
		_ = l.Learn("acme", sha, analysis.Normalize(acmeText("300"+string(rune('0'+i)))))
	}
	if l.Index().Filed("acme") != 3 {
		t.Fatalf("filed %d", l.Index().Filed("acme"))
	}
	sug = analysis.Classify(analysis.Doc{Text: analysis.Normalize(acmeText("4004")), FileName: "x.pdf"}, types, l.Index())
	t.Logf("after 3 filed: %+v", sug[0])
	if sug[0].TypeID != "acme" || !sug[0].Confident || sug[0].LearnedScore <= 0.7 {
		t.Errorf("after 3 filed: %+v", sug[0])
	}
}

func TestCacheRoundTripAndForget(t *testing.T) {
	root := t.TempDir()
	st := store.NewFake()
	l := NewLearner(root, st, pdf.NewFake())
	text := analysis.Normalize(acmeText("1"))
	if err := l.Learn("acme", "sha1", text); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(CachePath(root)); err != nil {
		t.Fatal(err)
	}
	if matches, _ := filepath.Glob(filepath.Join(root, "office", ".sanity", "*.tmp")); len(matches) != 0 {
		t.Errorf("temp files left: %v", matches)
	}
	c := OpenCache(CachePath(root))
	e, ok := c.Get("sha1")
	if !ok || e.Text != text || e.Source != pdf.SourceLayer || e.TextHash == "" || len(e.Fingerprint) == 0 {
		t.Fatalf("round trip: %+v %v", e, ok)
	}
	if err := l.Forget("sha1"); err != nil {
		t.Fatal(err)
	}
	if l.Index().Filed("acme") != 0 || acmeScore(t, l, acmeText("2")) != 0 {
		t.Error("forget left the document in the index")
	}
	if OpenCache(CachePath(root)).Len() != 0 {
		t.Error("forget left the document in the cache")
	}
}

func TestRebuildFromCacheAndPDF(t *testing.T) {
	root := t.TempDir()
	st := store.NewFake()
	ctx := context.Background()
	pr, _ := st.Project(ctx)
	for _, e := range pr.Library {
		_ = st.RemoveEntry(ctx, e.ID)
	}
	_ = writePDF(t, filepath.Join(root, "receipts"), "a.pdf")
	_ = writePDF(t, filepath.Join(root, "receipts"), "b.pdf")
	add := func(id, path, sha string) {
		if err := st.AddEntry(ctx, domain.LibraryEntry{ID: id, TypeID: "acme", Path: path, SHA256: sha}); err != nil {
			t.Fatal(err)
		}
	}
	add("1", "receipts/a.pdf", "shaA")
	add("2", "receipts/b.pdf", "shaB")
	add("3", "receipts/missing.pdf", "shaC") // skipped without error

	// shaA comes from the cache (the fake engine would return other text).
	c := OpenCache(CachePath(root))
	c.Put("shaA", CacheEntry{Text: analysis.Normalize(acmeText("1")), Source: pdf.SourceLayer})
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	l := NewLearner(root, st, pdf.NewFake(acmeText("2")))
	// the fake returns the same text for every path, including missing.pdf
	if err := l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	if got := l.Index().Filed("acme"); got != 3 {
		t.Errorf("filed %d, want 3", got)
	}
	e, ok := OpenCache(CachePath(root)).Get("shaB")
	if !ok || e.Text == "" || e.Fingerprint == nil {
		t.Errorf("shaB not cached after rebuild: %+v", e)
	}
	if acmeScore(t, l, acmeText("9")) <= 0 {
		t.Error("rebuilt index scores 0")
	}
}

func TestLearnerConcurrent(t *testing.T) {
	l := NewLearner(t.TempDir(), store.NewFake(), pdf.NewFake(acmeText("x")))
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			sha := string(rune('a' + i))
			_ = l.Learn("acme", sha, analysis.Normalize(acmeText(sha)))
			if i%3 == 0 {
				_ = l.Forget(sha)
			}
		}()
		go func() {
			defer wg.Done()
			for range 20 {
				l.Index().Score(analysis.Normalize(acmeText("q")))
				l.Index().Filed("acme")
			}
		}()
	}
	wg.Add(1)
	go func() { defer wg.Done(); _ = l.Rebuild(context.Background()) }()
	wg.Wait()
}
