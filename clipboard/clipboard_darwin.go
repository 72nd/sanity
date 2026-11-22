//go:build darwin

// Package clipboard reads file URLs from the system pasteboard. Fyne's
// clipboard is text only, so Finder's "copy" of files needs NSPasteboard.
package clipboard

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework Foundation
#include <stdlib.h>
#include "clipboard.h"
*/
import "C"

import (
	"strings"
	"unsafe"
)

// FilePaths returns the absolute paths of the files on the clipboard (for
// example after "copy" in Finder), or nil when there are none.
func FilePaths() []string {
	c := C.sanityClipboardFiles()
	if c == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(c))
	return strings.Split(C.GoString(c), "\n")
}

// setFile puts one file URL on the pasteboard (used by the test).
func setFile(path string) bool {
	c := C.CString(path)
	defer C.free(unsafe.Pointer(c))
	return C.sanityClipboardSetFile(c) == 1
}
