package ui

import "fyne.io/fyne/v2"

// minHeight gives its single child at least h of height (a grid has a small
// minimum size of its own) and the full width.
type minHeight struct{ h float32 }

func (m minHeight) MinSize(o []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, c := range o {
		s = s.Max(c.MinSize())
	}
	s.Height = max(s.Height, m.h)
	return s
}

func (m minHeight) Layout(o []fyne.CanvasObject, size fyne.Size) {
	for _, c := range o {
		c.Move(fyne.NewPos(0, 0))
		c.Resize(size)
	}
}

// withMinHeight wraps o so that it is at least h tall.
func withMinHeight(h float32, o fyne.CanvasObject) fyne.CanvasObject {
	return fyne.NewContainerWithLayout(minHeight{h}, o)
}
