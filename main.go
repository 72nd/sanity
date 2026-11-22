// Command sanity files PDF receipts into a project's folder structure.
package main

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2/app"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/theme"
	"github.com/72nd/sanity/locales"
	"github.com/72nd/sanity/ui"
	"github.com/72nd/sanity/ui/presenter"
)

// application owns the window and the services of the open project. Opening
// another project replaces the services and rebuilds the pages.
type application struct {
	sh      *shell.Shell
	act     *ui.SortActions // outlive the services: the commands are registered once
	keys    *presenter.Keys
	recents *presenter.Recents
	nav     *ui.ProjectNav
	cur     *services
	demo    bool
}

// pages returns the navigation for the current services.
func (a *application) pages() []shell.Page {
	s := a.cur
	return append(ui.Pages(s.lib, s.sort, s.queue, a.act, a.keys, s.types), ui.ProjectPageEntry(s.project, a.nav))
}

// remember adds the open project to the recent list and shows its name in the
// window title.
func (a *application) remember() {
	if !a.demo {
		a.recents.Add(a.cur.Root, a.cur.Name)
	}
	if a.sh != nil {
		a.sh.Win.SetTitle(a.cur.Name + " – " + i18n.T("app.title"))
	}
}

// switchTo opens the project at root (creating it when it has none), stops
// the services of the old one and rebuilds the pages. Call it on the UI thread.
// On failure the old project stays open.
func (a *application) switchTo(root string) {
	next, err := openServices(root, false, a.act)
	if err != nil {
		a.sh.Toasts.Notify(theme.StatusDanger, i18n.T("project.open.failed", "Reason", err.Error()), 5*time.Second)
		return
	}
	a.cur.Close()
	a.cur, a.demo = next, false
	a.remember()
	a.sh.SetPages(a.pages())
	a.sh.Show("library")
}

// main opens the project folder given as the first argument (default: the
// current directory) or runs on sample data with -demo.
func main() {
	if len(os.Args) > 2 && os.Args[1] == "types" && os.Args[2] == "check" {
		os.Exit(runTypesCheck(os.Args[3:], os.Stdout, os.Stderr))
	}
	i18n.MustAddFiles(locales.FS, locales.Files...)

	a := &application{act: ui.NewSortActions(), keys: presenter.NewKeys(nil)}
	root := ""
	if len(os.Args) > 1 && os.Args[1] == "-demo" {
		root, _ = os.MkdirTemp("", "sanity-demo-")
		a.demo = true
	} else if len(os.Args) > 1 {
		root = os.Args[1]
	} else if wd, err := os.Getwd(); err == nil {
		root = wd
	}
	s, err := openServices(root, a.demo, a.act)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sanity:", err)
		os.Exit(1)
	}
	a.cur = s
	defer func() { a.cur.Close() }()

	fyneApp := app.NewWithID("com.72nd.sanity")
	fyneApp.SetIcon(appIcon())
	a.nav = &ui.ProjectNav{
		Switch: a.switchTo,
		Dirty:  func() bool { return a.cur.project.Dirty() },
		Saved: func(inboxChanged bool) {
			a.cur.Name = a.cur.project.Name()
			a.remember()
			if inboxChanged && !a.demo {
				a.switchTo(a.cur.Root) // the inbox watcher follows the new folder
			}
		},
	}
	a.sh = shell.New(fyneApp, shell.Config{
		TitleKey: "app.title",
		Pages:    a.pages(),
		Width:    1280,
		Height:   800,
		Settings: []shell.SettingsSection{ui.VimSettings()},
	})
	a.recents = presenter.NewRecents(a.sh.Prefs)
	a.nav.Recents = a.recents
	a.remember()
	for _, c := range append(append(ui.Commands(a.act), ui.ProjectCommands(a.sh, a.nav)...), ui.TypesCommands(ui.SharedTypesActions().Attach(a.sh))...) {
		if err := a.sh.Registry.Register(c); err != nil {
			fmt.Fprintln(os.Stderr, "sanity:", err)
		}
	}
	for _, c := range ui.ShortcutsCommands(a.sh) {
		if err := a.sh.Registry.Register(c); err != nil {
			fmt.Fprintln(os.Stderr, "sanity:", err)
		}
	}
	a.sh.SetOnDropped(func(paths []string) { // dropped files are shown at once
		a.act.Import(paths...)
		a.sh.Show("sort")
	})
	a.sh.Run()
}
