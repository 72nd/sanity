package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
)

// SortActions are the sort page's operations that commands and the Vim layer
// call. They are set while the page is built and are no-ops before.
type SortActions struct {
	File, Undo, Skip, SkipBack, AddFiles func()
	ZoomIn, ZoomOut, ZoomFit             func()
	ScrollDown, ScrollUp, Search         func()
	PageNext, PagePrev                   func()
	LearnRules                           func()
	Import                               func(paths ...string)
	CopyRel, CopyAbs, Reveal, Remove     func()
	First, Last                          func()
	PasteFiles                           func() // import files copied in Finder
	FileBatch, UndoBatch                 func() // file the selected documents with one type; undo the last batch
	// SetMode is called with true when the keyboard focus left the form (Vim
	// normal mode) and with false when a field has it again (insert mode).
	SetMode func(normal bool)
	Focus   func(field string) // "type", "date" or "" for the first empty field

	// Library holds the library page's operations, see LibraryActions.
	Library *LibraryActions

	shown  func()
	fields *sortWidgets
}

func nop() {}

// NewSortActions returns actions that do nothing until the page exists.
func NewSortActions() *SortActions {
	return &SortActions{
		File: nop, Undo: nop, Skip: nop, SkipBack: nop, AddFiles: nop,
		ZoomIn: nop, ZoomOut: nop, ZoomFit: nop,
		ScrollDown: nop, ScrollUp: nop, Search: nop, PageNext: nop, PagePrev: nop, LearnRules: nop,
		Import: func(...string) {}, Focus: func(string) {}, shown: nop, Library: NewLibraryActions(),
		CopyRel: nop, CopyAbs: nop, Reveal: nop, Remove: nop, First: nop, Last: nop, PasteFiles: nop, FileBatch: nop, UndoBatch: nop, SetMode: func(bool) {},
	}
}

// Commands returns the sort page's commands; each is reachable through the
// palette and has the shortcut from PLAN.md section 2.
func Commands(a *SortActions) []shell.Command { return libraryAware(a, sortCommands(a)) }

// sortCommands are the commands of the sort page; Commands makes the shared
// shortcuts page-aware.
func sortCommands(a *SortActions) []shell.Command {
	mod := func(k fyne.KeyName, m fyne.KeyModifier) *desktop.CustomShortcut {
		return &desktop.CustomShortcut{KeyName: k, Modifier: fyne.KeyModifierShortcutDefault | m}
	}
	t := func(id string) func() string { return func() string { return i18n.T(id) } }
	return []shell.Command{
		{ID: "sort.file", Title: t("cmd.sort.file"), Shortcut: shell.Ctrl(fyne.KeyReturn), Run: func() { a.File() }},
		{ID: "sort.batch", Title: t("cmd.sort.batch"), Shortcut: mod(fyne.KeyReturn, fyne.KeyModifierShift), Run: func() { a.FileBatch() }},
		{ID: "sort.undobatch", Title: t("cmd.sort.undobatch"), Run: func() { a.UndoBatch() }},
		{ID: "sort.undo", Title: t("cmd.sort.undo"), Shortcut: mod(fyne.KeyZ, fyne.KeyModifierAlt), Run: func() { a.Undo() }},
		{ID: "sort.next", Title: t("cmd.sort.next"), Shortcut: shell.Ctrl(fyne.KeyRightBracket), Run: func() { a.Skip() }},
		{ID: "sort.prev", Title: t("cmd.sort.prev"), Shortcut: shell.Ctrl(fyne.KeyLeftBracket), Run: func() { a.SkipBack() }},
		{ID: "sort.add", Title: t("cmd.sort.add"), Shortcut: shell.Ctrl(fyne.KeyO), Run: func() { a.AddFiles() }},
		{ID: "sort.search", Title: t("cmd.sort.search"), Shortcut: shell.Ctrl(fyne.KeyF), Run: func() { a.Search() }},
		{ID: "sort.copyrel", Title: t("cmd.sort.copyrel"), Shortcut: mod(fyne.KeyC, fyne.KeyModifierShift), Run: func() { a.CopyRel() }},
		{ID: "sort.copyabs", Title: t("cmd.sort.copyabs"), Shortcut: mod(fyne.KeyC, fyne.KeyModifierAlt), Run: func() { a.CopyAbs() }},
		{ID: "sort.paste", Title: t("cmd.sort.paste"), Shortcut: mod(fyne.KeyV, fyne.KeyModifierShift), Run: func() { a.PasteFiles() }},
		{ID: "sort.reveal", Title: t("cmd.sort.reveal"), Shortcut: shell.Ctrl(fyne.KeyR), Run: func() { a.Reveal() }},
		{ID: "sort.remove", Title: t("cmd.sort.remove"), Shortcut: mod(fyne.KeyBackspace, fyne.KeyModifierShift), Run: func() { a.Remove() }},
		{ID: "sort.pagenext", Title: t("cmd.sort.pagenext"), Shortcut: &desktop.CustomShortcut{KeyName: fyne.KeyDown, Modifier: fyne.KeyModifierAlt}, Run: func() { a.PageNext() }},
		{ID: "sort.pageprev", Title: t("cmd.sort.pageprev"), Shortcut: &desktop.CustomShortcut{KeyName: fyne.KeyUp, Modifier: fyne.KeyModifierAlt}, Run: func() { a.PagePrev() }},
		{ID: "sort.scrolldown", Title: t("cmd.sort.scrolldown"), Shortcut: &desktop.CustomShortcut{KeyName: fyne.KeyD, Modifier: fyne.KeyModifierControl}, Run: func() { a.ScrollDown() }},
		{ID: "sort.scrollup", Title: t("cmd.sort.scrollup"), Shortcut: &desktop.CustomShortcut{KeyName: fyne.KeyU, Modifier: fyne.KeyModifierControl}, Run: func() { a.ScrollUp() }},
		{ID: "sort.learn", Title: t("cmd.sort.learn"), Shortcut: shell.Ctrl(fyne.KeyL), Run: func() { a.LearnRules() }},
		{ID: "sort.zoomin", Title: t("cmd.sort.zoomin"), Shortcut: shell.Ctrl(fyne.KeyEqual), Run: func() { a.ZoomIn() }},
		{ID: "sort.zoomout", Title: t("cmd.sort.zoomout"), Shortcut: shell.Ctrl(fyne.KeyMinus), Run: func() { a.ZoomOut() }},
		{ID: "sort.zoomfit", Title: t("cmd.sort.zoomfit"), Shortcut: shell.Ctrl(fyne.Key0), Run: func() { a.ZoomFit() }},
	}
}
