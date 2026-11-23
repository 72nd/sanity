package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/sanity/ui/presenter"
)

// ShortcutsCommands returns the command that opens the shortcut overview. It
// reads the registry when the command runs, so it lists every command that is
// registered by then.
func ShortcutsCommands(sh *shell.Shell) []shell.Command {
	return []shell.Command{{
		ID:       "shortcuts.show",
		Title:    func() string { return i18n.T("cmd.shortcuts.show") },
		Shortcut: shell.Ctrl(fyne.KeySlash),
		Run:      func() { ShowShortcuts(sh) },
	}}
}

// ShortcutsButton returns the footer button that opens the shortcut overview.
// The footer itself is wired in main.go.
func ShortcutsButton(sh *shell.Shell) fyne.CanvasObject {
	b := widgets.NewButton(i18n.T("footer.shortcuts"), widgets.ButtonToolbar, func() { ShowShortcuts(sh) })
	b.SetIcon(theme.IconNameHelp)
	return b
}

// ShowShortcuts opens the overview dialog with a filter field. The rows sit in
// two columns, so the dialog stays short and wide.
func ShowShortcuts(sh *shell.Shell) {
	var cmds []presenter.ShortcutCmd
	for _, c := range sh.Registry.Commands() {
		sc := presenter.ShortcutCmd{ID: c.ID, Title: c.Title()}
		if c.Shortcut != nil {
			sc.Shortcut = shell.ShortcutLabel(c.Shortcut)
		}
		cmds = append(cmds, sc)
	}
	p := presenter.NewShortcuts(cmds, VimEnabled(sh), func(id string) string { return i18n.T(id) })

	left := widgets.VBox().(*fyne.Container)
	right := widgets.VBox().(*fyne.Container)
	render := func(q string) {
		groups := p.Filter(q)
		l, r := splitGroups(groups)
		left.Objects = groupObjects(l)
		right.Objects = groupObjects(r)
		if len(groups) == 0 {
			left.Objects = []fyne.CanvasObject{widgets.NewLabel(i18n.T("shortcuts.none"), widgets.LabelMuted)}
		}
		left.Refresh()
		right.Refresh()
	}
	render("")

	filter := widgets.NewEntry()
	filter.SetPlaceHolder(i18n.T("shortcuts.filter"))
	filter.OnChanged = render
	content := withFitHeight(func() float32 { return 460 },
		widgets.Border(filter, nil, nil, nil, widgets.VScroll(widgets.PagePadded(widgets.Columns(left, right)))))
	dlg := widgets.ShowDialog(sh.Win, i18n.T("shortcuts.title"), content, widgets.DialogOptions{
		MinWidth:    720,
		CancelLabel: i18n.T("shortcuts.close"),
	})
	filter.OnEscape = dlg.Hide // the first Esc leaves the field, the second closes
	sh.Win.Canvas().Focus(filter)
}

// splitGroups divides the groups into a left and a right column of about the
// same height, keeping their order. A group is never split. Each group counts
// its title as one row.
func splitGroups(groups []presenter.ShortcutGroup) (left, right []presenter.ShortcutGroup) {
	if len(groups) < 2 {
		return groups, nil
	}
	weight := func(g presenter.ShortcutGroup) int { return 1 + len(g.Rows) }
	total := 0
	for _, g := range groups {
		total += weight(g)
	}
	best, bestDiff, acc := 1, -1, 0
	for k := 1; k < len(groups); k++ {
		acc += weight(groups[k-1])
		diff := total - 2*acc
		if diff < 0 {
			diff = -diff
		}
		if bestDiff < 0 || diff < bestDiff {
			best, bestDiff = k, diff
		}
	}
	return groups[:best], groups[best:]
}

// groupObjects lays out the groups as headings with one line per row.
func groupObjects(groups []presenter.ShortcutGroup) []fyne.CanvasObject {
	var objs []fyne.CanvasObject
	for _, g := range groups {
		objs = append(objs, widgets.NewLabel(g.Title, widgets.LabelHeading))
		for _, r := range g.Rows {
			objs = append(objs, widgets.Border(nil, nil, widgets.NewLabel(r.Title, widgets.LabelBody),
				widgets.NewLabel(r.Keys, widgets.LabelMuted), nil))
		}
	}
	return objs
}
