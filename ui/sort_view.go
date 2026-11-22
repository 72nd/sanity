package ui

import (
	"context"
	"io"
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

// SortActions are the sort page's operations that commands call. They are set
// while the page is built and are no-ops before.
type SortActions struct {
	File, Undo, Skip, SkipBack, AddFiles func()
	ZoomIn, ZoomOut, ZoomFit             func()
}

func nop() {}

// NewSortActions returns actions that do nothing until the page exists.
func NewSortActions() *SortActions {
	return &SortActions{nop, nop, nop, nop, nop, nop, nop, nop}
}

func queueColumns() []grid.Column[inbox.Item] {
	return []grid.Column[inbox.Item]{
		{ID: "name", Title: i18n.T("col.file"), Width: 220,
			Value: func(it inbox.Item) string { return it.Name },
			Less:  func(a, b inbox.Item) bool { return a.Name < b.Name }},
	}
}

// SortPage builds the work screen: queue, PDF preview and the sort form.
func SortPage(sh *shell.Shell, p *presenter.Sort, q *inbox.Queue, act *SortActions) fyne.CanvasObject {
	queue := grid.New(queueColumns())
	queue.AccessibleLabel = i18n.T("nav.sort")
	queue.StretchLast = true

	preview := widgets.NewImageView()

	typeBox := widgets.NewComboBox(nil, nil)
	typeBox.PlaceHolder = i18n.T("sort.type.placeholder")
	date := widgets.NewDateEntry()
	invoice := widgets.NewEntry()
	amount := widgets.NewEntry()
	pathLabel := widgets.NewLabel("", widgets.LabelMuted)
	hint := widgets.NewLabel("", widgets.LabelMuted)

	var syncing bool // true while code fills the fields

	typeIDs := map[string]string{} // option text -> type id
	fillTypes := func() {
		var opts []string
		for _, t := range p.Types() {
			typeIDs[t.Name] = t.ID
			opts = append(opts, t.Name)
		}
		typeBox.SetOptions(opts)
	}

	refreshForm := func() {
		rel, ok := p.PathPreview()
		switch {
		case ok:
			pathLabel.Text = "→ " + rel
			hint.Text = i18n.T("sort.hint.file")
		default:
			pathLabel.Text = ""
			if m := p.Missing(); len(m) > 0 {
				hint.Text = i18n.T("sort.hint.missing", "Fields", joinComma(m))
			} else {
				hint.Text = ""
			}
		}
		pathLabel.Refresh()
		hint.Refresh()
	}

	var loadPreview func()
	showCurrent := func() {
		syncing = true
		typeBox.SetSelected("")
		date.SetDate(nil)
		invoice.SetText("")
		amount.SetText("")
		syncing = false
		refreshForm()
		loadPreview()
		items := p.Items()
		queue.SetItems(items)
		sh.Status.SetRight(i18n.T("sort.status.open", "Count", itoa(len(items))))
	}

	loadPreview = func() {
		cur, ok := p.Current()
		if !ok {
			preview.SetImage(nil)
			preview.SetLoading(false)
			return
		}
		preview.SetLoading(true)
		go func() { // rendering never blocks the UI thread
			img, err := p.Preview(cur.Path)
			fyne.Do(func() {
				if c, ok := p.Current(); !ok || c.ID != cur.ID {
					return // moved on meanwhile
				}
				preview.SetLoading(false)
				if err != nil {
					sh.Toasts.Notify(theme.StatusDanger, err.Error(), 5*time.Second)
					return
				}
				preview.SetImage(img)
			})
		}()
	}

	typeBox.OnChanged = func(s string) {
		if syncing {
			return
		}
		p.SetType(typeIDs[s])
		refreshForm()
	}
	date.OnChanged = func(t *time.Time) {
		if syncing {
			return
		}
		if t != nil {
			p.SetDate(*t)
		}
		refreshForm()
	}
	invoice.OnChanged = func(s string) {
		if !syncing {
			p.SetInvoice(s)
			refreshForm()
		}
	}
	amount.OnChanged = func(s string) {
		if !syncing {
			p.SetAmount(s)
			refreshForm()
		}
	}

	queue.OnActivated = func(it *inbox.Item) {
		if it != nil && p.Open(it.ID) {
			showCurrent()
		}
	}

	notify := func(st theme.Status, msg string) { sh.Toasts.Notify(st, msg, 4*time.Second) }

	act.File = func() {
		if !p.CanFile() {
			notify(theme.StatusWarning, i18n.T("error.sort.incomplete"))
			return
		}
		go func() {
			e, err := p.File(context.Background())
			fyne.Do(func() {
				if err != nil {
					notify(theme.StatusDanger, err.Error())
					return
				}
				notify(theme.StatusSuccess, i18n.T("toast.filed", "Path", e.Path))
				showCurrent()
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
				showCurrent()
			})
		}()
	}
	act.Skip = func() { p.Step(1); showCurrent() }
	act.SkipBack = func() { p.Step(-1); showCurrent() }
	act.AddFiles = func() {
		widgets.ShowFileOpen(sh.Win, widgets.FileDialogOptions{Extensions: []string{".pdf"}}, func(r io.ReadCloser, path string) {
			r.Close()
			if p.Import(path) {
				showCurrent()
			}
		})
	}
	act.ZoomIn, act.ZoomOut, act.ZoomFit = preview.ZoomIn, preview.ZoomOut, preview.FitWidth

	fileBtn := widgets.NewButton(i18n.T("sort.file"), widgets.ButtonPrimary, func() { act.File() })
	skipBtn := widgets.NewButton(i18n.T("sort.skip"), widgets.ButtonSecondary, func() { act.Skip() })
	addBtn := widgets.NewButton(i18n.T("sort.add"), widgets.ButtonSecondary, func() { act.AddFiles() })

	form := widgets.VBox(
		widgets.NewFormRow(i18n.T("sort.type"), typeBox),
		widgets.NewFormRow(i18n.T("sort.date"), date),
		widgets.NewFormRow(i18n.T("sort.invoice"), invoice),
		widgets.NewFormRow(i18n.T("sort.amount"), amount),
		pathLabel, hint,
		widgets.HBox(fileBtn, skipBtn),
	)
	side := widgets.Border(nil, widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeTop, 6, addBtn), nil, nil, queue)
	right := widgets.Scroll(widgets.PagePadded(widgets.NewCard(i18n.T("sort.form.title"), "", form)))
	zoomLabel := widgets.NewLabel("100 %", widgets.LabelMuted)
	preview.OnZoomChanged = func(z float64) {
		zoomLabel.Text = strconv.Itoa(int(z*100+0.5)) + " %"
		zoomLabel.Refresh()
	}
	zoomBtn := func(icon fyne.ThemeIconName, fn func()) *widgets.Button {
		b := widgets.NewButton("", widgets.ButtonToolbar, fn)
		b.SetIcon(icon)
		return b
	}
	fitBtn := widgets.NewButton(i18n.T("sort.zoom.fit"), widgets.ButtonToolbar, func() { act.ZoomFit() })
	zoomBar := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeBottom, 4, widgets.HBox(
		zoomBtn(fynetheme.IconNameContentRemove, func() { act.ZoomOut() }),
		zoomLabel,
		zoomBtn(fynetheme.IconNameContentAdd, func() { act.ZoomIn() }),
		fitBtn,
	))
	main := widgets.HSplit(widgets.Border(zoomBar, nil, nil, nil, preview), right)
	main.SetOffset(0.66)
	root := widgets.HSplit(side, main)
	root.SetOffset(0.17)

	go func() {
		err := p.LoadTypes(context.Background())
		fyne.Do(func() {
			if err != nil {
				notify(theme.StatusDanger, i18n.T("toast.load.failed"))
				return
			}
			fillTypes()
			p.Step(0)
			showCurrent()
		})
	}()
	q.SetOnChange(func() {
		fyne.Do(func() {
			if _, ok := p.Current(); !ok {
				p.Step(0)
				showCurrent()
				return
			}
			queue.SetItems(p.Items())
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

func itoa(n int) string { return strconv.Itoa(n) }

// Commands returns the sort page's commands; each is reachable through the
// palette and has the shortcut from PLAN.md section 2.
func Commands(a *SortActions) []shell.Command {
	mod := func(k fyne.KeyName, m fyne.KeyModifier) *desktop.CustomShortcut {
		return &desktop.CustomShortcut{KeyName: k, Modifier: fyne.KeyModifierShortcutDefault | m}
	}
	t := func(id string) func() string { return func() string { return i18n.T(id) } }
	return []shell.Command{
		{ID: "sort.file", Title: t("cmd.sort.file"), Shortcut: shell.Ctrl(fyne.KeyReturn), Run: func() { a.File() }},
		{ID: "sort.undo", Title: t("cmd.sort.undo"), Shortcut: mod(fyne.KeyZ, fyne.KeyModifierAlt), Run: func() { a.Undo() }},
		{ID: "sort.next", Title: t("cmd.sort.next"), Shortcut: shell.Ctrl(fyne.KeyRightBracket), Run: func() { a.Skip() }},
		{ID: "sort.prev", Title: t("cmd.sort.prev"), Shortcut: shell.Ctrl(fyne.KeyLeftBracket), Run: func() { a.SkipBack() }},
		{ID: "sort.add", Title: t("cmd.sort.add"), Shortcut: shell.Ctrl(fyne.KeyO), Run: func() { a.AddFiles() }},
		{ID: "sort.zoomin", Title: t("cmd.sort.zoomin"), Shortcut: shell.Ctrl(fyne.KeyEqual), Run: func() { a.ZoomIn() }},
		{ID: "sort.zoomout", Title: t("cmd.sort.zoomout"), Shortcut: shell.Ctrl(fyne.KeyMinus), Run: func() { a.ZoomOut() }},
		{ID: "sort.zoomfit", Title: t("cmd.sort.zoomfit"), Shortcut: shell.Ctrl(fyne.Key0), Run: func() { a.ZoomFit() }},
	}
}
