package inbox

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQueueAddFolderRecursivePDFOnly(t *testing.T) {
	dir := t.TempDir()
	writePDF(t, dir, "a.pdf")
	writePDF(t, filepath.Join(dir, "sub"), "b.PDF")
	writePDF(t, filepath.Join(dir, ".hidden"), "c.pdf")
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	q := NewQueue(nil)
	if n := q.Add(dir); n != 2 {
		t.Fatalf("added %d", n)
	}
	if n := q.Add(dir); n != 0 {
		t.Errorf("duplicate import added %d", n)
	}
	if q.Len() != 2 {
		t.Errorf("len %d", q.Len())
	}
}

func TestQueueRemoveStatusPrune(t *testing.T) {
	dir := t.TempDir()
	a := writePDF(t, dir, "a.pdf")
	b := writePDF(t, dir, "b.pdf")
	changes := 0
	q := NewQueue(func() { changes++ })
	q.Add(a, b)
	q.SetStatus(a, StatusReady, "")
	if it, _ := q.Get(a); it.Status != StatusReady {
		t.Error("status not set")
	}
	os.Remove(b)
	if q.Prune() != 1 || q.Len() != 1 {
		t.Error("prune failed")
	}
	if !q.Remove(a) || q.Remove(a) {
		t.Error("remove semantics")
	}
	if changes < 3 {
		t.Errorf("changes %d", changes)
	}
}

func TestWatchPicksUpNewPDF(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "inbox")
	q := NewQueue(nil)
	stop, err := Watch(q, dir, 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	writePDF(t, dir, "new.pdf")
	deadline := time.Now().Add(3 * time.Second)
	for q.Len() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if q.Len() != 1 {
		t.Fatalf("watcher did not import, len %d", q.Len())
	}
}
