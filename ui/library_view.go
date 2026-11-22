package ui

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/theme"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/fine/widgets/grid"
	"github.com/72nd/sanity/ui/presenter"
)

func libraryColumns(p *presenter.Library) []grid.Column[presenter.Row] {
	return []grid.Column[presenter.Row]{
		{ID: "date", Title: i18n.T("col.date"), Width: 110,
			Value: func(r presenter.Row) string { return p.DateText(r.Entry.Date) },
			Less:  func(a, b presenter.Row) bool { return a.Entry.Date.Before(b.Entry.Date) }},
		{ID: "type", Title: i18n.T("col.type"), Width: 170,
			Value: func(r presenter.Row) string { return r.TypeName },
			Less:  func(a, b presenter.Row) bool { return a.TypeName < b.TypeName }},
		{ID: "path", Title: i18n.T("col.path"), Width: 460,
			Value: func(r presenter.Row) string { return r.Entry.Path },
			Less:  func(a, b presenter.Row) bool { return a.Entry.Path < b.Entry.Path }},
		{ID: "invoice", Title: i18n.T("col.invoice"), Width: 130,
			Value: func(r presenter.Row) string { return r.Entry.Invoice },
			Less:  func(a, b presenter.Row) bool { return a.Entry.Invoice < b.Entry.Invoice }},
		{ID: "amount", Title: i18n.T("col.amount"), Width: 110, Align: grid.AlignRight,
			Value: func(r presenter.Row) string { return r.Entry.Amount },
			Less:  func(a, b presenter.Row) bool { return a.Entry.Amount < b.Entry.Amount }},
		{ID: "filed", Title: i18n.T("col.filedAt"), Width: 150,
			Value: func(r presenter.Row) string { return p.FiledText(r.Entry.FiledAt) },
			Less:  func(a, b presenter.Row) bool { return a.Entry.FiledAt.Before(b.Entry.FiledAt) }},
	}
}

// LibraryPage builds the start page: a quick filter above the grid of all
// filed documents, counts in the status bar.
func LibraryPage(sh *shell.Shell, p *presenter.Library) fyne.CanvasObject {
	g := grid.New(libraryColumns(p))
	g.AccessibleLabel = i18n.T("nav.library")
	g.StretchLast = true
	status := func() { sh.Status.SetRight(p.StatusItems(g.Len(), g.SelectedCount())...) }
	g.OnSelectionChanged = status

	filter := widgets.NewEntry()
	filter.SetPlaceHolder(i18n.T("library.filter.placeholder"))
	filter.OnChanged = func(q string) {
		g.SetItems(p.Visible(q))
		status()
	}

	loading := widgets.NewEmptyState(fynetheme.IconNameViewRefresh, i18n.T("library.loading.title"), i18n.T("library.loading.desc"), nil)
	g.Hide()
	show := func() {
		loading.Hide()
		g.Show()
		g.SetItems(p.Visible(filter.Text))
		status()
	}
	if p.Loaded() {
		show() // show the old data at once, refresh below
	}
	go func() { // never block the UI thread on the store
		err := p.Load(context.Background())
		fyne.Do(func() {
			if err != nil {
				sh.Toasts.Notify(theme.StatusDanger, i18n.T("toast.load.failed"), 6*time.Second)
				return
			}
			show()
		})
	}()

	bar := widgets.NewPanel(theme.SurfaceRaised, widgets.EdgeBottom, 6, filter)
	return widgets.Border(bar, nil, nil, nil, widgets.Stack(g, loading))
}
