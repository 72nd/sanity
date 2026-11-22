package pdf

import "image"

// CropWhite cuts the empty margin off a rendered page so the content fills
// the preview. The background is the median luminance of the page (white for
// born-digital PDFs, light grey for many scans); a pixel is content when it is
// clearly darker. Rows and columns need a few content pixels, so scanner dust
// does not count. The result keeps a small padding. The original is returned
// when nothing can be cropped or the page looks blank.
func CropWhite(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 16 || h < 16 {
		return img
	}
	lum := make([]uint8, w*h)
	var hist [256]int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			l := uint8((299*(r>>8) + 587*(g>>8) + 114*(bl>>8)) / 1000)
			lum[y*w+x] = l
			hist[l]++
		}
	}
	median, acc := 255, 0
	for i, n := range hist {
		acc += n
		if acc*2 >= w*h {
			median = i
			break
		}
	}
	thr := max(median-40, 20)
	rows := make([]int, h)
	cols := make([]int, w)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if int(lum[y*w+x]) < thr {
				rows[y]++
				cols[x]++
			}
		}
	}
	minRow, minCol := max(2, w/500), max(2, h/500)
	top, bottom := firstLast(rows, minRow)
	left, right := firstLast(cols, minCol)
	if top < 0 || left < 0 || bottom <= top || right <= left {
		return img
	}
	padX, padY := max(8, (right-left)/25), max(8, (bottom-top)/25)
	r := image.Rect(max(left-padX, 0), max(top-padY, 0), min(right+padX+1, w), min(bottom+padY+1, h))
	if r.Dx()*100 >= w*96 && r.Dy()*100 >= h*96 {
		return img // nothing to gain
	}
	r = r.Add(b.Min)
	if s, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return s.SubImage(r)
	}
	return img
}

// firstLast returns the first and last index with a count of at least min, or
// -1, -1.
func firstLast(counts []int, min int) (int, int) {
	first, last := -1, -1
	for i, c := range counts {
		if c >= min {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	return first, last
}
