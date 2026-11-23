// Command sanity files PDF receipts into a project's folder structure.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2/app"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/theme"
	"github.com/72nd/fine/widgets"
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
	cur     *services // nil while the welcome page is shown
	demo    bool
}

// pages returns the navigation for the current services.
func (a *application) pages() []shell.Page {
	s := a.cur
	if s == nil {
		return []shell.Page{ui.WelcomePageEntry(appIcon(), a.nav)}
	}
	return append(ui.Pages(s.lib, s.sort, s.queue, a.act, a.keys, s.types), ui.ProjectPageEntry(s.project, a.nav))
}

// remember adds the open project to the recent list and shows its name in the
// window title.
func (a *application) remember() {
	if a.cur == nil {
		if a.sh != nil {
			a.sh.Win.SetTitle(i18n.T("app.title"))
		}
		return
	}
	if !a.demo {
		a.recents.Add(a.cur.Root, a.cur.Name)
	}
	if a.sh != nil {
		a.sh.Win.SetTitle(a.cur.Name + " – " + i18n.T("app.title"))
	}
	a.updateToolbar()
}

// updateToolbar shows the project drop-down on the right of the toolbar, or
// nothing on the welcome page.
func (a *application) updateToolbar() {
	if a.sh == nil || a.nav == nil || a.nav.Recents == nil {
		return
	}
	if a.cur == nil {
		a.sh.SetToolbarAppLeft()
		return
	}
	a.sh.SetToolbarAppLeft(ui.ProjectMenu(a.sh, a.nav, a.cur.Root, a.cur.Name))
}

// showWelcome closes the open project and shows the welcome page. Unsaved
// edits on the project page are discarded only on request. UI thread.
func (a *application) showWelcome() {
	if a.cur == nil {
		return
	}
	do := func() {
		a.cur.Close()
		a.cur, a.demo = nil, false
		a.remember()
		a.sh.SetPages(a.pages())
		a.sh.Show("welcome")
	}
	if a.nav.Dirty != nil && a.nav.Dirty() {
		widgets.ShowConfirm(a.sh.Win, i18n.T("project.discard.title"), i18n.T("project.discard.message"),
			i18n.T("project.discard"), true, func(yes bool) {
				if yes {
					do()
				}
			})
		return
	}
	do()
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
	if a.cur != nil {
		a.cur.Close()
	}
	a.cur, a.demo = next, false
	a.remember()
	a.sh.SetPages(a.pages())
	a.sh.Show("library")
}

// main opens the project folder given as the first argument or runs on
// sample data with -demo. Without an argument it shows the welcome page.
func main() {
	if len(os.Args) > 1 && os.Args[1] == "corrections" {
		os.Exit(runCorrections(os.Args[2:], os.Stdout, os.Stderr))
	}
	if len(os.Args) > 2 && os.Args[1] == "types" && os.Args[2] == "check" {
		os.Exit(runTypesCheck(os.Args[3:], os.Stdout, os.Stderr))
	}
	i18n.MustAddFiles(locales.FS, locales.Files...)

	// The Fyne app comes first: the welcome page is built inside shell.New and
	// already lists the recent projects, so they read the preferences directly.
	fyneApp := app.NewWithID("com.72nd.sanity")
	if !inAppBundle() {
		// Inside Sanity.app the Dock takes icon.icns; SetIcon would replace it at
		// startup with the raw PNG, which the Dock draws smaller than its own icons.
		fyneApp.SetIcon(appIcon())
	}
	a := &application{act: ui.NewSortActions(), keys: presenter.NewKeys(nil)}
	a.recents = presenter.NewRecents(shell.NewPrefs(fyneApp.Preferences()))
	root := ""
	if len(os.Args) > 1 && os.Args[1] == "-demo" {
		root, _ = os.MkdirTemp("", "sanity-demo-")
		a.demo = true
	} else if len(os.Args) > 1 {
		root = os.Args[1]
	}
	if root != "" {
		s, err := openServices(root, a.demo, a.act)
		if err != nil {
			fmt.Fprintln(os.Stderr, "sanity:", err)
			os.Exit(1)
		}
		a.cur = s
	}
	defer func() {
		if a.cur != nil {
			a.cur.Close()
		}
	}()

	a.nav = &ui.ProjectNav{
		Recents:  a.recents,
		Switch:   a.switchTo,
		Dirty:    func() bool { return a.cur != nil && a.cur.project.Dirty() },
		Overview: a.showWelcome,
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
		Version:  Version,
	})
	a.sh.SetFooterTrailing(ui.ShortcutsButton(a.sh))
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
		if a.cur == nil { // the welcome page has no sort page
			return
		}
		a.sh.Show("sort") // first: the page owns the import and may not be built yet
		a.act.Import(paths...)
	})
	a.sh.Run()
}

// inAppBundle reports whether the binary runs from a macOS .app bundle.
func inAppBundle() bool {
	exe, err := os.Executable()
	return err == nil && strings.Contains(exe, ".app/Contents/MacOS/")
}
