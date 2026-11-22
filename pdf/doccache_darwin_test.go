//go:build darwin

package pdf

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func uncachedEngine() *engine { return &engine{} }
func cachedEngine() *engine   { return &engine{docs: docCache{max: maxOpenDocs}} }

func TestDocCacheSameResults(t *testing.T) {
	files := testPDFs(t)
	a, b := uncachedEngine(), cachedEngine()
	defer b.Close()
	for _, f := range files {
		for round := 0; round < 2; round++ { // second round is served from the cache
			ma, err := a.Meta(f)
			if err != nil {
				t.Fatal(err)
			}
			mb, err := b.Meta(f)
			if err != nil {
				t.Fatal(err)
			}
			if ma != mb {
				t.Errorf("%s: meta %v != %v", f, ma, mb)
			}
			ta, err := a.Text(f, 0)
			if err != nil {
				t.Fatal(err)
			}
			tb, err := b.Text(f, 0)
			if err != nil {
				t.Fatal(err)
			}
			if ta != tb {
				t.Errorf("%s: text differs", f)
			}
			ia, err := a.Render(f, 0, 1.5)
			if err != nil {
				t.Fatal(err)
			}
			ib, err := b.Render(f, 0, 1.5)
			if err != nil {
				t.Fatal(err)
			}
			if ia.Bounds() != ib.Bounds() || !bytes.Equal(ia.(*image.RGBA).Pix, ib.(*image.RGBA).Pix) {
				t.Errorf("%s: render differs", f)
			}
		}
	}
}

func TestDocCacheOpensOnce(t *testing.T) {
	f := testPDFs(t)[0]
	e := cachedEngine()
	defer e.Close()
	for i := 0; i < 10; i++ {
		if _, err := e.Meta(f); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Text(f, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Render(f, 0, 1); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.docs.opens.Load(); n != 1 {
		t.Fatalf("opened %d times, want 1", n)
	}
}

func TestDocCacheInvalidation(t *testing.T) {
	files := testPDFs(t)
	if len(files) < 2 {
		t.Skip("need two PDFs")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "x.pdf")
	copyFile(t, files[0], p)
	e := cachedEngine()
	defer e.Close()
	m1, err := e.Meta(p)
	if err != nil {
		t.Fatal(err)
	}
	// Replace the content (different size): the next call must see the new file.
	var other string
	for _, f := range files[1:] {
		if si, _ := os.Stat(f); si != nil {
			if so, _ := os.Stat(files[0]); so.Size() != si.Size() {
				other = f
				break
			}
		}
	}
	if other == "" {
		t.Skip("no PDF of a different size")
	}
	copyFile(t, other, p)
	want, err := uncachedEngine().Meta(p)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := e.Meta(p)
	if err != nil {
		t.Fatal(err)
	}
	if m2 != want {
		t.Fatalf("stale meta after the file changed: %v, want %v (was %v)", m2, want, m1)
	}
	if n := e.docs.opens.Load(); n != 2 {
		t.Fatalf("opens = %d, want 2", n)
	}
	// Evict and removal.
	e.Evict(p)
	if _, err := e.Render(p, 0, 1); err != nil {
		t.Fatal(err)
	}
	if n := e.docs.opens.Load(); n != 3 {
		t.Fatalf("opens after Evict = %d, want 3", n)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Meta(p); err == nil {
		t.Fatal("Meta of a removed file succeeded")
	}
	e.docs.mu.Lock()
	n := len(e.docs.entries)
	e.docs.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d entries left after removal", n)
	}
}

func TestDocCacheLRU(t *testing.T) {
	f := testPDFs(t)[0]
	dir := t.TempDir()
	e := cachedEngine()
	defer e.Close()
	for i := 0; i < maxOpenDocs+3; i++ {
		p := filepath.Join(dir, string(rune('a'+i))+".pdf")
		copyFile(t, f, p)
		if _, err := e.Meta(p); err != nil {
			t.Fatal(err)
		}
	}
	e.docs.mu.Lock()
	n := len(e.docs.entries)
	e.docs.mu.Unlock()
	if n != maxOpenDocs {
		t.Fatalf("%d entries, want %d", n, maxOpenDocs)
	}
}

func TestDocCacheConcurrent(t *testing.T) {
	files := testPDFs(t)
	e := cachedEngine()
	defer e.Close()
	var wg sync.WaitGroup
	for g := 0; g < 6; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 6; i++ {
				f := files[(g+i)%len(files)]
				if _, err := e.Render(f, 0, 1); err != nil {
					t.Error(err)
				}
				if _, err := e.Text(f, 0); err != nil {
					t.Error(err)
				}
				if i == 3 {
					e.Evict(f)
				}
			}
		}(g)
	}
	wg.Wait()
}

// TestDocCacheTiming logs the time of N repeated calls without and with the
// cache (run with -v).
func TestDocCacheTiming(t *testing.T) {
	files := testPDFs(t)
	const rounds = 10
	run := func(e *engine) time.Duration {
		start := time.Now()
		for _, f := range files { // one document at a time, as the app works
			for i := 0; i < rounds; i++ {
				e.Meta(f)
				e.Text(f, 0)
				e.Render(f, 0, 1)
			}
		}
		return time.Since(start)
	}
	// the OCR is the same in both runs and would hide the difference
	before := run(uncachedEngine())
	c := cachedEngine()
	after := run(c)
	c.Close()
	calls := rounds * len(files)
	t.Logf("%d x (Meta+Text+Render) on %d PDFs: uncached %v (%v/call), cached %v (%v/call), opens cached=%d",
		calls, len(files), before.Round(time.Millisecond), (before / time.Duration(calls)).Round(time.Microsecond),
		after.Round(time.Millisecond), (after / time.Duration(calls)).Round(time.Microsecond), c.docs.opens.Load())
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
