package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
)

// LibraryActions are the library page's operations that commands, the context
// menu and the Vim layer call. They are assigned while the page is built (the
// page is rebuilt on every visit) and are no-ops before.
//
// Page-aware shortcuts: the shell registry has one global shortcut namespace,
// and Sort already owns Cmd+F, Cmd+Shift+C, Cmd+Alt+C and Cmd+R. Instead of
// swapping the SortActions fields (the Sort page assigns them once and is
// kept), Commands wraps those four commands (see libraryAware): while the
// current page is "library" they run the matching LibraryActions field, on
// every other page the unchanged Sort behaviour. Cmd+E (re-sort) exists for
// the Library only and does nothing elsewhere.
type LibraryActions struct {
	Search, CopyRel, CopyAbs, Reveal, Resort, Remove, Open func()
	// FilterColumn opens the value picker of the active column, Conditions
	// the condition dialog; ClearFilters and Ungroup reset the grid view.
	FilterColumn, Conditions, ClearFilters, Ungroup func()
	Move                                            func(delta int) // rows down (positive) or up
	Jump                                            func(first bool)
	Hit                                             func(forward bool) // n / N
	Visual                                          func(all bool)     // v extends from here, V selects all rows
	// Rune handles a typed rune of the Vim layer; true means it was used.
	Rune func(r rune) bool
	// SetMode is called with true for normal mode (grid focus) and false for
	// insert mode (the filter field has the focus).
	SetMode func(normal bool)

	sh    *shell.Shell
	focus fyne.Focusable // the grid, focused when the page is shown
}

// NewLibraryActions returns actions that do nothing until the page exists.
func NewLibraryActions() *LibraryActions {
	return &LibraryActions{
		FilterColumn: nop, Conditions: nop, ClearFilters: nop, Ungroup: nop,
		Search: nop, CopyRel: nop, CopyAbs: nop, Reveal: nop, Resort: nop, Remove: nop, Open: nop,
		Move: func(int) {}, Jump: func(bool) {}, Hit: func(bool) {}, Visual: func(bool) {},
		Rune: func(rune) bool { return false }, SetMode: func(bool) {},
	}
}

// Shown is called from Page.OnShow of the library page.
func (l *LibraryActions) Shown(sh *shell.Shell) { l.sh = sh }

// Active reports whether the library is the page on screen.
func (l *LibraryActions) Active() bool { return l.sh != nil && l.sh.Current() == "library" }

// Focus is the Page.Focus function of the library page: the grid.
func (l *LibraryActions) Focus(*shell.Shell, fyne.CanvasObject) fyne.Focusable { return l.focus }

// libraryAware makes the shared shortcuts page-aware and adds the Library only
// commands. Commands calls it on the sort commands.
func libraryAware(a *SortActions, cmds []shell.Command) []shell.Command {
	lib := a.Library
	route := map[string]func(){
		"sort.search":  func() { lib.Search() },
		"sort.copyrel": func() { lib.CopyRel() },
		"sort.copyabs": func() { lib.CopyAbs() },
		"sort.reveal":  func() { lib.Reveal() },
	}
	for i := range cmds {
		onLib := route[cmds[i].ID]
		if onLib == nil {
			continue
		}
		onSort := cmds[i].Run
		cmds[i].Run = func() {
			if lib.Active() {
				onLib()
				return
			}
			onSort()
		}
	}
	only := func(id, title string, sc *desktop.CustomShortcut, run func()) shell.Command {
		return shell.Command{ID: id, Title: func() string { return i18n.T(title) }, Shortcut: sc, Run: func() {
			if lib.Active() {
				run()
			}
		}}
	}
	return append(cmds,
		only("library.resort", "cmd.library.resort", shell.Ctrl(fyne.KeyE), func() { lib.Resort() }),
		only("library.filtercol", "cmd.library.filtercol",
			&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierAlt}, func() { lib.FilterColumn() }),
		only("library.conditions", "cmd.library.conditions",
			&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift}, func() { lib.Conditions() }),
		only("library.clearfilters", "cmd.library.clearfilters", nil, func() { lib.ClearFilters() }),
		only("library.ungroup", "cmd.library.ungroup", nil, func() { lib.Ungroup() }),
	)
}
