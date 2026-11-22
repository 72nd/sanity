//go:build darwin

package pdf

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework PDFKit -framework Vision -framework AppKit -framework CoreGraphics
#include <stdlib.h>
#include "pdfmac.h"
*/
import "C"

import (
	"errors"
	"image"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// OCRPage runs Vision on one page with explicit options and returns the text
// (empty when nothing was found). It is meant for tuning with the ocrdump tool.
func OCRPage(path string, page int, o OCROptions) (string, error) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var errp *C.char
	t := C.sanity_ocr_page(cp, C.int(page), C.double(o.Scale), C.double(o.MinTextHeight), &errp)
	if t == nil {
		if errp != nil {
			return "", cerr(errp)
		}
		return "", nil
	}
	defer C.free(unsafe.Pointer(t))
	return C.GoString(t), nil
}

type engine struct {
	once sync.Once
	ocr  bool
}

// NewEngine returns the PDFKit/Vision engine.
func NewEngine() Engine { return &engine{} }

func cerr(p *C.char) error {
	if p == nil {
		return errors.New("pdf: unknown error")
	}
	defer C.free(unsafe.Pointer(p))
	return errors.New("pdf: " + C.GoString(p))
}

func (e *engine) Meta(path string) (Meta, error) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var prod, creat, errp *C.char
	var created C.double
	var has C.int
	n := C.sanity_meta(cp, &prod, &creat, &created, &has, &errp)
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

func (e *engine) Render(path string, page int, scale float64) (image.Image, error) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var w, h C.int
	var errp *C.char
	buf := C.sanity_render(cp, C.int(page), C.double(scale), &w, &h, &errp)
	if buf == nil {
		return nil, cerr(errp)
	}
	defer C.free(unsafe.Pointer(buf))
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	copy(img.Pix, unsafe.Slice((*byte)(unsafe.Pointer(buf)), int(w)*int(h)*4))
	return img, nil
}

func (e *engine) Text(path string, page int) (PageText, error) {
	m, err := e.Meta(path)
	if err != nil {
		return PageText{}, err
	}
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var pw, ph C.double
	var iw, ih C.int
	var errp *C.char
	t := C.sanity_page_text(cp, C.int(page), &pw, &ph, &iw, &ih, &errp)
	if t == nil {
		return PageText{}, cerr(errp)
	}
	layer := C.GoString(t)
	C.free(unsafe.Pointer(t))

	scan, _ := ClassifyScan(PageInfo{
		Text: layer, Producer: m.Producer, Creator: m.Creator,
		PageW: float64(pw), PageH: float64(ph), ImageW: int(iw), ImageH: int(ih),
	})
	res := PageText{Text: layer, Source: SourceLayer, Scan: scan}
	if !scan || !e.OCRAvailable() {
		return res, nil
	}
	// OCR failing or finding nothing leaves the layer as the fallback. A good
	// layer also stays when the OCR text is much shorter (see PreferOCR).
	if ocr, err := OCRPage(path, page, DefaultOCR); err == nil && PreferOCR(layer, ocr) {
		res.Text, res.Source = ocr, SourceVision
	}
	return res, nil
}

func (e *engine) Dates(text string) []string {
	ct := C.CString(text)
	defer C.free(unsafe.Pointer(ct))
	r := C.sanity_dates(ct)
	if r == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(r))
	var out []string
	for _, l := range strings.Split(C.GoString(r), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func (e *engine) OCRAvailable() bool {
	e.once.Do(func() { e.ocr = C.sanity_ocr_available() != 0 })
	return e.ocr
}
