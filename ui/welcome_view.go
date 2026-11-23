package ui

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	fynetheme "fyne.io/fyne/v2/theme"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/sanity/ui/presenter"
)

// welcomeIconSize is the edge of the large app icon on the welcome page.
const welcomeIconSize = 128

// WelcomePageEntry is the only page while no project is open.
func WelcomePageEntry(icon fyne.Resource, nav *ProjectNav) shell.Page {
	return shell.Page{ID: "welcome", TitleKey: "welcome.nav", Icon: fynetheme.IconNameHome,
		Build: func(sh *shell.Shell) fyne.CanvasObject { return WelcomePage(sh, icon, nav) }}
}

// WelcomePage shows the app icon, the recent projects and the buttons to
// create or open a project.
func WelcomePage(sh *shell.Shell, icon fyne.Resource, nav *ProjectNav) fyne.CanvasObject {
	img := canvas.NewImageFromResource(icon)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(welcomeIconSize, welcomeIconSize))

	newBtn := widgets.NewButton(i18n.T("welcome.new"), widgets.ButtonPrimary, func() { NewProjectDialog(sh, nav, "") })
	openBtn := widgets.NewButton(i18n.T("welcome.open"), widgets.ButtonSecondary, func() { OpenProjectDialog(sh, nav, "") })

	recentBox := widgets.VBox().(*fyne.Container)
	list := nav.Recents.List()
	if len(list) == 0 {
		recentBox.Add(widgets.NewLabel(i18n.T("welcome.recent.empty"), widgets.LabelMuted))
	}
	for _, r := range list {
		recentBox.Add(recentRow(sh, nav, r))
	}

	head := widgets.NewLabel(i18n.T("app.title"), widgets.LabelHeading)
	sub := widgets.NewLabel(i18n.T("welcome.subtitle"), widgets.LabelMuted)
	body := stack(fieldGap,
		widgets.HBox(widgets.Spacer(), img, widgets.Spacer()),
		widgets.HBox(widgets.Spacer(), head, widgets.Spacer()),
		widgets.HBox(widgets.Spacer(), sub, widgets.Spacer()),
		gap(cardGap),
		widgets.HBox(widgets.Spacer(), newBtn, openBtn, widgets.Spacer()),
		gap(cardGap),
		widgets.NewLabel(i18n.T("welcome.recent"), widgets.LabelHeading),
		recentBox)
	return widgets.Scroll(widgets.PagePadded(widgets.MaxWidth(560, body)))
}

// recentRow is one clickable recent project: name and folder.
func recentRow(sh *shell.Shell, nav *ProjectNav, r presenter.RecentProject) fyne.CanvasObject {
	root := r.Root
	open := widgets.NewButton(i18n.T("project.open"), widgets.ButtonSecondary, func() { RequestSwitch(sh, nav, root) })
	title := widgets.NewLabel(r.Name, widgets.LabelBody)
	title.Bold = true
	text := stack(4, title, widgets.NewLabel(root, widgets.LabelMono))
	return widgets.NewCard("", "", widgets.HBox(text, widgets.Spacer(), open))
}

// OpenProjectDialog asks for a folder and opens the project there (after the
// usual questions, see RequestSwitch). start is the folder the dialog shows
// first; empty means the folder above the project that is open.
func OpenProjectDialog(sh *shell.Shell, nav *ProjectNav, start string) {
	widgets.ShowFolderOpen(sh.Win, widgets.FolderDialogOptions{StartDir: start}, func(dir string) { RequestSwitch(sh, nav, dir) })
}

// NewProjectDialog asks for a folder and creates a project there without the
// extra question; a folder that already has a project is simply opened.
func NewProjectDialog(sh *shell.Shell, nav *ProjectNav, start string) {
	widgets.ShowFolderOpen(sh.Win, widgets.FolderDialogOptions{StartDir: start}, func(dir string) {
		if presenter.HasProject(dir) || (nav.Dirty != nil && nav.Dirty()) {
			RequestSwitch(sh, nav, dir)
			return
		}
		nav.Switch(filepath.Clean(dir))
	})
}

// ProjectMenu builds the drop-down for the toolbar: the open project, the
// other recent projects, "open project" and "overview". cur names the open
// project. Build it again after the project or the recent list changed.
func ProjectMenu(sh *shell.Shell, nav *ProjectNav, root, name string) *widgets.Select {
	entries := nav.Recents.Menu(root, name, i18n.T)
	labels := make([]string, len(entries))
	for i, e := range entries {
		labels[i] = e.Label
	}
	sel := widgets.NewSelect(labels, nil)
	sel.SetSelectedIndex(0)
	sel.OnPicked = func(label string) {
		for i, e := range entries {
			if e.Label != label || i == 0 {
				continue
			}
			switch e.Kind {
			case presenter.MenuRecent:
				RequestSwitch(sh, nav, e.Root)
			case presenter.MenuOpen:
				OpenProjectDialog(sh, nav, filepath.Dir(root))
			case presenter.MenuOverview:
				if nav.Overview != nil {
					nav.Overview()
				}
			}
			break
		}
		sel.SetSelectedIndex(0) // the field always shows the open project
	}
	return sel
}
