// Command sanity files PDF receipts into a project's folder structure.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2/app"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/locales"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
	"github.com/72nd/sanity/ui"
	"github.com/72nd/sanity/ui/presenter"
)

// main opens the project folder given as the first argument (default: the
// current directory) or runs on sample data with -demo.
func main() {
	i18n.MustAddFiles(locales.FS, locales.Files...)

	var st store.Store
	root := ""
	if len(os.Args) > 1 && os.Args[1] == "-demo" {
		root, _ = os.MkdirTemp("", "sanity-demo-")
		st = store.NewFake()
	} else {
		if len(os.Args) > 1 {
			root = os.Args[1]
		} else if wd, err := os.Getwd(); err == nil {
			root = wd
		}
		js, err := store.OpenJSON(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, "sanity:", err)
			os.Exit(1)
		}
		st = js
	}

	q := inbox.NewQueue(nil)
	stop, err := inbox.Watch(q, filepath.Join(root, "office", "inbox"), 400*time.Millisecond)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sanity:", err)
		os.Exit(1)
	}
	defer stop()

	lib := presenter.NewLibrary(st, i18n.T)
	sortP := presenter.NewSort(st, q, inbox.NewFiler(root, st), pdf.NewEngine(), i18n.T)
	act := ui.NewSortActions()

	a := app.NewWithID("com.72nd.sanity")
	sh := shell.New(a, shell.Config{
		TitleKey: "app.title",
		Pages:    ui.Pages(lib, sortP, q, act),
		Width:    1280,
		Height:   800,
	})
	for _, c := range ui.Commands(act) {
		if err := sh.Registry.Register(c); err != nil {
			fmt.Fprintln(os.Stderr, "sanity:", err)
		}
	}
	sh.Run()
}
