//go:build !darwin

package inbox

import "errors"

// Reveal is only implemented on macOS.
func Reveal(string) error { return errors.New("inbox: reveal is not supported on this platform") }

// Trash is only implemented on macOS.
func Trash(string) error { return errors.New("inbox: trash is not supported on this platform") }

// Open is only implemented on macOS.
func Open(string) error { return errors.New("inbox: open is not supported on this platform") }
