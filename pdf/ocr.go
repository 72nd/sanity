package pdf

// OCROptions tune the Vision run on a page.
type OCROptions struct {
	// Scale renders the page (1 = 72 dpi) before recognition.
	Scale float64
	// MinTextHeight is the smallest text height Vision looks for, relative to
	// the image height. 0 keeps Vision's default of about 1/32, which skips the
	// small print of a receipt on a full page.
	MinTextHeight float64
}

// DefaultOCR is what Text uses for scans: about 250 dpi, and text down to
// about 0.5 % of the page height (6 pt on an A4 page).
var DefaultOCR = OCROptions{Scale: 1.5, MinTextHeight: 0.005}
