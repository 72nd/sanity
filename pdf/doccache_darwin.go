//go:build darwin

package pdf

/*
#include <stdlib.h>
#include "pdfmac.h"
*/
import "C"

import (
	"image"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

const (
	// maxOpenDocs is how many documents the engine keeps open.
	maxOpenDocs = 4
	// idlePerDoc is how many open handles of one document stay idle (the
	// preview render and the analysis may work on the same file at once).
	idlePerDoc = 2
)

// docCache keeps a few PDFDocuments open (retained in the ObjC handle table of
// pdfmac.m) plus what is cheap to remember about them: the Meta and the text
// layer facts per page. An entry is dropped when the file's mtime or size
// changes, when it is evicted (file moved, filed or trashed), or as the least
// recently used one. A handle is leased to one goroutine at a time because
// PDFDocument is not documented as thread-safe; a second user opens a second
// handle instead of waiting, so a slow OCR never blocks a preview render.
type docCache struct {
	mu      sync.Mutex
	max     int // <= 0 disables the cache (every call opens the file)
	entries map[string]*docEntry
	tick    uint64
	opens   atomic.Int64 // number of C document opens, for tests and benchmarks
}

type docEntry struct {
	path   string
	mtime  time.Time
	size   int64
	used   uint64
	idle   []C.long
	dead   bool
	meta   *Meta
	layers map[int]layerInfo
}

// layerInfo is what sanity_page_text reports for one page.
type layerInfo struct {
	text   string
	pw, ph float64
	iw, ih int
}

// entry returns the live entry of path (created when new, replaced when the
// file changed), or nil when the cache is off or the file cannot be stat'ed.
func (c *docCache) entry(path string) *docEntry {
	if c.max <= 0 {
		return nil
	}
	st, err := os.Stat(path)
	var closing []C.long
	c.mu.Lock()
	if err != nil {
		if old := c.entries[path]; old != nil {
			closing = c.dropLocked(old)
		}
		c.mu.Unlock()
		closeHandles(closing)
		return nil
	}
	ent := c.entries[path]
	if ent != nil && (!ent.mtime.Equal(st.ModTime()) || ent.size != st.Size()) {
		closing = append(closing, c.dropLocked(ent)...)
		ent = nil
	}
	if ent == nil {
		if c.entries == nil {
			c.entries = map[string]*docEntry{}
		}
		ent = &docEntry{path: path, mtime: st.ModTime(), size: st.Size(), layers: map[int]layerInfo{}}
		c.entries[path] = ent
		for len(c.entries) > c.max {
			var oldest *docEntry
			for _, e := range c.entries {
				if e != ent && (oldest == nil || e.used < oldest.used) {
					oldest = e
				}
			}
			if oldest == nil {
				break
			}
			closing = append(closing, c.dropLocked(oldest)...)
		}
	}
	c.tick++
	ent.used = c.tick
	c.mu.Unlock()
	closeHandles(closing)
	return ent
}

// dropLocked removes an entry and returns its idle handles to close.
func (c *docCache) dropLocked(ent *docEntry) []C.long {
	delete(c.entries, ent.path)
	ent.dead = true
	h := ent.idle
	ent.idle = nil
	return h
}

func closeHandles(hs []C.long) {
	for _, h := range hs {
		C.sanity_doc_close(h)
	}
}

// lease returns an open handle of the entry; 0 means "no cache, open per
// call" (ent == nil).
func (c *docCache) lease(ent *docEntry, path string) (C.long, error) {
	if ent == nil {
		return 0, nil
	}
	c.mu.Lock()
	if n := len(ent.idle); n > 0 {
		h := ent.idle[n-1]
		ent.idle = ent.idle[:n-1]
		c.mu.Unlock()
		return h, nil
	}
	c.mu.Unlock()
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var errp *C.char
	c.opens.Add(1)
	h := C.sanity_doc_open(cp, &errp)
	if h == 0 {
		return 0, cerr(errp)
	}
	return h, nil
}

func (c *docCache) release(ent *docEntry, h C.long) {
	if ent == nil || h == 0 {
		return
	}
	c.mu.Lock()
	if !ent.dead && len(ent.idle) < idlePerDoc {
		ent.idle = append(ent.idle, h)
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	C.sanity_doc_close(h)
}

// evict drops the entry of path.
func (c *docCache) evict(path string) {
	c.mu.Lock()
	var closing []C.long
	if ent := c.entries[path]; ent != nil {
		closing = c.dropLocked(ent)
	}
	c.mu.Unlock()
	closeHandles(closing)
}

func (c *docCache) clear() {
	c.mu.Lock()
	var closing []C.long
	for _, ent := range c.entries {
		closing = append(closing, c.dropLocked(ent)...)
	}
	c.mu.Unlock()
	closeHandles(closing)
}

// withDoc runs fn with a leased handle of path.
func (c *docCache) withDoc(path string, fn func(ent *docEntry, h C.long) error) error {
	ent := c.entry(path)
	h, err := c.lease(ent, path)
	if err != nil {
		return err
	}
	defer c.release(ent, h)
	return fn(ent, h)
}

// Evict implements Closer: it forgets the open document of path, call it
// after the file was filed, moved or trashed.
func (e *engine) Evict(path string) { e.docs.evict(path) }

// Close implements Closer.
func (e *engine) Close() { e.docs.clear() }

// meta returns the document info, cached per entry.
func (e *engine) meta(path string) (Meta, error) {
	var m Meta
	err := e.docs.withDocMeta(path, func(mm Meta, _ layerInfo, _ bool) { m = mm }, -1)
	return m, err
}

// withDocMeta reads the meta (and, for page >= 0, the page layer facts) through
// the cache and hands them to fn.
func (c *docCache) withDocMeta(path string, fn func(Meta, layerInfo, bool), page int) error {
	ent := c.entry(path)
	if ent != nil {
		c.mu.Lock()
		m, ok := ent.meta, ent.meta != nil
		li, lok := ent.layers[page]
		c.mu.Unlock()
		if ok && (page < 0 || lok) {
			fn(*m, li, lok)
			return nil
		}
	}
	h, err := c.lease(ent, path)
	if err != nil {
		return err
	}
	defer c.release(ent, h)
	var mm Meta
	c.mu.Lock()
	if ent != nil && ent.meta != nil {
		mm = *ent.meta
	}
	have := ent != nil && ent.meta != nil
	c.mu.Unlock()
	if !have {
		mm, err = readMeta(h, path)
		if err != nil {
			return err
		}
		if ent != nil {
			c.mu.Lock()
			ent.meta = &mm
			c.mu.Unlock()
		}
	}
	var li layerInfo
	if page >= 0 {
		li, err = readLayer(h, path, page)
		if err != nil {
			return err
		}
		if ent != nil {
			c.mu.Lock()
			ent.layers[page] = li
			c.mu.Unlock()
		}
	}
	fn(mm, li, page >= 0)
	return nil
}

// pageLayer returns the meta and the text layer facts of a page.
func (e *engine) pageLayer(path string, page int) (Meta, layerInfo, error) {
	var m Meta
	var li layerInfo
	err := e.docs.withDocMeta(path, func(mm Meta, l layerInfo, _ bool) { m, li = mm, l }, page)
	return m, li, err
}

func (e *engine) render(path string, page int, scale float64) (image.Image, error) {
	var img image.Image
	err := e.docs.withDoc(path, func(_ *docEntry, h C.long) error {
		cp := C.CString(path)
		defer C.free(unsafe.Pointer(cp))
		var w, ht C.int
		var errp *C.char
		buf := C.sanity_render(h, cp, C.int(page), C.double(scale), &w, &ht, &errp)
		if buf == nil {
			return cerr(errp)
		}
		defer C.free(unsafe.Pointer(buf))
		rgba := image.NewRGBA(image.Rect(0, 0, int(w), int(ht)))
		copy(rgba.Pix, unsafe.Slice((*byte)(unsafe.Pointer(buf)), int(w)*int(ht)*4))
		img = rgba
		return nil
	})
	return img, err
}

func readMeta(h C.long, path string) (Meta, error) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var prod, creat, errp *C.char
	var created C.double
	var has C.int
	n := C.sanity_meta(h, cp, &prod, &creat, &created, &has, &errp)
	m := Meta{Pages: int(n)}
	if prod != nil {
		m.Producer = C.GoString(prod)
		C.free(unsafe.Pointer(prod))
	}
	if creat != nil {
		m.Creator = C.GoString(creat)
		C.free(unsafe.Pointer(creat))
	}
	if n < 0 {
		return Meta{}, cerr(errp)
	}
	if has != 0 {
		sec := float64(created)
		m.Created = time.Unix(int64(sec), int64((sec-float64(int64(sec)))*1e9))
	}
	return m, nil
}

func readLayer(h C.long, path string, page int) (layerInfo, error) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var pw, ph C.double
	var iw, ih C.int
	var errp *C.char
	t := C.sanity_page_text(h, cp, C.int(page), &pw, &ph, &iw, &ih, &errp)
	if t == nil {
		return layerInfo{}, cerr(errp)
	}
	defer C.free(unsafe.Pointer(t))
	return layerInfo{text: C.GoString(t), pw: float64(pw), ph: float64(ph), iw: int(iw), ih: int(ih)}, nil
}
