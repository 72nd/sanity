package ui

import (
	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/sanity/ui/presenter"
)

// modeStatus shows the Vim mode on the left of the status bar, the same text
// as the sort page (empty when the Vim layer is off).
func modeStatus(sh *shell.Shell, normal bool) {
	switch {
	case !VimEnabled(sh):
		sh.Status.SetLeft("")
	case normal:
		sh.Status.SetLeft(i18n.T("vim.mode.normal"))
	default:
		sh.Status.SetLeft(i18n.T("vim.mode.insert"))
	}
}

// InstallLibraryVim sets up the modal layer of the library page; call it from
// Page.OnShow. The grid has the focus, so its OnTypedRune receives the runes
// (it calls LibraryActions.Rune); after Esc in the filter field nothing has the
// focus and the canvas hook feeds the same handler. The filter field is insert
// mode, Enter in it returns to the grid. Enter on a row (open) works without
// the Vim layer, through the grid's OnActivated.
func InstallLibraryVim(sh *shell.Shell, act *SortActions, keys *presenter.Keys) {
	la := act.Library
	la.Shown(sh)
	la.Rune = func(rune) bool { return false }
	la.SetMode = func(normal bool) {
		keys.Reset()
		modeStatus(sh, normal)
	}
	modeStatus(sh, true)
	if !VimEnabled(sh) {
		return
	}

	run := func(cmd string) {
		switch cmd {
		case "row.down":
			la.Move(1)
		case "row.up":
			la.Move(-1)
		case "row.first":
			la.Jump(true)
		case "row.last":
			la.Jump(false)
		case "search":
			la.Search()
		case "hit.next":
			la.Hit(true)
		case "hit.prev":
			la.Hit(false)
		case "select":
			la.Visual(false)
		case "select.rows":
			la.Visual(true)
		case "copy.rel":
			la.CopyRel()
		case "reveal":
			la.Reveal()
		case "resort":
			la.Resort()
		case "open":
			la.Open()
		}
	}
	la.Rune = func(r rune) bool {
		cmd, consumed := keys.Feed(presenter.ContextLibrary, r)
		if cmd != "" {
			run(cmd)
		}
		for _, res := range keys.Drain() {
			if res.Cmd != "" {
				run(res.Cmd)
			}
		}
		return consumed
	}
	sh.SetOnTypedRune(func(r rune) { la.Rune(r) })
}
