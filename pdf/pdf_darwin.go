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
	docs docCache
}

// NewEngine returns the PDFKit/Vision engine.
func NewEngine() Engine { return &engine{docs: docCache{max: maxOpenDocs}} }

func cerr(p *C.char) error {
	if p == nil {
		return errors.New("pdf: unknown error")
	}
	defer C.free(unsafe.Pointer(p))
	return errors.New("pdf: " + C.GoString(p))
}

func (e *engine) Meta(path string) (Meta, error) { return e.meta(path) }

func (e *engine) Render(path string, page int, scale float64) (image.Image, error) {
	return e.render(path, page, scale)
}

func (e *engine) Text(path string, page int) (PageText, error) {
	m, li, err := e.pageLayer(path, page) // one cached open for meta and layer
	if err != nil {
		return PageText{}, err
	}
	layer, pw, ph, iw, ih := li.text, li.pw, li.ph, li.iw, li.ih

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
