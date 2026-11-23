package presenter

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Grouping ids of the date columns; they are stored in the grid state.
const (
	GroupYear    = "year"
	GroupQuarter = "quarter"
	GroupMonth   = "month"
)

// DateGroupKey returns the sortable group key of a day for a grouping id:
// "2026", "2026-Q1" or "2026-03". A zero time or an unknown id gives "" (the
// empty group), so the keys sort chronologically as plain strings.
func DateGroupKey(id string, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	switch id {
	case GroupYear:
		return fmt.Sprintf("%04d", t.Year())
	case GroupQuarter:
		return fmt.Sprintf("%04d-Q%d", t.Year(), (int(t.Month())-1)/3+1)
	case GroupMonth:
		return fmt.Sprintf("%04d-%02d", t.Year(), int(t.Month()))
	}
	return ""
}

// DateGroupLabel turns a key of DateGroupKey into the group header text:
// "2026", "Q1 2026" or "03.2026". A key it does not understand comes back
// unchanged, an empty key becomes the "no date" text.
func (p *Library) DateGroupLabel(id, key string) string {
	if key == "" {
		return p.T("library.group.nodate")
	}
	switch id {
	case GroupQuarter:
		if y, q, ok := strings.Cut(key, "-"); ok {
			return q + " " + y
		}
	case GroupMonth:
		if y, m, ok := strings.Cut(key, "-"); ok {
			return m + "." + y
		}
	}
	return key
}

// AmountValue is the numeric value of an amount text (cents / 100) for the
// grid's number filters and sort; the currency is not part of it. ok is false
// when the text has no amount.
func AmountValue(s string) (float64, bool) {
	cents, _, ok := ParseAmount(s)
	if !ok {
		return 0, false
	}
	return float64(cents) / 100, true
}

// AmountSum is the text of a group header's amount cell: the sums of rows per
// currency joined by ", " ("169.85 EUR, 12.00 USD"), or "" when no amount
// parses.
func (p *Library) AmountSum(rows []Row) string {
	sums := p.Totals(rows)
	parts := make([]string, len(sums))
	for i, s := range sums {
		parts[i] = s.String()
	}
	return strings.Join(parts, ", ")
}

// ConditionsLabel is the text of the conditions button: "Conditions" without
// active conditions, "Conditions (2)" with.
func (p *Library) ConditionsLabel(n int) string {
	if n <= 0 {
		return p.T("library.conditions")
	}
	return p.T("library.conditions.n", "Count", strconv.Itoa(n))
}

// AmountLess orders rows by the numeric amount; rows without an amount come
// first. Amounts of different currencies compare by number only.
func (p *Library) AmountLess(a, b Row) bool {
	va, oka := AmountValue(a.Entry.Amount)
	vb, okb := AmountValue(b.Entry.Amount)
	if oka && okb {
		return va < vb
	}
	return !oka && okb
}
