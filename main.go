// Command sanity files PDF receipts into a project's folder structure.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2/app"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/sanity/domain"
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

	engine := pdf.NewEngine()
	lib := presenter.NewLibrary(st, i18n.T)
	learner := inbox.NewLearner(root, st, engine)
	go func() { // fill the learned index from the library in the background
		if err := learner.Rebuild(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, "sanity: learning:", err)
		}
	}()
	filer := inbox.NewFiler(root, st)
	filer.OnFiled = func(e domain.LibraryEntry, text string) {
		if err := learner.Learn(e.TypeID, e.SHA256, text); err != nil {
			fmt.Fprintln(os.Stderr, "sanity: learning:", err)
		}
	}
	filer.OnUndone = func(e domain.LibraryEntry) {
		if err := learner.Forget(e.SHA256); err != nil {
			fmt.Fprintln(os.Stderr, "sanity: learning:", err)
		}
	}
	sortP := presenter.NewSort(st, q, filer, engine, i18n.T)
	act := ui.NewSortActions()
	sortP.SetAnalyzer(&inbox.Analyzer{PDF: engine, Store: st, Learned: learner.Index()})

	a := app.NewWithID("com.72nd.sanity")
	sh := shell.New(a, shell.Config{
		TitleKey: "app.title",
		Pages:    ui.Pages(lib, sortP, q, act, presenter.NewKeys(nil), presenter.NewTypes(st, i18n.T)),
		Width:    1280,
		Height:   800,
		Settings: []shell.SettingsSection{ui.VimSettings()},
	})
	for _, c := range ui.Commands(act) {
		if err := sh.Registry.Register(c); err != nil {
			fmt.Fprintln(os.Stderr, "sanity:", err)
		}
	}
	sh.SetOnDropped(func(paths []string) { // dropped files are shown at once
		act.Import(paths...)
		sh.Show("sort")
	})
	sh.Run()
}
