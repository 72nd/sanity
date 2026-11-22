package pdf

import (
	"fmt"
	"image"
)

// Fake is an in-memory Engine for tests. Every path maps to the same
// configured document unless Docs has an entry for it.
type Fake struct {
	Docs    map[string]FakeDoc
	Default FakeDoc
	OCR     bool // value of OCRAvailable
	DateOut []string
}

// FakeDoc configures one fake document.
type FakeDoc struct {
	Meta  Meta
	Pages []PageText // len(Pages) wins over Meta.Pages
	Image image.Image
}

var _ Engine = (*Fake)(nil)

// NewFake returns a Fake whose documents have the given page texts (layer source).
func NewFake(texts ...string) *Fake {
	d := FakeDoc{Meta: Meta{Pages: len(texts)}}
	for _, t := range texts {
		d.Pages = append(d.Pages, PageText{Text: t, Source: SourceLayer})
	}
	return &Fake{Default: d, OCR: true}
}

func (f *Fake) doc(path string) FakeDoc {
	if d, ok := f.Docs[path]; ok {
		return d
	}
	return f.Default
}

// Meta implements Engine.
func (f *Fake) Meta(path string) (Meta, error) {
	d := f.doc(path)
	m := d.Meta
	if len(d.Pages) > 0 {
		m.Pages = len(d.Pages)
	}
	return m, nil
}

// Render implements Engine. It returns the configured image or a white A4 page.
func (f *Fake) Render(path string, page int, scale float64) (image.Image, error) {
	d := f.doc(path)
	if page < 0 || page >= max(len(d.Pages), d.Meta.Pages) {
		return nil, fmt.Errorf("pdf: page %d out of range", page)
	}
	if d.Image != nil {
		return d.Image, nil
	}
	w, h := int(595*scale), int(842*scale)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff // opaque white
	}
	return img, nil
}

// Text implements Engine.
func (f *Fake) Text(path string, page int) (PageText, error) {
	d := f.doc(path)
	if page < 0 || page >= len(d.Pages) {
		return PageText{}, fmt.Errorf("pdf: page %d out of range", page)
	}
	return d.Pages[page], nil
}

// Dates implements Engine.
func (f *Fake) Dates(string) []string { return f.DateOut }

// OCRAvailable implements Engine.
func (f *Fake) OCRAvailable() bool { return f.OCR }
