package ui

import (
	"fyne.io/fyne/v2"

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

// ShowShortcuts opens the overview dialog with a filter field.
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

	body := widgets.VBox().(*fyne.Container)
	render := func(q string) {
		var objs []fyne.CanvasObject
		groups := p.Filter(q)
		if len(groups) == 0 {
			objs = append(objs, widgets.NewLabel(i18n.T("shortcuts.none"), widgets.LabelMuted))
		}
		for _, g := range groups {
			objs = append(objs, widgets.NewLabel(g.Title, widgets.LabelHeading))
			for _, r := range g.Rows {
				objs = append(objs, widgets.Border(nil, nil, widgets.NewLabel(r.Title, widgets.LabelBody),
					widgets.NewLabel(r.Keys, widgets.LabelMuted), nil))
			}
		}
		body.Objects = objs
		body.Refresh()
	}
	render("")

	filter := widgets.NewEntry()
	filter.SetPlaceHolder(i18n.T("shortcuts.filter"))
	filter.OnChanged = render
	content := withFitHeight(func() float32 { return 460 },
		widgets.Border(filter, nil, nil, nil, widgets.Scroll(body)))
	dlg := widgets.ShowCustom(sh.Win, i18n.T("shortcuts.title"), content, "")
	filter.OnEscape = dlg.Hide // the first Esc leaves the field, the second closes
	sh.Win.Canvas().Focus(filter)
}
