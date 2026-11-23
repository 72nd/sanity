package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/sanity/ui/presenter"
)

// labelTexts returns the text of every label laid out under o.
func labelTexts(o fyne.CanvasObject) []string {
	var out []string
	for _, c := range test.LaidOutObjects(o) {
		if l, ok := c.(*widgets.Label); ok {
			out = append(out, l.Text)
		}
	}
	return out
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// The footer button opens the overview dialog, and the dialog lists the
// registered commands.
func TestShortcutsButtonOpensDialog(t *testing.T) {
	sh, _, _ := newShell(t, 0)
	for _, c := range ShortcutsCommands(sh) {
		if err := sh.Registry.Register(c); err != nil {
			t.Fatal(err)
		}
	}
	b, ok := ShortcutsButton(sh).(*widgets.Button)
	if !ok {
		t.Fatal("ShortcutsButton is not a *widgets.Button")
	}
	if b.Label != i18n.T("footer.shortcuts") || b.Icon == "" {
		t.Errorf("button label %q icon %q", b.Label, b.Icon)
	}
	if sh.Win.Canvas().Overlays().Top() != nil {
		t.Fatal("dialog open before the tap")
	}
	b.Tapped(&fyne.PointEvent{})
	top := sh.Win.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("tap did not open the dialog")
	}
	texts := labelTexts(top)
	for _, want := range []string{i18n.T("shortcuts.group.everywhere"), i18n.T("cmd.shortcuts.show")} {
		if !contains(texts, want) {
			t.Errorf("dialog misses %q, labels: %v", want, texts)
		}
	}
}

// The columns keep the group order and split the rows about evenly.
func TestShortcutsSplitGroups(t *testing.T) {
	group := func(title string, rows int) presenter.ShortcutGroup {
		return presenter.ShortcutGroup{Title: title, Rows: make([]presenter.ShortcutRow, rows)}
	}
	gs := []presenter.ShortcutGroup{group("a", 5), group("b", 5), group("c", 2)}
	l, r := splitGroups(gs)
	if len(l) != 1 || l[0].Title != "a" || len(r) != 2 || r[0].Title != "b" || r[1].Title != "c" {
		t.Errorf("split = %v | %v", titles(l), titles(r))
	}
	if l, r := splitGroups(gs[:1]); len(l) != 1 || len(r) != 0 {
		t.Errorf("single group split = %v | %v", titles(l), titles(r))
	}
	if l, r := splitGroups(nil); len(l) != 0 || len(r) != 0 {
		t.Errorf("no groups split = %v | %v", titles(l), titles(r))
	}
}

func titles(gs []presenter.ShortcutGroup) []string {
	var out []string
	for _, g := range gs {
		out = append(out, g.Title)
	}
	return out
}
