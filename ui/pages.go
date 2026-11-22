// Package ui contains the views. Views only compose kit widgets and forward
// input to presenters; they hold no business logic.
package ui

import (
	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"

	"github.com/72nd/fine/shell"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/ui/presenter"
)

// Pages returns the navigation of the app. The Library is always the start
// page (Cmd+1); Sort is Cmd+2, Types Cmd+3.
func Pages(lib *presenter.Library, sort *presenter.Sort, q *inbox.Queue, act *SortActions, keys *presenter.Keys, types *presenter.Types) []shell.Page {
	lib.InboxCount = q.Len // the hint above the library grid
	return []shell.Page{
		{ID: "library", TitleKey: "nav.library", Icon: fynetheme.IconNameFolder,
			Build:  func(sh *shell.Shell) fyne.CanvasObject { return LibraryPage(sh, lib, act) },
			OnShow: func(sh *shell.Shell) { InstallLibraryVim(sh, act, keys) },
			Focus:  act.Library.Focus},
		// Keep: the form, the zoom and the position in the queue survive a
		// visit to another page.
		{ID: "sort", TitleKey: "nav.sort", Icon: fynetheme.IconNameDocument, Keep: true,
			Build:  func(sh *shell.Shell) fyne.CanvasObject { return SortPage(sh, sort, q, act) },
			OnShow: func(sh *shell.Shell) { act.shown(); InstallVim(sh, act, keys) }},
		{ID: "types", TitleKey: "nav.types", Icon: fynetheme.IconNameSettings,
			Build: func(sh *shell.Shell) fyne.CanvasObject { return TypesPage(sh, types) }},
	}
}
