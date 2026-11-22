// Package inbox moves documents: importing, filing (always a move) and undo.
// It imports no UI.
package inbox

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Move records one performed move so it can be undone.
type Move struct {
	From, To       string   // absolute paths
	CreatedFolders []string // absolute, outermost first, created by this move
}

// FileHash returns the hex SHA-256 of a file.
func FileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MoveFile moves from to to, creating folders. It never overwrites: an existing
// target is an error. Across volumes it copies, syncs, compares hashes and
// only then deletes the source.
func MoveFile(from, to string) (Move, error) {
	if _, err := os.Lstat(to); err == nil {
		return Move{}, fmt.Errorf("inbox: target exists: %s", to)
	}
	created, err := mkdirAll(filepath.Dir(to))
	if err != nil {
		return Move{}, err
	}
	m := Move{From: from, To: to, CreatedFolders: created}
	if err := os.Rename(from, to); err == nil {
		return m, nil
	} else if !isCrossDevice(err) {
		removeFolders(created)
		return Move{}, err
	}
	if err := copyVerified(from, to); err != nil {
		os.Remove(to)
		removeFolders(created)
		return Move{}, err
	}
	if err := os.Remove(from); err != nil {
		return Move{}, err
	}
	return m, nil
}

// Undo moves the file back to its original path (recreating the folder) and
// removes folders the move created if they are empty now.
func Undo(m Move) error {
	if _, err := os.Lstat(m.From); err == nil {
		return fmt.Errorf("inbox: original path is taken: %s", m.From)
	}
	if _, err := os.Lstat(m.To); err != nil {
		return err
	}
	made, err := mkdirAll(filepath.Dir(m.From))
	if err != nil {
		return err
	}
	if err := moveRaw(m.To, m.From); err != nil {
		removeFolders(made)
		return err
	}
	removeFolders(m.CreatedFolders)
	return nil
}

func moveRaw(from, to string) error {
	if err := os.Rename(from, to); err == nil {
		return nil
	} else if !isCrossDevice(err) {
		return err
	}
	if err := copyVerified(from, to); err != nil {
		os.Remove(to)
		return err
	}
	return os.Remove(from)
}

// mkdirAll creates dir and returns the folders it had to create, outermost first.
func mkdirAll(dir string) ([]string, error) {
	var missing []string
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(d); err == nil {
			break
		}
		missing = append([]string{d}, missing...)
		if filepath.Dir(d) == d {
			break
		}
	}
	return missing, os.MkdirAll(dir, 0o755)
}

// removeFolders removes empty folders, innermost first; non-empty ones stay.
func removeFolders(dirs []string) {
	for i := len(dirs) - 1; i >= 0; i-- {
		if os.Remove(dirs[i]) != nil {
			return
		}
	}
}

func isCrossDevice(err error) bool {
	return errors.Is(err, syscall.EXDEV) || strings.Contains(err.Error(), "cross-device")
}

func copyVerified(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	if err := dst.Sync(); err != nil {
		dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	a, err := FileHash(from)
	if err != nil {
		return err
	}
	b, err := FileHash(to)
	if err != nil {
		return err
	}
	if !bytes.Equal([]byte(a), []byte(b)) {
		return errors.New("inbox: copy verification failed")
	}
	return nil
}
