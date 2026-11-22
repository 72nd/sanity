// Command sanity files PDF receipts into a project's folder structure.
package main

import (
	"fyne.io/fyne/v2/app"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/sanity/locales"
	"github.com/72nd/sanity/store"
	"github.com/72nd/sanity/ui"
	"github.com/72nd/sanity/ui/presenter"
)

func main() {
	i18n.MustAddFiles(locales.FS, locales.Files...)

	lib := presenter.NewLibrary(store.NewFake(), i18n.T)

	a := app.NewWithID("com.72nd.sanity")
	sh := shell.New(a, shell.Config{
		TitleKey: "app.title",
		Pages:    ui.Pages(lib),
		Width:    1200,
		Height:   760,
	})
	sh.Run()
}
