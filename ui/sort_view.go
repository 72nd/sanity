package ui

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/theme"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/fine/widgets/grid"
	"github.com/72nd/sanity/clipboard"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/ui/presenter"
)

// clipboardFiles reads the files Finder copied; tests replace it.
var clipboardFiles = clipboard.FilePaths

// queueColumns marks items that already lie in the project folder with a
// folder icon.
func queueColumns(inProject func(inbox.Item) bool) []grid.Column[inbox.Item] {
	return []grid.Column[inbox.Item]{
		{ID: "name", Title: i18n.T("col.file"), Width: 220,
			Icon: func(it inbox.Item) fyne.ThemeIconName {
				if inProject(it) {
					return fynetheme.IconNameFolder
				}
				return ""
			},
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
	// step moves the keyboard focus from the named field ("type", "date",
	// "invoice", "amount" or "text:<label>") by delta fields along the
	// chain; it stops at both ends.
	step func(from string, delta int)
	// panel is the right-hand side (form, batch plan, actions).
	panel *sortPanel
}

// SortPage builds the work screen: queue, PDF preview and the sort form.
func SortPage(sh *shell.Shell, p *presenter.Sort, q *inbox.Queue, act *SortActions) fyne.CanvasObject {
	w := &sortWidgets{
		queue:   grid.New(queueColumns(p.InProject)),
		preview: widgets.NewImageView(),
		picker:  widgets.NewPicker(nil),
		date:    widgets.NewDateEntry(),
		invoice: widgets.NewEntry(),
		amount:  widgets.NewEntry(),
	}
	w.queue.AccessibleLabel = i18n.T("nav.sort")
	w.queue.StretchLast = true
	w.queue.Mode = grid.SelectMulti // Shift/Cmd-click or Cmd+A select a batch
	w.queue.Key = func(it inbox.Item) string { return it.ID }
	w.picker.PlaceHolder = i18n.T("sort.type.placeholder")
	var refreshBatch func() // the panel's; analysis results call it
	var pnl *sortPanel

	var syncing bool // true while code, not the user, fills the fields

	// Text rows: one entry per {text:<label>} placeholder of the selected
	// type. The container is rebuilt when the labels change; the entries are
	// kept per label so a typed text and the focus survive.
	textBox := widgets.VBox()
	textEntries := map[string]*widgets.Entry{}
	textKey := "" // labels of the rows on screen
	var focusNext func(from string)
	var refreshForm func()
	textEntry := func(label string) *widgets.Entry {
		if e, ok := textEntries[label]; ok {
			return e
		}
		e := widgets.NewEntry()
		e.OnChanged = func(s string) {
			if !syncing {
				p.SetText(label, s)
				refreshForm()
			}
		}
		e.OnSubmitted = func(string) { w.step("text:"+label, 1) }
		e.OnSubmitBack = func() { w.step("text:"+label, -1) }
		e.OnEscape = func() { act.SetMode(true) }
		textEntries[label] = e
		return e
	}
	syncTexts := func() {
		labels := p.TextFields()
		key := strings.Join(labels, "\x00")
		if key != textKey {
			textKey = key
			rows := make([]fyne.CanvasObject, 0, len(labels))
			for _, l := range labels {
				rows = append(rows, widgets.NewFormRow(l, textEntry(l)))
			}
			textBox.(*fyne.Container).Objects = rows
			textBox.Refresh()
		}
		f := p.Form()
		for _, l := range labels {
			if e := textEntry(l); e.Text != f.Text[l] {
				e.SetText(f.Text[l])
			}
		}
	}
	shownID := "" // the document whose preview is on screen

	notify := func(st theme.Status, msg string) { sh.Toasts.Notify(st, msg, toastDur(4*time.Second)) }

	setText := func(e *widgets.Entry, s string) {
		if e.Text != s {
			e.SetText(s)
		}
	}
	// syncDate pushes the form date into the date widget; call it with syncing set.
	syncDate := func() {
		if f := p.Form(); f.Date.IsZero() {
			w.date.SetDate(nil)
		} else {
			d := f.Date
			w.date.SetDate(&d)
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
		syncDate()
		setText(w.invoice, f.Invoice)
		setText(w.amount, f.Amount)
		syncTexts()
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
	shownScale := 0.0 // render scale of the image on screen
	var rerenderTimer *time.Timer
	scheduleRerender := func() {} // set below; sharper render after the zoom settled
	w.preview.OnZoomChanged = func(float64) { showZoom(); scheduleRerender() }

	loadPreview := func() {
		cur, ok := p.Current()
		if !ok {
			w.preview.SetImage(nil)
			w.preview.SetLoading(false)
			return
		}
		w.preview.SetLoading(true)
		page, _ := p.Page(cur.ID)
		// sharp at once for a remembered zoom
		scale := p.PreviewScale(p.View(cur.ID).Zoom)
		go func() { // rendering never blocks the UI thread
			img, pages, err := p.PreviewAt(cur.Path, page, scale)
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
				shownScale = p.CachedScale(cur.Path, page)
				w.preview.SetImage(img)
				v := p.View(cur.ID) // zoom and scroll are remembered per document
				w.preview.SetZoom(v.Zoom)
				w.preview.SetScrollOffset(fyne.NewPos(float32(v.X), float32(v.Y)))
				showZoom()
				p.PreviewShown(cur.ID)
				p.Prefetch(context.Background()) // the next document renders while this one is read
			})
		}()
	}
	// rerender swaps in a sharper image after zooming. The cut is the same page
	// fraction at every scale, so the zoom and the scroll offset (display
	// pixels of the fit-width view) stay valid and the same part stays visible.
	rerender := func() {
		cur, ok := p.Current()
		page, _ := p.Page(cur.ID)
		scale := p.PreviewScale(w.preview.Zoom())
		if !ok || cur.ID != shownID || scale <= shownScale*1.01 {
			return
		}
		go func() {
			img, _, err := p.PreviewAt(cur.Path, page, scale)
			fyne.Do(func() {
				if c, ok := p.Current(); err != nil || !ok || c.ID != cur.ID || c.ID != shownID {
					return
				}
				if pg, _ := p.Page(cur.ID); pg != page {
					return
				}
				zoom, off := w.preview.Zoom(), w.preview.ScrollOffset()
				shownScale = p.CachedScale(cur.Path, page)
				w.preview.SetImage(img)
				w.preview.SetZoom(zoom)
				w.preview.SetScrollOffset(off)
			})
		}()
	}
	scheduleRerender = func() {
		if rerenderTimer != nil {
			rerenderTimer.Stop()
		}
		rerenderTimer = time.AfterFunc(300*time.Millisecond, func() { fyne.Do(rerender) })
	}

	analyse := func() {
		p.Analyse(context.Background(), func(id string) {
			fyne.Do(func() {
				if cur, ok := p.Current(); ok && cur.ID == id {
					p.Prefill()
					syncFields()
				}
				w.queue.SetItems(p.Items())
				pnl.refreshHeader()
				if p.BatchActive() {
					refreshBatch() // the plan follows the arriving analysis
				}
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
		if ok && !p.BatchActive() { // the open document is the marked row, also after a restart
			if keys := w.queue.SelectedKeys(); len(keys) != 1 || keys[0] != cur.ID {
				w.queue.SelectKeys([]string{cur.ID})
			}
		}
		sh.Status.SetRight(i18n.T("sort.status.open", "Count", strconv.Itoa(len(items))))
		if ok {
			act.SetMode(false)
			w.picker.Focus()
		}
		analyse()
	}

	// Keyboard flow: Enter goes to the next field, whether it is empty or not
	// (type, date, invoice, amount, then the free texts); Shift+Enter goes back.
	fieldOrder := func() []string {
		order := []string{"type", "date", "invoice", "amount"}
		for _, l := range p.TextFields() {
			order = append(order, "text:"+l)
		}
		return order
	}
	focusField := func(n string) {
		switch {
		case n == "type":
			w.picker.Focus()
		case n == "date":
			w.date.Focus()
		case n == "invoice":
			sh.Focus(w.invoice)
		case n == "amount":
			sh.Focus(w.amount)
		case strings.HasPrefix(n, "text:"):
			sh.Focus(textEntry(n[5:]))
		}
	}
	w.step = func(from string, delta int) {
		order := fieldOrder()
		for i, n := range order {
			if n == from {
				if j := i + delta; j >= 0 && j < len(order) {
					focusField(order[j])
				}
				return
			}
		}
	}
	// focusNext moves to the next empty field after from ("" = from the top).
	focusNext = func(from string) {
		order := fieldOrder()
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
			case strings.HasPrefix(n, "text:") && f.Text[n[5:]] == "":
				sh.Focus(textEntry(n[5:]))
				return
			}
		}
	}
	normal := func() { act.SetMode(true) }
	w.picker.OnEscape, w.date.OnEscape, w.invoice.OnEscape, w.amount.OnEscape = normal, normal, normal, normal
	w.picker.OnChanged = func(id string) {
		if !syncing {
			p.SetType(id)
			syncing = true
			syncDate()
			syncTexts()
			syncing = false
			refreshForm()
		}
	}
	w.picker.OnCommitted = func(id string) {
		p.SetType(id)
		syncing = true
		syncDate()
		syncTexts()
		syncing = false
		refreshForm()
		w.step("type", 1)
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
	w.date.OnSubmitted = func(*time.Time) { w.step("date", 1) }
	w.date.OnSubmitBack = func(*time.Time) { w.step("date", -1) }
	w.picker.OnCommitBack = func() { w.step("type", -1) }
	w.invoice.OnChanged = func(s string) {
		if !syncing {
			p.SetInvoice(s)
			refreshForm()
		}
	}
	w.invoice.OnSubmitted = func(string) { w.step("invoice", 1) }
	w.invoice.OnSubmitBack = func() { w.step("invoice", -1) }
	w.amount.OnChanged = func(s string) {
		if !syncing {
			p.SetAmount(s)
			refreshForm()
		}
	}
	w.amount.OnSubmitted = func(string) { w.step("amount", 1) }
	w.amount.OnSubmitBack = func() { w.step("amount", -1) }
	w.queue.OnActivated = func(it *inbox.Item) {
		if it != nil && p.Open(it.ID) {
			showDoc()
		}
	}

	act.File = func() {
		if p.BatchActive() { // Cmd+Enter files the batch when one is selected
			act.FileBatch()
			return
		}
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
	act.FileBatch = func() {
		if !p.CanBatch(pnl.batchType) {
			notify(theme.StatusWarning, i18n.T("sort.batch.incomplete"))
			return
		}
		typeID := pnl.batchType
		saveView()
		go func() {
			filed, failed := p.FileBatch(context.Background(), typeID)
			fyne.Do(func() {
				if len(failed) > 0 {
					f := failed[0]
					notify(theme.StatusDanger, i18n.T("toast.batch.partial", "Done", strconv.Itoa(len(filed)),
						"Failed", strconv.Itoa(len(failed)), "Name", f.Name, "Err", f.Err.Error()))
				} else {
					notify(theme.StatusSuccess, i18n.T("toast.batch.filed", "Count", strconv.Itoa(len(filed))))
				}
				w.queue.SelectKeys(p.Selection())
				showDoc()
			})
		}()
	}
	act.UndoBatch = func() {
		if !p.CanUndo() {
			return
		}
		go func() {
			es, err := p.UndoBatch(context.Background())
			fyne.Do(func() {
				if err != nil {
					notify(theme.StatusDanger, err.Error())
				}
				if len(es) > 0 {
					notify(theme.StatusInfo, i18n.T("toast.batch.undone", "Count", strconv.Itoa(len(es))))
				}
				if _, ok := p.Current(); !ok {
					p.Step(0)
				}
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
	// importPaths moves the files into the inbox in a goroutine (the import
	// moves files); done gets the number of new documents on the UI thread.
	// A move that failed is shown in an error dialog.
	importPaths := func(paths []string, done func(added int)) {
		go func() {
			before := p.Count()
			opened, err := p.ImportPaths(paths...)
			added := p.Count() - before
			fyne.Do(func() {
				if opened {
					showDoc()
				}
				if err != nil {
					widgets.ShowError(sh.Win, err)
				}
				if done != nil {
					done(added)
				}
			})
		}()
	}
	act.Import = func(paths ...string) { importPaths(paths, nil) }
	act.PasteFiles = func() {
		paths := clipboardFiles()
		if len(paths) == 0 {
			notify(theme.StatusWarning, i18n.T("toast.paste.none"))
			return
		}
		importPaths(paths, func(n int) {
			if n > 0 {
				notify(theme.StatusInfo, i18n.T("toast.pasted", "Count", strconv.Itoa(n)))
			} else {
				notify(theme.StatusWarning, i18n.T("toast.paste.none"))
			}
		})
	}
	act.LearnRules = func() {
		go func() {
			sug, err := p.SuggestRules(context.Background())
			fyne.Do(func() {
				switch {
				case err != nil:
					notify(theme.StatusWarning, err.Error())
				case len(sug) == 0:
					notify(theme.StatusInfo, i18n.T("sort.learn.none"))
				default:
					showLearnDialog(sh, p, sug, notify)
				}
			})
		}()
	}
	act.AddFiles = func() {
		widgets.ShowFilesOpen(sh.Win, widgets.FileDialogOptions{Extensions: []string{".pdf"}}, func(paths []string) {
			act.Import(paths...)
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
							widgets.ShowError(sh.Win, err)
							return
						}
						notify(theme.StatusInfo, i18n.T("toast.deleted", "Name", cur.Name))
						showDoc()
					})
				}()
			})
	}
	// PutBack moves the document back to where it was imported from; it
	// needs no confirmation because nothing is lost.
	act.PutBack = func() {
		cur, ok := p.Current()
		if !ok {
			return
		}
		if !p.CanPutBack() {
			notify(theme.StatusWarning, i18n.T("error.sort.noorigin"))
			return
		}
		saveView()
		go func() {
			err := p.PutBack()
			fyne.Do(func() {
				if err != nil {
					widgets.ShowError(sh.Win, err)
					return
				}
				notify(theme.StatusInfo, i18n.T("toast.putback", "Name", cur.Name))
				showDoc()
			})
		}()
	}
	// AdoptInPlace records the document in the library where it lies (no
	// renaming); it needs type and date and an origin inside the project.
	act.AdoptInPlace = func() {
		if _, ok := p.Current(); !ok {
			return
		}
		if !p.CanAdoptInPlace() {
			notify(theme.StatusWarning, i18n.T("error.sort.noorigin"))
			return
		}
		saveView()
		go func() {
			e, err := p.AdoptInPlace(context.Background())
			fyne.Do(func() {
				if err != nil {
					widgets.ShowError(sh.Win, err)
					return
				}
				notify(theme.StatusSuccess, i18n.T("toast.adopted", "Path", e.Path))
				showDoc()
			})
		}()
	}

	addBtn := widgets.NewButton(i18n.T("sort.add"), widgets.ButtonSecondary, func() { act.AddFiles() })
	pnl = newSortPanel(p, act, w, textBox)
	w.panel = pnl
	refreshForm, refreshBatch = pnl.refreshSingle, pnl.refreshBatch
	w.queue.OnSelectionChanged = func() {
		p.SetSelection(w.queue.SelectedKeys())
		refreshBatch()
	}
	side := widgets.Border(nil, widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeTop, 6, addBtn), nil, nil, w.queue)
	right := pnl.root

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
	main.SetMinTrailing(320) // side panel: never clipped, the preview shrinks first
	root := widgets.HSplit(side, main)
	root.SetOffset(0.17)
	root.SetMinLeading(180) // queue

	act.shown = func() { // the shell calls this on every show of the page
		if cur, ok := p.Current(); ok && shownID != cur.ID {
			showDoc()
		}
		if p.Types() == nil {
			return // the first load of Build is still running
		}
		// A type created or changed on the Types page shows up without a restart.
		go func() {
			old := p.Types()
			if err := p.LoadTypes(context.Background()); err != nil {
				return
			}
			if reflect.DeepEqual(old, p.Types()) {
				return
			}
			fyne.Do(syncFields)
		}()
	}
	act.fields = w
	go func() { // the page is attached to the window when this returns
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
			pnl.refreshHeader()
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

// showLearnDialog lets the user tick the rules to add to the selected type.
func showLearnDialog(sh *shell.Shell, p *presenter.Sort, sug []presenter.RuleSuggestion, notify func(theme.Status, string)) {
	ticked := make([]bool, len(sug))
	rows := make([]*widgets.FormRow, 0, len(sug))
	for i, s := range sug {
		ticked[i] = s.Checked
		c := widgets.NewCheck(s.Label, func(on bool) { ticked[i] = on })
		c.SetChecked(s.Checked)
		rows = append(rows, widgets.NewFormRow("", c))
	}
	widgets.ShowForm(sh.Win, i18n.T("sort.learn.title"), rows, i18n.T("sort.learn.confirm"), func(ok bool) {
		if !ok {
			return
		}
		var picked []domain.Rule
		for i, s := range sug {
			if ticked[i] {
				picked = append(picked, s.Rule)
			}
		}
		go func() {
			err := p.LearnRules(context.Background(), picked)
			fyne.Do(func() {
				if err != nil {
					notify(theme.StatusDanger, err.Error())
					return
				}
				notify(theme.StatusSuccess, i18n.T("sort.learn.done", "Count", strconv.Itoa(len(picked))))
			})
		}()
	})
}
