//go:build !darwin

// Package clipboard reads file URLs from the system pasteboard (macOS only).
package clipboard

// FilePaths returns nil on platforms without a file clipboard helper.
func FilePaths() []string { return nil }
