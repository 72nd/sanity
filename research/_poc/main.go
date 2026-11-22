package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework PDFKit -framework Vision -framework AppKit -framework CoreGraphics
#include "pdfmac.h"
*/
import "C"
import (
	"fmt"
	"os"
	"time"
	"unsafe"
)

func main() {
	if os.Args[1]=="mk" { a:=C.CString(os.Args[2]); b:=C.CString(os.Args[3]); fmt.Println("png_to_pdf", C.png_to_pdf(a,b)); return }
	path := C.CString(os.Args[1])
	defer C.free(unsafe.Pointer(path))
	t := time.Now()
	fmt.Println("pages:", C.pdf_page_count(path), time.Since(t))

	t = time.Now()
	var n C.size_t
	var cerr *C.char
	p := C.pdf_render_png(path, 0, 2.0, &n, &cerr)
	png := C.GoBytes(unsafe.Pointer(p), C.int(n))
	C.free(unsafe.Pointer(p))
	os.WriteFile(os.Args[2], png, 0o644)
	fmt.Println("render 144dpi png bytes:", len(png), time.Since(t))

	t = time.Now()
	tx := C.pdf_text(path, -1)
	txt := C.GoString(tx)
	fmt.Printf("PDFKit text (%v): %q\n", time.Since(t), txt)

	// OCR at ~216dpi (scale 3)
	p = C.pdf_render_png(path, 0, 4.0, &n, &cerr)
	for _, acc := range []int{0, 1} {
		for i := 0; i < 2; i++ {
			t = time.Now()
			o := C.ocr_png((*C.uchar)(p), n, C.int(acc), &cerr)
			if o == nil {
				fmt.Println("ocr err", C.GoString(cerr))
				break
			}
			fmt.Printf("OCR accurate=%d run%d %v\n", acc, i, time.Since(t))
			if i == 1 {
				fmt.Println(C.GoString(o))
				ds := C.detect_dates(o)
				fmt.Println("dates from OCR:\n" + C.GoString(ds))
			}
		}
	}
	ds := C.detect_dates(tx)
	fmt.Println("dates from text:\n" + C.GoString(ds))
	extra := C.CString("Rechnung vom 12. März 2025, fällig 3. Okt. 2025, Lieferung 12.03.25, Stand 01.02.2024, 5/2025, Dezember 2024, 31.12.")
	fmt.Println("dates misc:\n" + C.GoString(C.detect_dates(extra)))
}
