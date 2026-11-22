package ui

import (
	"context"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	fynetheme "fyne.io/fyne/v2/theme"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/theme"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/ui/presenter"
)

// ProjectNav connects the project page and the switch command to the app,
// which owns the running services.
type ProjectNav struct {
	Recents *presenter.Recents
	// Switch opens the project at root, creating it when it has no
	// office/sanity.json yet. It runs on the UI thread.
	Switch func(root string)
	// Saved is called after the project was saved; inboxChanged is true when
	// the folder to watch is another one now.
	Saved func(inboxChanged bool)
	// Dirty reports unsaved edits on the project page; nil means none.
	Dirty func() bool
}

// RequestSwitch opens the project at root after the safety questions: unsaved
// edits are discarded only on request, and a folder that holds no project
// asks whether to create one there.
func RequestSwitch(sh *shell.Shell, nav *ProjectNav, root string) {
	create := func() {
		if presenter.HasProject(root) {
			nav.Switch(root)
			return
		}
		widgets.ShowConfirm(sh.Win, i18n.T("project.new.title"), i18n.T("project.new.message", "Folder", root),
			i18n.T("project.new.create"), false, func(yes bool) {
				if yes {
					nav.Switch(root)
				}
			})
	}
	if nav.Dirty != nil && nav.Dirty() {
		widgets.ShowConfirm(sh.Win, i18n.T("project.discard.title"), i18n.T("project.discard.message"),
			i18n.T("project.discard"), true, func(yes bool) {
				if yes {
					create()
				}
			})
		return
	}
	create()
}

// recentLabel is how a recent project is listed: name and folder.
func recentLabel(r presenter.RecentProject) string { return r.Name + "  (" + r.Root + ")" }

// ShowSwitchDialog lets the user pick one of the recent projects and opens it.
func ShowSwitchDialog(sh *shell.Shell, nav *ProjectNav) {
	recents := nav.Recents.List()
	if len(recents) == 0 {
		widgets.ShowInfo(sh.Win, i18n.T("project.switch.title"), i18n.T("project.recent.empty"))
		return
	}
	labels := make([]string, len(recents))
	for i, r := range recents {
		labels[i] = recentLabel(r)
	}
	sel := widgets.NewSelect(labels, nil)
	sel.SetSelected(labels[0])
	row := widgets.NewFormRow(i18n.T("project.switch.field"), sel)
	widgets.ShowForm(sh.Win, i18n.T("project.switch.title"), []*widgets.FormRow{row}, i18n.T("project.open"), func(ok bool) {
		if !ok {
			return
		}
		for i, l := range labels {
			if l == sel.Selected() {
				RequestSwitch(sh, nav, recents[i].Root)
				return
			}
		}
	})
}

// projectWidgets are the fields of the project page, for tests.
type projectWidgets struct {
	name, inbox, ignore *widgets.Entry
	nameRow, inboxRow   *widgets.FormRow
	ignoreRow           *widgets.FormRow
	save, revert        *widgets.Button
	recentBox           *fyne.Container
}

// gap is an empty, fixed-height spacer between stacked blocks.
func gap(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(nil)
	r.SetMinSize(fyne.NewSize(1, h))
	return r
}

// stack lays out blocks top to bottom with h between them.
func stack(h float32, blocks ...fyne.CanvasObject) fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0, 2*len(blocks))
	for i, b := range blocks {
		if i > 0 {
			objs = append(objs, gap(h))
		}
		objs = append(objs, b)
	}
	return widgets.VBox(objs...)
}

// Spacing of the project page: between fields, between cards, and before the
// recent projects, which are a section of their own.
const (
	fieldGap   = 12
	cardGap    = 20
	sectionGap = 40
)

// ProjectPage builds the project page: name, root, inbox, own identifiers and
// the recent projects.
func ProjectPage(sh *shell.Shell, p *presenter.Project, nav *ProjectNav) fyne.CanvasObject {
	obj, _ := projectPage(sh, p, nav)
	return obj
}

func projectPage(sh *shell.Shell, p *presenter.Project, nav *ProjectNav) (fyne.CanvasObject, *projectWidgets) {
	var syncing bool
	notify := func(st theme.Status, msg string) { sh.Toasts.Notify(st, msg, toastDur(5*time.Second)) }

	name := widgets.NewEntry()
	nameRow := widgets.NewFormRow(i18n.T("project.name"), name)
	rootLabel := widgets.NewLabel("", widgets.LabelMono)
	revealBtn := widgets.NewButton(i18n.T("project.root.reveal"), widgets.ButtonSecondary, func() {
		if err := inbox.Reveal(p.Root()); err != nil {
			notify(theme.StatusDanger, i18n.T("project.reveal.failed", "Reason", err.Error()))
		}
	})
	rootRow := widgets.NewFormRow(i18n.T("project.root"), widgets.HBox(rootLabel, widgets.Spacer(), revealBtn))
	inboxEntry := widgets.NewEntry()
	chooseBtn := widgets.NewButton(i18n.T("project.inbox.choose"), widgets.ButtonSecondary, nil)
	inboxRow := widgets.NewFormRow(i18n.T("project.inbox"), widgets.Border(nil, nil, nil, chooseBtn, inboxEntry))
	ignore := widgets.NewTextArea()
	ignore.SetMinRowsVisible(5)
	ignoreRow := widgets.NewFormRow(i18n.T("project.ignore"), ignore)
	hint := widgets.NewLabel(i18n.T("project.ignore.hint"), widgets.LabelSmall)

	state := widgets.NewLabel("", widgets.LabelMuted)
	saveBtn := widgets.NewButton(i18n.T("project.save"), widgets.ButtonPrimary, nil)
	revertBtn := widgets.NewButton(i18n.T("project.revert"), widgets.ButtonSecondary, nil)

	refresh := func() {
		pr := p.Problems()
		nameRow.SetError(pr.Name)
		inboxRow.SetError(pr.Inbox)
		ignoreRow.SetError(pr.Ignore)
		state.SetText(p.StateText())
		if p.Dirty() && !pr.Any() {
			saveBtn.Enable()
		} else {
			saveBtn.Disable()
		}
		if p.Dirty() {
			revertBtn.Enable()
		} else {
			revertBtn.Disable()
		}
	}
	populate := func() {
		syncing = true
		name.SetText(p.Name())
		inboxEntry.SetText(p.Inbox())
		ignore.SetText(p.IgnoreText())
		rootLabel.SetText(p.Root())
		syncing = false
		refresh()
	}
	edited := func(set func(string)) func(string) {
		return func(s string) {
			if !syncing {
				set(s)
				refresh()
			}
		}
	}
	name.OnChanged = edited(p.SetName)
	inboxEntry.OnChanged = edited(p.SetInbox)
	ignore.OnChanged = edited(p.SetIgnoreText)

	chooseBtn.OnTapped = func() {
		widgets.ShowFolderOpen(sh.Win, widgets.FolderDialogOptions{StartDir: p.Root()}, func(dir string) {
			rel, err := p.SetInboxFolder(dir)
			if err != nil {
				notify(theme.StatusDanger, i18n.T("project.inbox.outside"))
				return
			}
			syncing = true
			inboxEntry.SetText(rel)
			syncing = false
			refresh()
		})
	}
	revertBtn.OnTapped = func() { p.Revert(); populate() }
	saveBtn.OnTapped = func() {
		go func() {
			changed, err := p.Save(context.Background())
			fyne.Do(func() {
				if err != nil {
					notify(theme.StatusDanger, i18n.T("project.save.failed", "Reason", err.Error()))
					return
				}
				populate()
				notify(theme.StatusSuccess, i18n.T("project.saved"))
				if nav.Saved != nil {
					nav.Saved(changed)
				}
			})
		}()
	}

	// --- recent projects ---
	recentBox := widgets.VBox().(*fyne.Container)
	fillRecents := func() {
		recentBox.Objects = nil
		list := nav.Recents.List()
		if len(list) == 0 {
			recentBox.Add(widgets.NewLabel(i18n.T("project.recent.empty"), widgets.LabelMuted))
		}
		for _, r := range list {
			root := r.Root
			current := filepath.Clean(root) == filepath.Clean(p.Root())
			open := widgets.NewButton(i18n.T("project.open"), widgets.ButtonSecondary, func() { RequestSwitch(sh, nav, root) })
			if current {
				open.Disable()
			}
			title := widgets.NewLabel(r.Name, widgets.LabelBody)
			title.Bold = true
			text := stack(4, title, widgets.NewLabel(root, widgets.LabelMono))
			if recentBox.Objects != nil {
				recentBox.Add(gap(8))
			}
			recentBox.Add(widgets.NewCard("", "", widgets.HBox(text, widgets.Spacer(), open)))
		}
		recentBox.Refresh()
	}
	openOther := widgets.NewButton(i18n.T("project.open.other"), widgets.ButtonSecondary, func() {
		start := filepath.Dir(p.Root())
		widgets.ShowFolderOpen(sh.Win, widgets.FolderDialogOptions{StartDir: start}, func(dir string) { RequestSwitch(sh, nav, dir) })
	})

	basics := widgets.NewCard(i18n.T("project.card.basics"), "", stack(fieldGap, nameRow, rootRow, inboxRow))
	idents := widgets.NewCard(i18n.T("project.card.ignore"), i18n.T("project.card.ignore.desc"), stack(6, ignoreRow, hint))
	recent := stack(fieldGap, widgets.NewLabel(i18n.T("project.card.recent"), widgets.LabelHeading), recentBox, widgets.HBox(openOther))
	body := widgets.Scroll(widgets.PagePadded(widgets.MaxWidth(720,
		stack(cardGap, basics, idents, gap(sectionGap-cardGap), recent))))
	bar := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeBottom, 6,
		widgets.HBox(state, widgets.Spacer(), revertBtn, saveBtn))
	page := widgets.Border(bar, nil, nil, nil, body)

	background(func() {
		var err error
		if !p.Dirty() {
			err = p.Load(context.Background())
		}
		fyne.Do(func() {
			if err != nil {
				notify(theme.StatusDanger, i18n.T("toast.load.failed"))
			}
			populate()
			fillRecents()
		})
	})
	return page, &projectWidgets{name: name, inbox: inboxEntry, ignore: ignore, nameRow: nameRow, inboxRow: inboxRow,
		ignoreRow: ignoreRow, save: saveBtn, revert: revertBtn, recentBox: recentBox}
}

// ProjectPageEntry is the fourth page (after Types): root, inbox, own
// identifiers and the recent projects.
func ProjectPageEntry(p *presenter.Project, nav *ProjectNav) shell.Page {
	return shell.Page{ID: "project", TitleKey: "nav.project", Icon: fynetheme.IconNameHome,
		Build: func(sh *shell.Shell) fyne.CanvasObject { return ProjectPage(sh, p, nav) }}
}

// ProjectCommands returns the command "project.switch" (Cmd+Shift+P), which
// opens the picker of recent projects.
func ProjectCommands(sh *shell.Shell, nav *ProjectNav) []shell.Command {
	return []shell.Command{{
		ID:    "project.switch",
		Title: func() string { return i18n.T("cmd.project.switch") },
		Shortcut: &desktop.CustomShortcut{KeyName: fyne.KeyP,
			Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift},
		Run: func() { ShowSwitchDialog(sh, nav) },
	}}
}
