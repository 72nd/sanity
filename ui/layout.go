package ui

import "fyne.io/fyne/v2"

// fitHeight gives its single child the height h() asks for, at least the
// child's own minimum, and the full width. h is called on every layout, so the
// child can follow its content (a grid that shows all rows up to a limit).
type fitHeight struct{ h func() float32 }

func (f fitHeight) MinSize(o []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, c := range o {
		s = s.Max(c.MinSize())
	}
	s.Height = max(s.Height, f.h())
	return s
}

func (f fitHeight) Layout(o []fyne.CanvasObject, size fyne.Size) {
	for _, c := range o {
		c.Move(fyne.NewPos(0, 0))
		c.Resize(size)
	}
}

// withFitHeight wraps o so that its height follows h().
func withFitHeight(h func() float32, o fyne.CanvasObject) fyne.CanvasObject {
	return fyne.NewContainerWithLayout(fitHeight{h}, o)
}
