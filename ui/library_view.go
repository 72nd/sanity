package ui

import (
	"context"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	fynetheme "fyne.io/fyne/v2/theme"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/theme"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/fine/widgets/grid"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/ui/presenter"
)

// libraryGridKey is the preferences key of the library grid state.
const libraryGridKey = "library.grid"

// LibraryPage builds the start page: a hint when documents wait in the inbox,
// a quick filter above the grid of all filed documents, counts and the total
// of the selection in the status bar. The page is rebuilt on every visit (so
// documents filed meanwhile show up); the filter text and the selection live in
// the presenter and come back.
func LibraryPage(sh *shell.Shell, p *presenter.Library, act *SortActions) fyne.CanvasObject {
	la := act.Library
	g := grid.New(libraryColumns(p))
	g.AccessibleLabel = i18n.T("nav.library")
	g.StretchLast = true
	g.Mode = grid.SelectMulti
	g.Key = func(r presenter.Row) string { return r.Entry.ID }
	la.focus = g
	notify := func(st theme.Status, msg string) { sh.Toasts.Notify(st, msg, toastDur(4*time.Second)) }

	// The grid state (widths, sort, filters, conditions, grouping) comes back
	// before the first SetItems, so the view is right at once.
	var state grid.State
	if sh.Prefs.LoadJSON(libraryGridKey, &state) {
		g.SetState(state)
	}
	g.OnStateChanged = func(s grid.State) { sh.Prefs.SaveJSON(libraryGridKey, s) }

	var visual bool // v is active: movement extends the selection
	anchor := 0     // view row where v was pressed
	var own, restoring bool
	status := func() { sh.Status.SetRight(p.SelectionStatus(g.ItemCount(), g.Selected())...) }
	g.OnSelectionChanged = func() {
		if !own {
			visual = false
		}
		if !restoring {
			p.SelKeys = g.SelectedKeys()
		}
		status()
	}

	// selection is the selected rows, or the row under the cursor (none on a
	// group header).
	selection := func() []presenter.Row {
		if rows := g.Selected(); len(rows) > 0 {
			return rows
		}
		if it := g.ItemAt(g.CursorView()); it != nil {
			return []presenter.Row{*it}
		}
		return nil
	}
	// viewRows are the rows as shown: a group header is a zero Row (no entry id).
	viewRows := func() []presenter.Row {
		rows := make([]presenter.Row, g.Len())
		for i := range rows {
			if it := g.ItemAt(i); it != nil {
				rows[i] = *it
			}
		}
		return rows
	}
	cursor := func() int { return max(g.CursorView(), 0) }
	// goTo moves the cursor to view row v; with v active it extends the range
	// from the anchor.
	goTo := func(v int) {
		n := g.Len()
		if n == 0 {
			return
		}
		v = min(max(v, 0), n-1)
		own = true
		defer func() { own = false }()
		if !visual {
			g.SelectView(v)
			g.ScrollTo(v)
			return
		}
		lo, hi, step := anchor, v, 1
		if v < anchor {
			lo, hi, step = v, anchor, -1
		}
		keys := make([]string, 0, hi-lo+1)
		for i := v; i >= lo && i <= hi; i += -step { // cursor end first: SelectKeys puts the cursor there
			if it := g.ItemAt(i); it != nil { // group headers are skipped
				keys = append(keys, g.Key(*it))
			}
		}
		g.SelectKeys(keys)
		g.ScrollTo(v)
	}
	needSelection := func() ([]presenter.Row, bool) {
		rows := selection()
		if len(rows) == 0 {
			notify(theme.StatusWarning, i18n.T("library.nothing.selected"))
		}
		return rows, len(rows) > 0
	}

	condBtn := widgets.NewButton(p.ConditionsLabel(0), widgets.ButtonToolbar, func() { g.ShowConditions(sh.Win) })
	condBtn.SetIcon(fynetheme.IconNameSearch)
	clearBtn := widgets.NewButton(i18n.T("library.filters.clear"), widgets.ButtonToolbar, func() { g.ClearFilters() })
	clearBtn.SetIcon(fynetheme.IconNameCancel)
	// toolbar shows the number of active conditions and the clear button only
	// while a filter is active.
	toolbar := func() {
		condBtn.SetLabel(p.ConditionsLabel(len(g.Conditions().Items)))
		if g.FilterActive() {
			clearBtn.Show()
		} else {
			clearBtn.Hide()
		}
	}
	g.OnViewChanged = func() {
		status()
		toolbar()
	}
	toolbar()

	filter := widgets.NewEntry()
	filter.SetPlaceHolder(i18n.T("library.filter.placeholder"))
	filter.SetText(p.Query)
	filter.OnChanged = func(q string) {
		p.Query = q
		visual = false
		g.SetItems(p.Visible(q))
		status()
	}
	toGrid := func() {
		la.SetMode(true)
		sh.Focus(g)
	}
	filter.OnSubmitted = func(string) { toGrid() }
	filter.OnEscape = func() { la.SetMode(true) }

	// Inbox hint: a line above the filter, shown while documents wait.
	hintText := widgets.NewLabel("", widgets.LabelBody)
	hintBtn := widgets.NewButton(i18n.T("library.inbox.open"), widgets.ButtonPrimary, func() { sh.Show("sort") })
	hintBar := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeBottom, 6, widgets.HBox(hintText, widgets.Spacer(), hintBtn))
	refreshHint := func() {
		if h, ok := p.InboxHint(); ok {
			hintText.Text = h
			hintText.Refresh()
			hintBar.Show()
		} else {
			hintBar.Hide()
		}
	}
	refreshHint()

	loading := widgets.NewEmptyState(fynetheme.IconNameViewRefresh, i18n.T("library.loading.title"), i18n.T("library.loading.desc"), nil)
	g.Hide()
	show := func() {
		loading.Hide()
		g.Show()
		g.SetItems(p.Visible(filter.Text))
		if len(p.SelKeys) > 0 {
			restoring = true
			g.SelectKeys(p.SelKeys)
			restoring = false
		}
		status()
		refreshHint()
	}
	if p.Loaded() {
		show() // show the old data at once, refresh below
	}
	if !p.Fresh() {
		background(func() { // never block the UI thread on the store
			err := p.Load(context.Background())
			fyne.Do(func() {
				if err != nil {
					sh.Toasts.Notify(theme.StatusDanger, i18n.T("toast.load.failed"), toastDur(6*time.Second))
					return
				}
				show()
			})
		})
	}

	// Documents go back to the sort queue; the sort page is built first, since
	// it owns the import.
	p.ImportFn = func(paths ...string) {
		sh.Show("sort")
		act.Import(paths...)
	}

	open := func(r presenter.Row) {
		if err := inbox.Open(p.AbsPath(r)); err != nil {
			notify(theme.StatusDanger, i18n.T("library.open.failed", "Reason", err.Error()))
		}
	}
	g.OnActivated = func(item *presenter.Row) {
		if item != nil {
			open(*item)
		}
	}
	g.OnTypedRune = func(r rune) bool { return la.Rune(r) }

	la.Open = func() {
		if rows, ok := needSelection(); ok {
			open(rows[0])
		}
	}
	copyPaths := func(abs bool) {
		rows, ok := needSelection()
		if !ok {
			return
		}
		visual = false
		setClipboard(sh, p.CopyPaths(rows, abs))
		if len(rows) == 1 {
			notify(theme.StatusInfo, i18n.T("toast.copied", "Path", p.CopyPaths(rows, abs)))
		} else {
			notify(theme.StatusInfo, i18n.T("library.copied", "Count", strconv.Itoa(len(rows))))
		}
	}
	la.CopyRel = func() { copyPaths(false) }
	la.CopyAbs = func() { copyPaths(true) }
	la.Reveal = func() {
		if rows, ok := needSelection(); ok {
			if err := inbox.Reveal(p.RevealPath(rows[0])); err != nil {
				notify(theme.StatusDanger, err.Error())
			}
		}
	}
	la.Resort = func() {
		rows, ok := needSelection()
		if !ok {
			return
		}
		visual = false
		n := len(p.Resort(rows))
		notify(theme.StatusInfo, i18n.T("library.resorted", "Count", strconv.Itoa(n)))
	}
	la.Remove = func() {
		rows, ok := needSelection()
		if !ok {
			return
		}
		widgets.ShowConfirm(sh.Win, i18n.T("library.remove.title"),
			i18n.T("library.remove.message", "Count", strconv.Itoa(len(rows))),
			i18n.T("library.remove.confirm"), true, func(yes bool) {
				if !yes {
					return
				}
				go func() {
					n, err := p.Remove(context.Background(), rows)
					fyne.Do(func() {
						visual = false
						p.SelKeys = nil
						g.SetItems(p.Visible(filter.Text))
						status()
						if err != nil {
							notify(theme.StatusDanger, err.Error())
							return
						}
						notify(theme.StatusSuccess, i18n.T("library.removed", "Count", strconv.Itoa(n)))
					})
				}()
			})
	}
	la.Search = func() {
		la.SetMode(false)
		sh.Focus(filter)
	}
	la.Move = func(delta int) {
		if g.SelectedCount() == 0 { // nothing selected yet: the first move lands on the first row
			goTo(0)
			return
		}
		goTo(cursor() + delta)
	}
	la.Jump = func(first bool) {
		if first {
			goTo(0)
		} else {
			goTo(g.Len() - 1)
		}
	}
	la.Hit = func(forward bool) {
		if j := p.NextHit(viewRows(), cursor(), filter.Text, forward); j >= 0 {
			goTo(j)
		}
	}
	la.FilterColumn = func() {
		// The grid has no getter for the active column; Alt+Down opens the
		// picker there (see FINE-NOTES.md).
		g.KeyDown(&fyne.KeyEvent{Name: desktop.KeyAltLeft})
		g.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
		g.KeyUp(&fyne.KeyEvent{Name: desktop.KeyAltLeft})
	}
	la.Conditions = func() { g.ShowConditions(sh.Win) }
	la.ClearFilters = func() { g.ClearFilters() }
	la.Ungroup = func() { g.SetGroupBy(nil) }
	la.Visual = func(all bool) {
		switch {
		case all:
			visual = false
			own = true
			keys := make([]string, 0, g.Len())
			for i := 0; i < g.Len(); i++ {
				if it := g.ItemAt(i); it != nil {
					keys = append(keys, g.Key(*it))
				}
			}
			g.SelectKeys(keys)
			own = false
		case visual:
			visual = false
		default:
			visual, anchor = true, cursor()
			goTo(anchor)
		}
	}

	g.OnContextMenu = func(item *presenter.Row, ev *fyne.PointEvent) {
		widgets.ShowContextMenu(g, ev.AbsolutePosition, []widgets.MenuItem{
			{Label: i18n.T("library.menu.copy"), Icon: fynetheme.IconNameContentCopy, Action: func() { la.CopyRel() }},
			{Label: i18n.T("library.menu.reveal"), Icon: fynetheme.IconNameFolderOpen, Action: func() { la.Reveal() }},
			{Label: i18n.T("library.menu.resort"), Icon: fynetheme.IconNameViewRefresh, Action: func() { la.Resort() }},
			widgets.MenuSeparator(),
			{Label: i18n.T("library.menu.remove"), Icon: fynetheme.IconNameDelete, Danger: true, Action: func() { la.Remove() }},
		})
	}

	bar := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeBottom, 6,
		widgets.Border(nil, nil, nil, widgets.HBox(condBtn, clearBtn), filter))
	return widgets.Border(widgets.VBox(hintBar, bar), nil, nil, nil, widgets.Stack(g, loading))
}

// setClipboard puts text on the clipboard; tests replace it.
var setClipboard = func(sh *shell.Shell, text string) { sh.App.Clipboard().SetContent(text) }
