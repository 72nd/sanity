package inbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestImportMovesIntoInboxAndKeepsOrigin(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "office", "inbox")
	src := writePDF(t, filepath.Join(root, "elsewhere"), "a.pdf")
	q, err := OpenQueue(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := q.Import(src)
	if err != nil || len(ids) != 1 {
		t.Fatalf("ids %v err %v", ids, err)
	}
	if exists(src) || !exists(filepath.Join(dir, "a.pdf")) {
		t.Fatal("file was not moved into the inbox")
	}
	it, _ := q.Get(ids[0])
	if it.Origin != src {
		t.Errorf("origin %q, want %q", it.Origin, src)
	}
	if !exists(filepath.Join(dir, OriginsFile)) {
		t.Error("origins not saved")
	}
}

func TestImportNameConflictGetsSuffix(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "inbox")
	writePDF(t, dir, "a.pdf")
	s1 := writePDF(t, filepath.Join(root, "x"), "a.pdf")
	s2 := writePDF(t, filepath.Join(root, "y"), "a.pdf")
	q, _ := OpenQueue(dir, nil)
	ids, err := q.Import(s1, s2)
	if err != nil || len(ids) != 2 {
		t.Fatalf("%v %v", ids, err)
	}
	for _, want := range []string{"a_2.pdf", "a_3.pdf"} {
		if !exists(filepath.Join(dir, want)) {
			t.Errorf("%s missing", want)
		}
	}
	if q.Len() != 3 {
		t.Errorf("len %d", q.Len())
	}
	if o, _ := q.Origin(filepath.Join(dir, "a_3.pdf")); o != s2 {
		t.Errorf("origin of a_3 = %q", o)
	}
}

func TestImportFileAlreadyInInboxHasNoOrigin(t *testing.T) {
	dir := t.TempDir()
	p := writePDF(t, dir, "a.pdf")
	q, _ := OpenQueue(dir, nil)
	ids, _ := q.Import(p)
	if len(ids) != 0 {
		t.Errorf("known file imported again: %v", ids)
	}
	if _, ok := q.Origin(p); ok {
		t.Error("origin for a file that lay in the inbox")
	}
	if _, err := q.PutBack(p); !errors.Is(err, ErrNoOrigin) {
		t.Errorf("err %v", err)
	}
	if !exists(p) || q.Len() != 1 {
		t.Error("put back without origin changed something")
	}
}

func TestImportFolderRecursive(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "inbox")
	src := filepath.Join(root, "drop")
	writePDF(t, src, "a.pdf")
	writePDF(t, filepath.Join(src, "sub"), "b.pdf")
	q, _ := OpenQueue(dir, nil)
	ids, err := q.Import(src)
	if err != nil || len(ids) != 2 {
		t.Fatalf("%v %v", ids, err)
	}
	if o, _ := q.Origin(filepath.Join(dir, "b.pdf")); o != filepath.Join(src, "sub", "b.pdf") {
		t.Errorf("origin %q", o)
	}
}

func TestQueueOnFolderSurvivesRestart(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "office", "inbox")
	src := writePDF(t, filepath.Join(root, "proj", "docs"), "r.pdf")
	writePDF(t, dir, "plain.pdf")
	q1, _ := OpenQueue(dir, nil)
	if _, err := q1.Import(src); err != nil {
		t.Fatal(err)
	}

	q2, err := OpenQueue(dir, nil) // restart: new queue, same folder
	if err != nil {
		t.Fatal(err)
	}
	if q2.Len() != 2 {
		t.Fatalf("len %d after restart", q2.Len())
	}
	if o, ok := q2.Origin(filepath.Join(dir, "r.pdf")); !ok || o != src {
		t.Errorf("origin after restart %q %v", o, ok)
	}
	if _, ok := q2.Origin(filepath.Join(dir, "plain.pdf")); ok {
		t.Error("plain.pdf must have no origin")
	}
	if _, err := q2.PutBack(filepath.Join(dir, "r.pdf")); err != nil {
		t.Fatal(err)
	}
	if !exists(src) || q2.Len() != 1 {
		t.Error("not put back after restart")
	}
	if exists(filepath.Join(dir, OriginsFile)) {
		t.Error("origins file should be gone with the last record")
	}
}

func TestPutBackOccupiedOriginKeepsFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "inbox")
	src := writePDF(t, filepath.Join(root, "x"), "a.pdf")
	q, _ := OpenQueue(dir, nil)
	ids, _ := q.Import(src)
	writePDF(t, filepath.Join(root, "x"), "a.pdf") // someone put a new file there
	if _, err := q.PutBack(ids[0]); err == nil {
		t.Fatal("overwrote an existing file")
	}
	if q.Len() != 1 || !exists(ids[0]) {
		t.Error("document must stay in the inbox")
	}
	if _, ok := q.Origin(ids[0]); !ok {
		t.Error("origin lost")
	}
}

func TestOriginsDroppedWhenFileIsGone(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "inbox")
	src := writePDF(t, filepath.Join(root, "x"), "a.pdf")
	q, _ := OpenQueue(dir, nil)
	ids, _ := q.Import(src)
	os.Remove(ids[0]) // filed or deleted outside
	q.Prune()
	if exists(filepath.Join(dir, OriginsFile)) {
		t.Error("stale origin kept")
	}
}

func TestAdoptRejectsOriginOutsideRoot(t *testing.T) {
	f := NewFiler(t.TempDir(), nil)
	if _, err := f.Adopt(context.Background(), AdoptRequest{Path: "/x/a.pdf", Origin: filepath.Join(t.TempDir(), "a.pdf")}); !errors.Is(err, ErrOutsideProject) {
		t.Errorf("err %v", err)
	}
}
