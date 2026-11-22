// Package ui contains the views. Views only compose kit widgets and forward
// input to presenters; they hold no business logic.
package ui

import (
	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"

	"github.com/72nd/fine/shell"
	"github.com/72nd/sanity/ui/presenter"
)

// Pages returns the navigation of the app. The Library is always the start page.
func Pages(lib *presenter.Library) []shell.Page {
	return []shell.Page{
		{ID: "library", TitleKey: "nav.library", Icon: fynetheme.IconNameFolder,
			Build: func(sh *shell.Shell) fyne.CanvasObject { return LibraryPage(sh, lib) }},
	}
}
