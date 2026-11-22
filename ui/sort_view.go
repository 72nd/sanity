package ui

import (
	"context"
	"io"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/theme"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/fine/widgets/grid"
	"github.com/72nd/sanity/clipboard"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/ui/presenter"
)

// clipboardFiles reads the files Finder copied; tests replace it.
var clipboardFiles = clipboard.FilePaths

func queueColumns() []grid.Column[inbox.Item] {
	return []grid.Column[inbox.Item]{
		{ID: "name", Title: i18n.T("col.file"), Width: 220,
			Value: func(it inbox.Item) string { return it.Name },
			Less:  func(a, b inbox.Item) bool { return a.Name < b.Name }},
	}
}

// sortWidgets are the widgets of the sort page that commands and the Vim layer
// reach into.
type sortWidgets struct {
	queue   *grid.Grid[inbox.Item]
	preview *widgets.ImageView
	picker  *widgets.Picker
	date    *widgets.DateEntry
	invoice *widgets.Entry
	amount  *widgets.Entry
}

// SortPage builds the work screen: queue, PDF preview and the sort form.
func SortPage(sh *shell.Shell, p *presenter.Sort, q *inbox.Queue, act *SortActions) fyne.CanvasObject {
	w := &sortWidgets{
		queue:   grid.New(queueColumns()),
		preview: widgets.NewImageView(),
		picker:  widgets.NewPicker(nil),
		date:    widgets.NewDateEntry(),
		invoice: widgets.NewEntry(),
		amount:  widgets.NewEntry(),
	}
	w.queue.AccessibleLabel = i18n.T("nav.sort")
	w.queue.StretchLast = true
	w.picker.PlaceHolder = i18n.T("sort.type.placeholder")
	pathLabel := widgets.NewLabel("", widgets.LabelMuted)
	hint := widgets.NewLabel("", widgets.LabelMuted)
	dupLabel := widgets.NewLabel("", widgets.LabelBody)
	dupLabel.Hide()

	var syncing bool // true while code, not the user, fills the fields
	shownID := ""    // the document whose preview is on screen

	notify := func(st theme.Status, msg string) { sh.Toasts.Notify(st, msg, 4*time.Second) }

	refreshForm := func() {
		if rel, ok := p.PathPreview(); ok {
			pathLabel.Text, hint.Text = "→ "+rel, i18n.T("sort.hint.file")
		} else {
			pathLabel.Text, hint.Text = "", ""
			if m := p.Missing(); len(m) > 0 {
				hint.Text = i18n.T("sort.hint.missing", "Fields", joinComma(m))
			}
		}
		if e, why, ok := p.Duplicate(); ok {
			dupLabel.Text = i18n.T("sort.duplicate", "Path", e.Path, "Reason", why)
			dupLabel.Show()
		} else {
			dupLabel.Hide()
		}
		pathLabel.Refresh()
		hint.Refresh()
		dupLabel.Refresh()
	}

	setText := func(e *widgets.Entry, s string) {
		if e.Text != s {
			e.SetText(s)
		}
	}
	// syncFields pushes the presenter's form (including analysis prefill) into
	// the widgets without triggering their callbacks.
	syncFields := func() {
		syncing = true
		items := make([]widgets.PickerItem, 0, len(p.Types()))
		for _, c := range p.TypeChoices() {
			it := widgets.PickerItem{ID: c.ID, Label: c.Name, Score: c.Score}
			if c.Confident {
				it.Badge, it.BadgeStatus = i18n.T("sort.badge.suggested"), theme.StatusSuccess
			}
			items = append(items, it)
		}
		w.picker.SetItems(items)
		f := p.Form()
		w.picker.Preselect(f.TypeID)
		var cands []widgets.DateCandidate
		for _, d := range p.DateChoices() {
			label := d.Label
			if d.Fallback {
				label = i18n.T("sort.date.fallback")
			}
			cands = append(cands, widgets.DateCandidate{Date: d.Date, Label: label})
		}
		w.date.SetCandidates(cands)
		if f.Date.IsZero() {
			w.date.SetDate(nil)
		} else {
			d := f.Date
			w.date.SetDate(&d)
		}
		setText(w.invoice, f.Invoice)
		setText(w.amount, f.Amount)
		syncing = false
		refreshForm()
	}

	saveView := func() {
		off := w.preview.ScrollOffset()
		v := p.View(shownID) // keeps the page
		v.Zoom, v.X, v.Y = w.preview.Zoom(), float64(off.X), float64(off.Y)
		p.SaveView(shownID, v)
	}
	zoomLabel := widgets.NewLabel("100 %", widgets.LabelMuted)
	pageLabel := widgets.NewLabel("", widgets.LabelMuted)
	showZoom := func() {
		zoomLabel.Text = strconv.Itoa(int(w.preview.Zoom()*100+0.5)) + " %"
		zoomLabel.Refresh()
		page, total := p.Page(shownID)
		pageLabel.Text = ""
		if total > 1 {
			pageLabel.Text = i18n.T("sort.page", "Cur", strconv.Itoa(page+1), "Total", strconv.Itoa(total))
		}
		pageLabel.Refresh()
	}
	w.preview.OnZoomChanged = func(float64) { showZoom() }

	loadPreview := func() {
		cur, ok := p.Current()
		if !ok {
			w.preview.SetImage(nil)
			w.preview.SetLoading(false)
			return
		}
		w.preview.SetLoading(true)
		go func() { // rendering never blocks the UI thread
			page, _ := p.Page(cur.ID)
			img, pages, err := p.Preview(cur.Path, page)
			fyne.Do(func() {
				if c, ok := p.Current(); !ok || c.ID != cur.ID {
					return // moved on meanwhile
				}
				w.preview.SetLoading(false)
				if err != nil {
					notify(theme.StatusDanger, err.Error())
					return
				}
				p.SetPageCount(cur.ID, pages)
				w.preview.SetImage(img)
				v := p.View(cur.ID) // zoom and scroll are remembered per document
				w.preview.SetZoom(v.Zoom)
				w.preview.SetScrollOffset(fyne.NewPos(float32(v.X), float32(v.Y)))
				showZoom()
			})
		}()
	}

	analyse := func() {
		p.Analyse(context.Background(), func(id string) {
			fyne.Do(func() {
				if cur, ok := p.Current(); ok && cur.ID == id {
					p.Prefill()
					syncFields()
				}
				w.queue.SetItems(p.Items())
			})
		})
	}

	// showDoc switches the page to the current document.
	showDoc := func() {
		saveView()
		cur, ok := p.Current()
		shownID = ""
		if ok {
			shownID = cur.ID
		}
		syncFields()
		loadPreview()
		items := p.Items()
		w.queue.SetItems(items)
		sh.Status.SetRight(i18n.T("sort.status.open", "Count", strconv.Itoa(len(items))))
		if ok {
			act.SetMode(false)
			w.picker.Focus()
		}
		analyse()
	}

	// Keyboard flow: Enter accepts a field and moves to the next empty one.
	focusNext := func(from string) {
		order := []string{"type", "date", "invoice", "amount"}
		start := 0
		for i, n := range order {
			if n == from {
				start = i + 1
			}
		}
		f := p.Form()
		for _, n := range order[start:] {
			switch {
			case n == "date" && f.Date.IsZero():
				w.date.Focus()
				return
			case n == "invoice" && f.Invoice == "":
				sh.Focus(w.invoice)
				return
			case n == "amount" && f.Amount == "":
				sh.Focus(w.amount)
				return
			}
		}
	}
	normal := func() { act.SetMode(true) }
	w.picker.OnEscape, w.date.OnEscape, w.invoice.OnEscape, w.amount.OnEscape = normal, normal, normal, normal
	w.picker.OnChanged = func(id string) {
		if !syncing {
			p.SetType(id)
			refreshForm()
		}
	}
	w.picker.OnCommitted = func(id string) {
		p.SetType(id)
		refreshForm()
		focusNext("type")
	}
	w.date.OnChanged = func(t *time.Time) {
		if syncing {
			return
		}
		if t != nil {
			p.SetDate(*t)
		}
		refreshForm()
	}
	w.date.OnSubmitted = func(*time.Time) { focusNext("date") }
	w.invoice.OnChanged = func(s string) {
		if !syncing {
			p.SetInvoice(s)
			refreshForm()
		}
	}
	w.invoice.OnSubmitted = func(string) { focusNext("invoice") }
	w.amount.OnChanged = func(s string) {
		if !syncing {
			p.SetAmount(s)
			refreshForm()
		}
	}
	w.queue.OnActivated = func(it *inbox.Item) {
		if it != nil && p.Open(it.ID) {
			showDoc()
		}
	}

	act.File = func() {
		if !p.CanFile() {
			notify(theme.StatusWarning, i18n.T("error.sort.incomplete"))
			return
		}
		saveView()
		go func() {
			e, err := p.File(context.Background())
			fyne.Do(func() {
				if err != nil {
					notify(theme.StatusDanger, err.Error())
					return
				}
				notify(theme.StatusSuccess, i18n.T("toast.filed", "Path", e.Path))
				showDoc()
			})
		}()
	}
	act.Undo = func() {
		if !p.CanUndo() {
			return
		}
		go func() {
			e, err := p.Undo(context.Background())
			fyne.Do(func() {
				if err != nil {
					notify(theme.StatusDanger, err.Error())
					return
				}
				notify(theme.StatusInfo, i18n.T("toast.undone", "Path", e.Path))
				if _, ok := p.Current(); !ok {
					p.Step(0)
				}
				showDoc()
			})
		}()
	}
	act.Skip = func() { p.Step(1); showDoc() }
	act.SkipBack = func() { p.Step(-1); showDoc() }
	act.Import = func(paths ...string) {
		if p.Import(paths...) {
			showDoc()
		}
	}
	act.PasteFiles = func() {
		paths := clipboardFiles()
		if len(paths) == 0 {
			notify(theme.StatusWarning, i18n.T("toast.paste.none"))
			return
		}
		before := p.Count()
		act.Import(paths...)
		if n := p.Count() - before; n > 0 {
			notify(theme.StatusInfo, i18n.T("toast.pasted", "Count", strconv.Itoa(n)))
		} else {
			notify(theme.StatusWarning, i18n.T("toast.paste.none"))
		}
	}
	act.AddFiles = func() {
		widgets.ShowFileOpen(sh.Win, widgets.FileDialogOptions{Extensions: []string{".pdf"}}, func(r io.ReadCloser, path string) {
			r.Close()
			act.Import(path)
		})
	}
	act.ZoomIn, act.ZoomOut, act.ZoomFit = w.preview.ZoomIn, w.preview.ZoomOut, w.preview.FitWidth
	step := func(delta int) {
		saveView() // so the zoom is current when the page changes
		if p.PageStep(delta) {
			loadPreview()
		}
	}
	act.PageNext, act.PagePrev = func() { step(1) }, func() { step(-1) }
	act.ScrollDown = func() { w.preview.ScrollHalfPage(true) }
	act.ScrollUp = func() { w.preview.ScrollHalfPage(false) }
	act.Focus = func(field string) {
		act.SetMode(false)
		switch field {
		case "type":
			w.picker.Focus()
		case "date":
			w.date.Focus()
		default:
			focusNext("")
		}
	}
	act.Search = func() { act.SetMode(false); w.picker.Focus() }
	act.First = func() {
		if p.Jump(true) {
			showDoc()
		}
	}
	act.Last = func() {
		if p.Jump(false) {
			showDoc()
		}
	}
	act.CopyRel = func() { copyPath(sh, p, false, notify) }
	act.CopyAbs = func() { copyPath(sh, p, true, notify) }
	act.Reveal = func() {
		if path, ok := p.RevealPath(); ok {
			if err := inbox.Reveal(path); err != nil {
				notify(theme.StatusDanger, err.Error())
			}
		}
	}
	act.Remove = func() {
		cur, ok := p.Current()
		if !ok {
			return
		}
		widgets.ShowConfirm(sh.Win, i18n.T("sort.remove.title"), i18n.T("sort.remove.message", "Name", cur.Name),
			i18n.T("sort.remove.confirm"), true, func(yes bool) {
				if !yes {
					return
				}
				go func() {
					err := p.Remove()
					fyne.Do(func() {
						if err != nil {
							notify(theme.StatusDanger, err.Error())
							return
						}
						showDoc()
					})
				}()
			})
	}

	fileBtn := widgets.NewButton(i18n.T("sort.file"), widgets.ButtonPrimary, func() { act.File() })
	skipBtn := widgets.NewButton(i18n.T("sort.skip"), widgets.ButtonSecondary, func() { act.Skip() })
	addBtn := widgets.NewButton(i18n.T("sort.add"), widgets.ButtonSecondary, func() { act.AddFiles() })

	form := widgets.VBox(
		dupLabel,
		widgets.NewFormRow(i18n.T("sort.type"), w.picker),
		widgets.NewFormRow(i18n.T("sort.date"), w.date),
		widgets.NewFormRow(i18n.T("sort.invoice"), w.invoice),
		widgets.NewFormRow(i18n.T("sort.amount"), w.amount),
		pathLabel, hint,
		widgets.HBox(fileBtn, skipBtn),
	)
	side := widgets.Border(nil, widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeTop, 6, addBtn), nil, nil, w.queue)
	right := widgets.Scroll(widgets.PagePadded(widgets.NewCard(i18n.T("sort.form.title"), "", form)))

	zoomBtn := func(icon fyne.ThemeIconName, fn func()) *widgets.Button {
		b := widgets.NewButton("", widgets.ButtonToolbar, fn)
		b.SetIcon(icon)
		return b
	}
	pageBtn := zoomBtn
	zoomBar := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeBottom, 4, widgets.HBox(
		zoomBtn(fynetheme.IconNameContentRemove, func() { act.ZoomOut() }),
		zoomLabel,
		zoomBtn(fynetheme.IconNameContentAdd, func() { act.ZoomIn() }),
		widgets.NewButton(i18n.T("sort.zoom.fit"), widgets.ButtonToolbar, func() { act.ZoomFit() }),
		pageBtn(fynetheme.IconNameNavigateBack, func() { act.PagePrev() }),
		pageLabel,
		pageBtn(fynetheme.IconNameNavigateNext, func() { act.PageNext() }),
	))
	main := widgets.HSplit(widgets.Border(zoomBar, nil, nil, nil, w.preview), right)
	main.SetOffset(0.66)
	main.SetMinLeading(280)  // preview
	main.SetMinTrailing(320) // form card: never clipped, the preview shrinks first
	root := widgets.HSplit(side, main)
	root.SetOffset(0.17)
	root.SetMinLeading(180) // queue

	act.shown = func() { // the shell calls this on every show of the page
		if cur, ok := p.Current(); ok && shownID != cur.ID {
			showDoc()
		}
	}
	act.fields = w
	go func() {
		err := p.LoadTypes(context.Background())
		fyne.Do(func() {
			if err != nil {
				notify(theme.StatusDanger, i18n.T("toast.load.failed"))
				return
			}
			p.Step(0)
			showDoc()
		})
	}()
	q.SetOnChange(func() {
		fyne.Do(func() {
			if _, ok := p.Current(); !ok {
				p.Step(0)
				showDoc()
				return
			}
			w.queue.SetItems(p.Items())
			analyse()
		})
	})
	return root
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}

// copyPath puts the target path of the current (or last filed) document on the
// clipboard.
func copyPath(sh *shell.Shell, p *presenter.Sort, abs bool, notify func(theme.Status, string)) {
	path, ok := p.CopyPath(abs)
	if !ok {
		notify(theme.StatusWarning, i18n.T("error.sort.incomplete"))
		return
	}
	sh.App.Clipboard().SetContent(path)
	notify(theme.StatusInfo, i18n.T("toast.copied", "Path", path))
}
