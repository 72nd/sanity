package pdf

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"
)

func page(w, h int, bg uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = bg, bg, bg, 0xff
	}
	return img
}

func block(img *image.RGBA, r image.Rectangle, v uint8) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.Set(x, y, grey(v))
		}
	}
}

func TestCropWhiteCutsMargins(t *testing.T) {
	for _, bg := range []uint8{0xff, 0xe8} { // white and a grey scan background
		img := page(600, 800, bg)
		block(img, image.Rect(200, 300, 400, 500), 0x20)
		got := CropWhite(img).Bounds()
		if got.Dx() > 300 || got.Dy() > 300 {
			t.Errorf("bg %#x: %v is too large", bg, got)
		}
		if got.Min.X > 200 || got.Max.X < 400 || got.Min.Y > 300 || got.Max.Y < 500 {
			t.Errorf("bg %#x: %v cuts content", bg, got)
		}
	}
}

func TestCropWhiteIgnoresDust(t *testing.T) {
	img := page(600, 800, 0xff)
	block(img, image.Rect(200, 300, 400, 500), 0x20)
	img.Set(5, 5, grey(0)) // one stray pixel in the corner
	if got := CropWhite(img).Bounds(); got.Min.X < 100 || got.Min.Y < 100 {
		t.Errorf("dust widened the crop: %v", got)
	}
}

func TestCropWhiteKeepsBlankAndFullPages(t *testing.T) {
	blank := page(600, 800, 0xff)
	if CropWhite(blank).Bounds() != blank.Bounds() {
		t.Error("blank page must stay as is")
	}
	full := page(600, 800, 0xff)
	block(full, image.Rect(5, 5, 595, 795), 0x30)
	if CropWhite(full).Bounds() != full.Bounds() {
		t.Error("full page must stay as is")
	}
}

func TestCropWhiteOnCorpus(t *testing.T) {
	files, _ := filepath.Glob("../testdata/*.pdf")
	e := NewEngine()
	for _, f := range files {
		img, err := e.Render(f, 0, 1.5)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		c := CropWhite(img)
		t.Logf("%-28s %v -> %v (%.0f%% of the area)", filepath.Base(f), img.Bounds().Size(), c.Bounds().Size(),
			100*float64(c.Bounds().Dx()*c.Bounds().Dy())/float64(img.Bounds().Dx()*img.Bounds().Dy()))
	}
}

func grey(v uint8) color.Color { return color.Gray{Y: v} }
