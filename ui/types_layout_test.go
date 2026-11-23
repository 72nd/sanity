package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// withWrap puts the pair side by side from the break width and stacks it below.
func TestWithWrap(t *testing.T) {
	a, b := canvas.NewRectangle(nil), canvas.NewRectangle(nil)
	a.SetMinSize(fyne.NewSize(100, 30))
	b.SetMinSize(fyne.NewSize(100, 30))
	c := withWrap(500, 10, a, b)
	c.Resize(fyne.NewSize(800, 30))
	if a.Position().Y != b.Position().Y || b.Position().X <= a.Position().X {
		t.Errorf("wide: a=%v b=%v, want side by side", a.Position(), b.Position())
	}
	c.Resize(fyne.NewSize(300, 70))
	if b.Position().X != 0 || b.Position().Y <= a.Position().Y {
		t.Errorf("narrow: a=%v b=%v, want stacked", a.Position(), b.Position())
	}
}
