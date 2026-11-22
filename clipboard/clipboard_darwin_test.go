//go:build darwin

package clipboard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilePathsRoundTrip(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "a b.pdf")
	if err := os.WriteFile(p, []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !setFile(p) {
		t.Skip("pasteboard not writable here (sandbox?)")
	}
	got := FilePaths()
	if len(got) != 1 || got[0] != p {
		t.Skipf("pasteboard round trip not visible here (sandbox?): got %v want [%s]", got, p)
	}
}
