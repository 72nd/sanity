//go:build darwin

package inbox

import (
	"fmt"
	"os/exec"
)

// Reveal shows the file in Finder.
func Reveal(path string) error { return exec.Command("open", "-R", path).Run() }

// Trash moves the file to the macOS trash (so it can be recovered).
func Trash(path string) error {
	script := `on run argv
tell application "Finder" to delete (POSIX file (item 1 of argv) as alias)
end run`
	if out, err := exec.Command("osascript", "-e", script, path).CombinedOutput(); err != nil {
		return fmt.Errorf("inbox: move to trash: %w: %s", err, out)
	}
	return nil
}

// Open opens the file in the default application (the PDF viewer).
func Open(path string) error { return exec.Command("open", path).Run() }
