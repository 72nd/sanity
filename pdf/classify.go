package pdf

import (
	"strings"
	"unicode"
)

// Scan classification thresholds (PLAN.md section 3).
const (
	minLayerChars    = 40
	maxGarbageShare  = 0.25 // share of non-letter garbage among non-space runes
	maxSingleTokens  = 0.45 // share of one-rune tokens among all tokens
	minQualityRunes  = 40   // below this the quality check is not meaningful
	imageCoverMin    = 0.85 // image aspect/size coverage of the page
	imageMinPixelDPI = 100  // lower resolution images do not count as a page scan
)

// PageInfo is what the classifier needs to know about a page.
type PageInfo struct {
	Text     string  // embedded text layer
	Producer string  // document Producer
	Creator  string  // document Creator
	PageW    float64 // page size in points
	PageH    float64
	ImageW   int // pixel size of the largest embedded image, 0 when none
	ImageH   int
}

// ScanReason names why a page was classified as a scan; empty means born digital.
type ScanReason string

const (
	ReasonNone     ScanReason = ""
	ReasonNoText   ScanReason = "no-text"
	ReasonProducer ScanReason = "producer"
	ReasonImage    ScanReason = "page-image"
	ReasonQuality  ScanReason = "low-quality"
)

var ocrProducers = []string{"ocrmypdf", "tesseract", "paperless"}

// ClassifyScan decides whether a page needs OCR and says why.
func ClassifyScan(p PageInfo) (bool, ScanReason) {
	if len([]rune(strings.TrimSpace(p.Text))) < minLayerChars {
		return true, ReasonNoText
	}
	if producerSaysOCR(p.Producer) || producerSaysOCR(p.Creator) {
		return true, ReasonProducer
	}
	if pageSizedImage(p) {
		return true, ReasonImage
	}
	if LowQuality(p.Text) {
		return true, ReasonQuality
	}
	return false, ReasonNone
}

func producerSaysOCR(s string) bool {
	s = strings.ToLower(s)
	for _, k := range ocrProducers {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// pageSizedImage reports whether one image plausibly covers the whole page:
// its aspect ratio matches the page and its resolution is scan-like.
func pageSizedImage(p PageInfo) bool {
	if p.ImageW <= 0 || p.ImageH <= 0 || p.PageW <= 0 || p.PageH <= 0 {
		return false
	}
	dpiW := float64(p.ImageW) / p.PageW * 72
	dpiH := float64(p.ImageH) / p.PageH * 72
	if dpiW < imageMinPixelDPI || dpiH < imageMinPixelDPI {
		return false
	}
	ratio := dpiW / dpiH
	return ratio > imageCoverMin && ratio < 1/imageCoverMin
}

// LowQuality reports whether a text layer looks like OCR garbage: many
// symbols that are neither letters, digits nor usual punctuation, or mostly
// one-character tokens.
func LowQuality(text string) bool {
	total, garbage := 0, 0
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(".,;:!?-–—'\"()/%&@#+*€$£_´’“”„", r)) {
			garbage++
		}
	}
	if total < minQualityRunes {
		return false
	}
	if float64(garbage)/float64(total) > maxGarbageShare {
		return true
	}
	tokens := strings.Fields(text)
	single := 0
	for _, t := range tokens {
		if len([]rune(t)) == 1 {
			single++
		}
	}
	return len(tokens) > 0 && float64(single)/float64(len(tokens)) > maxSingleTokens
}

// PreferOCR decides whether the Vision text should replace the embedded text
// layer of a page that was classified as a scan. An empty OCR result never
// wins. A low-quality layer (Paperless/Tesseract garbage) yields unless the OCR
// text is much shorter; a good layer (an OCRmyPDF/Tesseract scan, say) is only replaced
// by clearly longer OCR text, since equal length says nothing about which text
// is right and Vision misreads digits at times.
func PreferOCR(layer, ocr string) bool {
	l, o := len([]rune(strings.TrimSpace(layer))), len([]rune(strings.TrimSpace(ocr)))
	switch {
	case o == 0:
		return false
	case l == 0:
		return true
	case LowQuality(layer):
		return o*2 >= l
	default:
		return o*5 >= l*6
	}
}
