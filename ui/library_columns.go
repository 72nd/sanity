package ui

import (
	"time"

	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/widgets/grid"
	"github.com/72nd/sanity/ui/presenter"
)

// dateGroupings are the derived groupings of a date column; keys and labels
// come from the presenter.
func dateGroupings(p *presenter.Library, day func(presenter.Row) (t time.Time)) []grid.Grouping[presenter.Row] {
	one := func(id, titleID string) grid.Grouping[presenter.Row] {
		return grid.Grouping[presenter.Row]{
			ID: id, Title: i18n.T(titleID),
			Key:   func(r presenter.Row) string { return presenter.DateGroupKey(id, day(r)) },
			Label: func(key string) string { return p.DateGroupLabel(id, key) },
		}
	}
	return []grid.Grouping[presenter.Row]{
		one(presenter.GroupYear, "library.group.year"),
		one(presenter.GroupQuarter, "library.group.quarter"),
		one(presenter.GroupMonth, "library.group.month"),
	}
}

func libraryColumns(p *presenter.Library) []grid.Column[presenter.Row] {
	date := func(r presenter.Row) time.Time { return r.Entry.Date }
	filed := func(r presenter.Row) time.Time { return r.Entry.FiledAt }
	return []grid.Column[presenter.Row]{
		{ID: "date", Title: i18n.T("col.date"), Width: 110, Kind: grid.KindDate,
			Value:     func(r presenter.Row) string { return p.DateText(r.Entry.Date) },
			Date:      func(r presenter.Row) (time.Time, bool) { return date(r), !date(r).IsZero() },
			Groupings: dateGroupings(p, date),
			Less:      func(a, b presenter.Row) bool { return a.Entry.Date.Before(b.Entry.Date) }},
		{ID: "type", Title: i18n.T("col.type"), Width: 170,
			Value: func(r presenter.Row) string { return r.TypeName },
			Less:  func(a, b presenter.Row) bool { return a.TypeName < b.TypeName }},
		{ID: "path", Title: i18n.T("col.path"), Width: 460,
			Value: func(r presenter.Row) string { return r.Entry.Path },
			Less:  func(a, b presenter.Row) bool { return a.Entry.Path < b.Entry.Path }},
		{ID: "invoice", Title: i18n.T("col.invoice"), Width: 130,
			Value: func(r presenter.Row) string { return r.Entry.Invoice },
			Less:  func(a, b presenter.Row) bool { return a.Entry.Invoice < b.Entry.Invoice }},
		{ID: "amount", Title: i18n.T("col.amount"), Width: 110, Align: grid.AlignRight, Kind: grid.KindNumber,
			Value:     func(r presenter.Row) string { return r.Entry.Amount },
			Number:    func(r presenter.Row) (float64, bool) { return presenter.AmountValue(r.Entry.Amount) },
			Aggregate: func(rows []presenter.Row) string { return p.AmountSum(rows) },
			Less:      func(a, b presenter.Row) bool { return p.AmountLess(a, b) }},
		{ID: "filed", Title: i18n.T("col.filedAt"), Width: 150, Kind: grid.KindDate,
			Value:     func(r presenter.Row) string { return p.FiledText(r.Entry.FiledAt) },
			Date:      func(r presenter.Row) (time.Time, bool) { return filed(r), !filed(r).IsZero() },
			Groupings: dateGroupings(p, filed),
			Less:      func(a, b presenter.Row) bool { return a.Entry.FiledAt.Before(b.Entry.FiledAt) }},
	}
}
