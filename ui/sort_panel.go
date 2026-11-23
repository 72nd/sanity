package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	fynetheme "fyne.io/fyne/v2/theme"

	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/theme"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/ui/presenter"
)

// sortPanel is the right-hand side of the sort page: a header (document name
// and position), a vertically scrolling body (form or batch plan) and a pinned
// footer (state, target path and the actions). It never takes more width than
// the split gives it: every text wraps and the actions share one row.
type sortPanel struct {
	p   *presenter.Sort
	act *SortActions
	w   *sortWidgets

	root fyne.CanvasObject

	title, position *widgets.Label

	// single document
	single     fyne.CanvasObject
	dup        *widgets.Callout
	dupEntry   domain.LibraryEntry
	fileBtn    *widgets.Button
	adoptBtn   *widgets.Button
	inProject  *widgets.StatusBadge
	skipBtn    *widgets.Button
	moreBtn    *widgets.Button
	rowMissing fyne.CanvasObject
	missing    *widgets.Label
	rowNoType  fyne.CanvasObject
	rowReady   fyne.CanvasObject
	pathLabel  *widgets.Label
	footOne    fyne.CanvasObject

	// batch
	batchBox      fyne.CanvasObject
	batchPicker   *widgets.Picker
	batchType     string // the type picked for the batch
	batchPlan     fyne.CanvasObject
	batchBtn      *widgets.Button
	batchOK       fyne.CanvasObject
	batchBlocked  fyne.CanvasObject
	batchNoType   fyne.CanvasObject
	batchOKCount  *widgets.Label
	batchBlkCount *widgets.Label
	footBatch     fyne.CanvasObject
}

// badgeRow is a status badge with a text that takes the remaining width. The
// text is vertically centered to the badge.
func badgeRow(b fyne.CanvasObject, text *widgets.Label) fyne.CanvasObject {
	return widgets.Border(nil, nil, b, nil, widgets.VBox(widgets.Spacer(), text, widgets.Spacer()))
}

// vgap is an empty block of the given height, used as spacing inside a VBox.
// The fill matches the surface it sits on, so it is invisible.
func vgap(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(theme.Color(theme.Surface))
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}

// newSortPanel builds the panel. textBox holds the free text rows of the form.
func newSortPanel(p *presenter.Sort, act *SortActions, w *sortWidgets, textBox fyne.CanvasObject) *sortPanel {
	s := &sortPanel{p: p, act: act, w: w}
	pad := theme.Size(theme.SizePagePad)

	s.title = widgets.NewWrapLabel("", widgets.LabelBody)
	s.title.Bold = true
	s.position = widgets.NewLabel("", widgets.LabelMuted)
	s.inProject = widgets.NewStatusBadge(theme.StatusInfo, i18n.T("sort.inproject"))
	s.inProject.Hide()
	header := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeBottom, pad,
		widgets.VBox(s.title, widgets.HBox(s.position, s.inProject)))

	// single form
	s.dup = widgets.NewCallout(theme.StatusWarning, "", widgets.NewButton(i18n.T("sort.duplicate.show"),
		widgets.ButtonSecondary, func() { act.ShowInLibrary(s.dupEntry) }))
	s.dup.Hide()
	s.single = widgets.VBox(
		s.dup,
		widgets.NewFormRow(i18n.T("sort.type"), w.picker),
		widgets.NewFormRow(i18n.T("sort.date"), w.date),
		widgets.Columns(widgets.NewFormRow(i18n.T("sort.invoice"), w.invoice), widgets.NewFormRow(i18n.T("sort.amount"), w.amount)),
		textBox,
	)

	// batch form
	s.batchPicker = widgets.NewPicker(nil)
	s.batchPicker.PlaceHolder = i18n.T("sort.type.placeholder")
	s.batchPicker.Rows = 5
	s.batchPlan = widgets.VBox()
	gapH := theme.Size(theme.SizePagePad)
	s.batchBox = widgets.VBox(widgets.NewWrapLabel(i18n.T("sort.batch.hint"), widgets.LabelMuted), vgap(gapH),
		widgets.NewFormRow(i18n.T("sort.type"), s.batchPicker), vgap(gapH), s.batchPlan)
	s.batchBox.Hide()
	s.batchPicker.OnChanged = func(id string) { s.batchType = id; s.refreshBatch() }
	s.batchPicker.OnCommitted = s.batchPicker.OnChanged
	w.picker.Rows = 5

	body := widgets.NewPanel(theme.Surface, widgets.EdgeNone, pad, widgets.VBox(s.single, s.batchBox))

	// footer, single
	s.missing = widgets.NewWrapLabel("", widgets.LabelMuted)
	s.rowMissing = badgeRow(widgets.NewStatusBadge(theme.StatusWarning, i18n.T("sort.state.missing")), s.missing)
	s.rowNoType = widgets.NewWrapLabel(i18n.T("sort.state.notype"), widgets.LabelMuted)
	s.rowReady = badgeRow(widgets.NewStatusBadge(theme.StatusSuccess, i18n.T("sort.state.ready")),
		widgets.NewWrapLabel(i18n.T("sort.hint.file"), widgets.LabelMuted))
	s.pathLabel = widgets.NewWrapLabel("", widgets.LabelMono)
	s.fileBtn = widgets.NewButton(i18n.T("sort.file"), widgets.ButtonPrimary, func() { act.File() })
	s.adoptBtn = widgets.NewButton(i18n.T("sort.adopt"), widgets.ButtonPrimary, func() { act.AdoptInPlace() })
	s.adoptBtn.Hide()
	s.skipBtn = widgets.NewButton(i18n.T("sort.skip"), widgets.ButtonSecondary, func() { act.Skip() })
	s.moreBtn = widgets.NewButton("", widgets.ButtonSecondary, nil)
	s.moreBtn.SetIcon(fynetheme.IconNameMoreVertical)
	s.moreBtn.OnTapped = func() {
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(s.moreBtn)
		widgets.ShowContextMenu(s.moreBtn, fyne.NewPos(pos.X, pos.Y+s.moreBtn.Size().Height), s.moreItems())
	}
	actions := widgets.Border(nil, nil, nil, widgets.HBox(s.skipBtn, s.moreBtn), widgets.VBox(s.fileBtn, s.adoptBtn))
	s.footOne = widgets.VBox(s.rowMissing, s.rowNoType, s.rowReady, s.pathLabel, actions)

	// footer, batch
	s.batchOKCount = widgets.NewWrapLabel("", widgets.LabelMuted)
	s.batchBlkCount = widgets.NewWrapLabel("", widgets.LabelMuted)
	s.batchOK = badgeRow(widgets.NewStatusBadge(theme.StatusSuccess, i18n.T("sort.batch.ready")), s.batchOKCount)
	s.batchBlocked = badgeRow(widgets.NewStatusBadge(theme.StatusWarning, i18n.T("sort.batch.blocked")), s.batchBlkCount)
	s.batchNoType = widgets.NewWrapLabel(i18n.T("sort.state.notype"), widgets.LabelMuted)
	s.batchBtn = widgets.NewButton("", widgets.ButtonPrimary, func() { act.FileBatch() })
	s.footBatch = widgets.VBox(s.batchOK, s.batchBlocked, s.batchNoType, s.batchBtn)
	s.footBatch.Hide()

	footer := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeTop, pad, widgets.VBox(s.footOne, s.footBatch))
	s.root = widgets.Border(header, footer, nil, nil, widgets.VScroll(body))
	return s
}

// moreItems are the entries of the "more" menu; they read the state when the
// menu opens. A document that already lies in the project folder has "Adopt in
// place" as its main action, so "File" moves into the menu instead.
func (s *sortPanel) moreItems() []widgets.MenuItem {
	items := []widgets.MenuItem{
		{Label: i18n.T("sort.learn"), Icon: fynetheme.IconNameDocumentCreate, Action: func() { s.act.LearnRules() }},
		widgets.MenuSeparator(),
	}
	if s.p.CanAdoptInPlace() {
		_, ready := s.p.PathPreview()
		items = append(items, widgets.MenuItem{Label: i18n.T("sort.file"), Icon: fynetheme.IconNameMoveDown, Disabled: !ready, Action: func() { s.act.File() }})
	}
	items = append(items,
		widgets.MenuItem{Label: i18n.T("sort.putback"), Icon: fynetheme.IconNameHistory, Disabled: !s.p.CanPutBack(), Action: func() { s.act.PutBack() }},
		widgets.MenuSeparator(),
		widgets.MenuItem{Label: i18n.T("sort.delete"), Icon: fynetheme.IconNameDelete, Danger: true, Action: func() { s.act.Remove() }},
	)
	return items
}

func setShown(o fyne.CanvasObject, on bool) {
	if on {
		o.Show()
	} else {
		o.Hide()
	}
}

func setEnabled(b *widgets.Button, on bool) {
	if on {
		b.Enable()
	} else {
		b.Disable()
	}
}

// refreshHeader shows the document name and its place in the queue (or the
// batch size).
func (s *sortPanel) refreshHeader() {
	s.inProject.Hide()
	if s.p.BatchActive() {
		s.title.SetText(i18n.T("sort.batch.title", "Count", strconv.Itoa(len(s.p.Selection()))))
		s.position.SetText("")
		s.position.Hide()
		return
	}
	s.position.Show()
	cur, ok := s.p.Current()
	if !ok {
		s.title.SetText(i18n.T("sort.form.title"))
		s.position.SetText("")
		s.position.Hide()
		return
	}
	s.title.SetText(cur.Name)
	setShown(s.inProject, s.p.CanAdoptInPlace())
	items := s.p.Items()
	for i, it := range items {
		if it.ID == cur.ID {
			s.position.SetText(i18n.T("sort.position", "Cur", strconv.Itoa(i+1), "Total", strconv.Itoa(len(items))))
		}
	}
}

// refreshSingle updates everything that depends on the form state.
func (s *sortPanel) refreshSingle() {
	s.refreshHeader()
	rel, ready := s.p.PathPreview()
	noType := s.p.Form().TypeID == ""
	setShown(s.rowReady, ready)
	setShown(s.pathLabel, ready)
	setShown(s.rowNoType, !ready && noType)
	setShown(s.rowMissing, !ready && !noType)
	s.pathLabel.SetText("")
	if ready {
		s.pathLabel.SetText(rel)
	}
	s.missing.SetText(joinComma(s.p.MissingLabels()))
	setEnabled(s.fileBtn, ready)
	inProj := s.p.CanAdoptInPlace()
	setShown(s.fileBtn, !inProj)
	setShown(s.adoptBtn, inProj)
	setEnabled(s.adoptBtn, s.p.CanAdopt())
	if e, why, ok := s.p.Duplicate(); ok {
		s.dupEntry = e
		s.dup.SetText(i18n.T("sort.duplicate", "Path", e.Path, "Reason", s.p.DuplicateReasonText(why)))
		s.dup.Show()
	} else {
		s.dup.Hide()
	}
	s.footOne.Refresh()
	s.single.Refresh()
	s.root.Refresh()
}

// refreshBatch shows the batch panel while several documents are selected and
// the single form otherwise.
func (s *sortPanel) refreshBatch() {
	sel := s.p.Selection()
	on := len(sel) > 1
	setShown(s.batchBox, on)
	setShown(s.footBatch, on)
	setShown(s.single, !on)
	setShown(s.footOne, !on)
	if !on {
		s.refreshSingle()
		return
	}
	s.refreshHeader()
	choices := s.p.BatchTypeChoices()
	valid, suggested := false, ""
	items := make([]widgets.PickerItem, 0, len(choices))
	for _, c := range choices {
		it := widgets.PickerItem{ID: c.ID, Label: c.Name, Score: c.Score}
		if c.Confident {
			it.Badge, it.BadgeStatus = i18n.T("sort.badge.suggested"), theme.StatusSuccess
			if suggested == "" {
				suggested = c.ID
			}
		}
		valid = valid || c.ID == s.batchType
		items = append(items, it)
	}
	if !valid { // keep a still valid pick, else take the suggestion
		s.batchType = suggested
	}
	s.batchPicker.SetItems(items)
	s.batchPicker.Preselect(s.batchType)
	n := strconv.Itoa(len(sel))
	s.batchBtn.SetLabel(i18n.T("sort.batch.file", "Count", n))

	// Each plan entry is one group (badge and name, then its detail line);
	// groups are separated by a gap so the list does not read as one block.
	var rows []fyne.CanvasObject
	gapH := theme.Size(theme.SizePagePad)
	add := func(g fyne.CanvasObject) {
		if len(rows) > 0 {
			rows = append(rows, vgap(gapH))
		}
		rows = append(rows, g)
	}
	ready := 0
	plan := s.p.BatchPlan(s.batchType)
	for _, b := range plan {
		name := widgets.NewWrapLabel(b.Name, widgets.LabelBody)
		switch {
		case s.batchType == "":
			add(name)
		case b.Blocked():
			why := b.Err
			if len(b.Missing) > 0 {
				why = i18n.T("sort.batch.missing", "Fields", joinComma(presenter.FieldLabels(s.p.T, b.Missing)))
			}
			badge := widgets.NewStatusBadge(theme.StatusWarning, i18n.T("sort.batch.blocked"))
			add(widgets.VBox(widgets.Border(nil, nil, badge, nil, name), widgets.NewWrapLabel(why, widgets.LabelMuted)))
		default:
			ready++
			badge := widgets.NewStatusBadge(theme.StatusSuccess, i18n.T("sort.batch.ready"))
			add(widgets.VBox(widgets.Border(nil, nil, badge, nil, name), widgets.NewWrapLabel("→ "+b.Path, widgets.LabelMuted)))
		}
	}
	pl := s.batchPlan.(*fyne.Container)
	pl.Objects = rows
	pl.Refresh()

	count := i18n.T("sort.batch.state", "Ready", strconv.Itoa(ready), "Total", strconv.Itoa(len(plan)))
	s.batchOKCount.SetText(count)
	s.batchBlkCount.SetText(count)
	all := s.batchType != "" && ready == len(plan)
	setShown(s.batchOK, all)
	setShown(s.batchBlocked, s.batchType != "" && !all)
	setShown(s.batchNoType, s.batchType == "")
	setEnabled(s.batchBtn, s.p.CanBatch(s.batchType))
	s.footBatch.Refresh()
	s.batchBox.Refresh()
	s.root.Refresh()
}
