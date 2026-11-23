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

// wrapPair places a and b side by side (equal widths) when the available width
// is at least breakAt, and stacks them otherwise.
type wrapPair struct {
	breakAt float32
	gap     float32
	stacked bool // result of the last Layout; MinSize follows it
}

func (w *wrapPair) MinSize(o []fyne.CanvasObject) fyne.Size {
	if len(o) != 2 {
		return fyne.Size{}
	}
	a, b := o[0].MinSize(), o[1].MinSize()
	if w.stacked {
		return fyne.NewSize(max(a.Width, b.Width), a.Height+w.gap+b.Height)
	}
	return fyne.NewSize(a.Width+w.gap+b.Width, max(a.Height, b.Height))
}

func (w *wrapPair) Layout(o []fyne.CanvasObject, size fyne.Size) {
	if len(o) != 2 {
		return
	}
	w.stacked = size.Width < w.breakAt
	if w.stacked {
		a, b := o[0].MinSize().Height, o[1].MinSize().Height
		o[0].Move(fyne.NewPos(0, 0))
		o[0].Resize(fyne.NewSize(size.Width, a))
		o[1].Move(fyne.NewPos(0, a+w.gap))
		o[1].Resize(fyne.NewSize(size.Width, b))
		return
	}
	half := (size.Width - w.gap) / 2
	o[0].Move(fyne.NewPos(0, 0))
	o[0].Resize(fyne.NewSize(half, size.Height))
	o[1].Move(fyne.NewPos(half+w.gap, 0))
	o[1].Resize(fyne.NewSize(half, size.Height))
}

// withWrap puts a and b side by side while the width is at least breakAt and
// stacks them below it.
func withWrap(breakAt, gap float32, a, b fyne.CanvasObject) fyne.CanvasObject {
	return fyne.NewContainerWithLayout(&wrapPair{breakAt: breakAt, gap: gap}, a, b)
}
