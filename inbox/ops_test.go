package inbox

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// writeFile creates path (and its parents) with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// readFile returns the content of path and fails the test if it cannot be read.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// assertGone fails the test if path still exists (symlinks included).
func assertGone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected %s to be gone, Lstat err = %v", path, err)
	}
}

// assertDir fails the test unless path exists and is a directory.
func assertDir(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected directory %s: %v", path, err)
	}
	if !fi.IsDir() {
		t.Fatalf("expected %s to be a directory", path)
	}
}

func TestMoveFileSameVolume(t *testing.T) {
	root := t.TempDir()
	from := filepath.Join(root, "inbox", "scan.pdf")
	to := filepath.Join(root, "filed", "scan.pdf")
	writeFile(t, from, "receipt-bytes")
	writeFile(t, filepath.Join(root, "filed", "keep.txt"), "x") // folder already exists

	m, err := MoveFile(from, to)
	if err != nil {
		t.Fatalf("MoveFile: %v", err)
	}
	if got := readFile(t, to); got != "receipt-bytes" {
		t.Errorf("target content = %q, want %q", got, "receipt-bytes")
	}
	assertGone(t, from)
	if m.From != from || m.To != to {
		t.Errorf("Move From/To = %q/%q, want %q/%q", m.From, m.To, from, to)
	}
	if len(m.CreatedFolders) != 0 {
		t.Errorf("CreatedFolders = %v, want none (folder existed)", m.CreatedFolders)
	}
}

func TestMoveFileCreatesFoldersOutermostFirst(t *testing.T) {
	tests := []struct {
		name     string
		existing []string // relative folders created before the move
		target   []string // relative folders expected in CreatedFolders
	}{
		{
			name:   "all folders missing",
			target: []string{"2024", "2024/03", "2024/03/receipts"},
		},
		{
			name:     "outer folder exists",
			existing: []string{"2024"},
			target:   []string{"2024/03", "2024/03/receipts"},
		},
		{
			name:     "all folders exist",
			existing: []string{"2024/03/receipts"},
			target:   nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, d := range tc.existing {
				if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			from := filepath.Join(root, "in.pdf")
			to := filepath.Join(root, "2024", "03", "receipts", "in.pdf")
			writeFile(t, from, "data")

			m, err := MoveFile(from, to)
			if err != nil {
				t.Fatalf("MoveFile: %v", err)
			}
			var want []string
			for _, d := range tc.target {
				want = append(want, filepath.Join(root, d))
			}
			if !slices.Equal(m.CreatedFolders, want) {
				t.Errorf("CreatedFolders = %v, want %v", m.CreatedFolders, want)
			}
			for _, d := range tc.target {
				assertDir(t, filepath.Join(root, d))
			}
			if got := readFile(t, to); got != "data" {
				t.Errorf("target content = %q, want %q", got, "data")
			}
		})
	}
}

func TestMoveFileRefusesExistingTarget(t *testing.T) {
	tests := []struct {
		name        string
		targetIsDir bool
	}{
		{name: "target is a file"},
		{name: "target is a directory", targetIsDir: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			from := filepath.Join(root, "in", "scan.pdf")
			to := filepath.Join(root, "out", "scan.pdf")
			writeFile(t, from, "new")
			if tc.targetIsDir {
				if err := os.MkdirAll(to, 0o755); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(to, "inner.txt"), "keep")
			} else {
				writeFile(t, to, "old")
			}

			if _, err := MoveFile(from, to); err == nil {
				t.Fatal("MoveFile: expected error for existing target")
			}
			if tc.targetIsDir {
				if got := readFile(t, filepath.Join(to, "inner.txt")); got != "keep" {
					t.Errorf("target directory was touched, inner = %q", got)
				}
			} else if got := readFile(t, to); got != "old" {
				t.Errorf("existing target was overwritten: %q", got)
			}
			if got := readFile(t, from); got != "new" {
				t.Errorf("source was touched: %q", got)
			}
		})
	}
}

func TestUndoRestoresFile(t *testing.T) {
	root := t.TempDir()
	from := filepath.Join(root, "inbox", "scan.pdf")
	to := filepath.Join(root, "filed", "2024", "scan.pdf")
	writeFile(t, from, "receipt")

	m, err := MoveFile(from, to)
	if err != nil {
		t.Fatalf("MoveFile: %v", err)
	}
	if err := Undo(m); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := readFile(t, from); got != "receipt" {
		t.Errorf("restored content = %q, want %q", got, "receipt")
	}
	assertGone(t, to)
	assertDir(t, filepath.Join(root, "inbox"))
}

func TestUndoRecreatesRemovedSourceFolder(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "inbox", "sub")
	from := filepath.Join(srcDir, "scan.pdf")
	to := filepath.Join(root, "filed", "scan.pdf")
	writeFile(t, from, "receipt")

	m, err := MoveFile(from, to)
	if err != nil {
		t.Fatalf("MoveFile: %v", err)
	}
	// The user (or a cleanup) removes the now empty source folder.
	if err := os.Remove(srcDir); err != nil {
		t.Fatalf("remove source folder: %v", err)
	}

	if err := Undo(m); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := readFile(t, from); got != "receipt" {
		t.Errorf("restored content = %q, want %q", got, "receipt")
	}
}

func TestUndoFolderCleanup(t *testing.T) {
	// The move creates a/b/c. Extra files placed at a given level keep that
	// folder and all its ancestors (created by the move); folders below it
	// are removed.
	tests := []struct {
		name      string
		extraFile string   // relative path of an unrelated file, "" for none
		gone      []string // created folders expected to be removed
		kept      []string // created folders expected to remain
	}{
		{
			name: "no other files: all created folders removed",
			gone: []string{"a", "a/b", "a/b/c"},
		},
		{
			name:      "other file in innermost folder: all kept",
			extraFile: "a/b/c/other.pdf",
			kept:      []string{"a", "a/b", "a/b/c"},
		},
		{
			name:      "other file in middle folder: innermost removed",
			extraFile: "a/b/other.pdf",
			gone:      []string{"a/b/c"},
			kept:      []string{"a", "a/b"},
		},
		{
			name:      "other file in outer folder: two innermost removed",
			extraFile: "a/other.pdf",
			gone:      []string{"a/b", "a/b/c"},
			kept:      []string{"a"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			from := filepath.Join(root, "src.pdf")
			to := filepath.Join(root, "a", "b", "c", "x.pdf")
			writeFile(t, from, "data")
			if tc.extraFile != "" {
				writeFile(t, filepath.Join(root, tc.extraFile), "other")
			}

			m, err := MoveFile(from, to)
			if err != nil {
				t.Fatalf("MoveFile: %v", err)
			}
			if err := Undo(m); err != nil {
				t.Fatalf("Undo: %v", err)
			}
			for _, d := range tc.gone {
				assertGone(t, filepath.Join(root, d))
			}
			for _, d := range tc.kept {
				assertDir(t, filepath.Join(root, d))
			}
			if got := readFile(t, from); got != "data" {
				t.Errorf("restored content = %q, want %q", got, "data")
			}
			if tc.extraFile != "" {
				if got := readFile(t, filepath.Join(root, tc.extraFile)); got != "other" {
					t.Errorf("unrelated file changed: %q", got)
				}
			}
		})
	}
}

func TestUndoRefusesWhenOriginalPathTaken(t *testing.T) {
	root := t.TempDir()
	from := filepath.Join(root, "inbox", "scan.pdf")
	to := filepath.Join(root, "filed", "scan.pdf")
	writeFile(t, from, "original")

	m, err := MoveFile(from, to)
	if err != nil {
		t.Fatalf("MoveFile: %v", err)
	}
	writeFile(t, from, "squatter")

	if err := Undo(m); err == nil {
		t.Fatal("Undo: expected error when original path is taken")
	}
	if got := readFile(t, from); got != "squatter" {
		t.Errorf("original path content = %q, want %q", got, "squatter")
	}
	if got := readFile(t, to); got != "original" {
		t.Errorf("filed file content = %q, want %q", got, "original")
	}
}

func TestFileHash(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "abc",
			content: "abc",
			want:    "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name:    "empty",
			content: "",
			want:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(root, tc.name+".bin")
			writeFile(t, p, tc.content)
			got, err := FileHash(p)
			if err != nil {
				t.Fatalf("FileHash: %v", err)
			}
			if got != tc.want {
				t.Errorf("FileHash = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestFileHashMissingFile(t *testing.T) {
	if _, err := FileHash(filepath.Join(t.TempDir(), "nope.pdf")); err == nil {
		t.Fatal("FileHash: expected error for missing file")
	}
}

func TestMoveFileFailedLeavesNoStrayFolders(t *testing.T) {
	tests := []struct {
		name string
		from func(root string) string // source path; the file may or may not exist
	}{
		{
			name: "source does not exist",
			from: func(root string) string { return filepath.Join(root, "missing.pdf") },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			from := tc.from(root)
			to := filepath.Join(root, "new1", "new2", "x.pdf")

			if _, err := MoveFile(from, to); err == nil {
				t.Fatal("MoveFile: expected error")
			}
			assertGone(t, filepath.Join(root, "new1"))
			assertGone(t, to)
		})
	}
}

// Cross-device moves (EXDEV) take the copyVerified path. They cannot be
// triggered reliably in a t.TempDir (all temp dirs share one volume), so they
// are not covered here.

// KNOWN BUG in ops.go Undo: it recreates the original folder (mkdirAll) before
// moveRaw runs, so a failed Undo leaves that folder behind. Remove the Skip
// once Undo checks the filed file first or removes the new folders on error.
func TestUndoFailedLeavesNoStrayFolders(t *testing.T) {
	root := t.TempDir()
	from := filepath.Join(root, "inbox", "sub", "scan.pdf")
	m := Move{From: from, To: filepath.Join(root, "filed", "scan.pdf")} // filed file does not exist

	if err := Undo(m); err == nil {
		t.Fatal("Undo: expected error when the filed file is missing")
	}
	assertGone(t, filepath.Join(root, "inbox"))
}
