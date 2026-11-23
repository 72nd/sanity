package presenter

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/72nd/sanity/domain"
)

// matches reports whether the row contains q (already lower case and trimmed).
func matches(r Row, q string) bool {
	hay := strings.ToLower(r.Entry.Path + " " + r.TypeName + " " + r.Entry.Invoice + " " + r.Entry.Amount)
	return strings.Contains(hay, q)
}

// NextHit returns the index in rows of the next (forward) or previous row that
// matches query, starting beside from and wrapping around. It returns -1 when
// no row matches. An empty query matches every row, so n and N then step
// through the rows. rows is the order the grid shows, which may be sorted.
func (p *Library) NextHit(rows []Row, from int, query string, forward bool) int {
	q := strings.ToLower(strings.TrimSpace(query))
	n := len(rows)
	step := 1
	if !forward {
		step = -1
	}
	for i := 1; i <= n; i++ {
		j := ((from+step*i)%n + n) % n
		if matches(rows[j], q) {
			return j
		}
	}
	return -1
}

// AbsPath is the absolute path of the row's document.
func (p *Library) AbsPath(r Row) string {
	return filepath.Join(p.root, filepath.FromSlash(r.Entry.Path))
}

// CopyPaths returns the paths of rows, one per line: relative to the office
// folder with "/" (domain.OfficeRel), or absolute.
func (p *Library) CopyPaths(rows []Row, abs bool) string {
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		if abs {
			lines = append(lines, p.AbsPath(r))
		} else {
			lines = append(lines, domain.OfficeRel(r.Entry.Path))
		}
	}
	return strings.Join(lines, "\n")
}

// RevealPath is the absolute path to show in Finder.
func (p *Library) RevealPath(r Row) string { return p.AbsPath(r) }

// Remove deletes the library entries of rows from the store. The files stay
// where they are. It stops at the first error; the rows removed so far are
// gone from the presenter as well, and their number is returned.
func (p *Library) Remove(ctx context.Context, rows []Row) (int, error) {
	gone := map[string]bool{}
	var err error
	for _, r := range rows {
		if err = p.store.RemoveEntry(ctx, r.Entry.ID); err != nil {
			break
		}
		gone[r.Entry.ID] = true
		if p.OnRemoved != nil {
			p.OnRemoved(r.Entry)
		}
	}
	kept := p.rows[:0:0]
	for _, r := range p.rows {
		if !gone[r.Entry.ID] {
			kept = append(kept, r)
		}
	}
	p.rows = kept
	return len(gone), err
}

// Resort sends the filed documents of rows back to the sort flow: it calls
// ImportFn with their absolute paths and returns them. The import moves each
// file into the inbox and remembers its library path as the origin, so
// "put back" in the sort screen returns it there and the entry stays valid.
// Filing a document again moves and renames it; the old library entry stays
// until then.
func (p *Library) Resort(rows []Row) []string {
	paths := make([]string, 0, len(rows))
	for _, r := range rows {
		paths = append(paths, p.AbsPath(r))
	}
	if len(paths) > 0 && p.ImportFn != nil {
		p.ImportFn(paths...)
	}
	return paths
}

// InboxHint returns the hint above the grid and whether to show it.
func (p *Library) InboxHint() (string, bool) {
	if p.InboxCount == nil {
		return "", false
	}
	n := p.InboxCount()
	if n <= 0 {
		return "", false
	}
	return p.T("library.inbox.hint", "Count", strconv.Itoa(n)), true
}

// Sum is the total of the parsed amounts of one currency.
type Sum struct {
	Currency string
	Cents    int64
	N        int // rows that contributed
}

// String formats the sum like the library stores amounts: "169.85 EUR".
func (s Sum) String() string {
	sign := ""
	c := s.Cents
	if c < 0 {
		sign, c = "-", -c
	}
	out := fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
	if s.Currency != "" {
		out += " " + s.Currency
	}
	return out
}

// Totals adds the amounts of rows per currency. Rows whose amount does not
// parse are skipped. The result is sorted by currency.
func (p *Library) Totals(rows []Row) []Sum {
	by := map[string]*Sum{}
	for _, r := range rows {
		cents, cur, ok := ParseAmount(r.Entry.Amount)
		if !ok {
			continue
		}
		s := by[cur]
		if s == nil {
			s = &Sum{Currency: cur}
			by[cur] = s
		}
		s.Cents += cents
		s.N++
	}
	out := make([]Sum, 0, len(by))
	for _, s := range by {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Currency < out[j].Currency })
	return out
}

// SelectionStatus builds the right side of the status bar for the visible row
// count and the selected rows: "N selected, total X EUR" when amounts parse.
func (p *Library) SelectionStatus(shown int, sel []Row) []string {
	items := []string{p.T("status.rows", "Shown", strconv.Itoa(shown), "Total", strconv.Itoa(p.Total()))}
	if len(sel) == 0 {
		return items
	}
	sums := p.Totals(sel)
	if len(sums) == 0 {
		if len(sel) > 1 {
			items = append(items, p.T("status.selected", "Count", strconv.Itoa(len(sel))))
		}
		return items
	}
	parts := make([]string, len(sums))
	for i, s := range sums {
		parts[i] = s.String()
	}
	return append(items, p.T("status.selected.total", "Count", strconv.Itoa(len(sel)), "Total", strings.Join(parts, ", ")))
}

var (
	amountNumber = regexp.MustCompile(`-?\d[\d.,']*`)
	amountCode   = regexp.MustCompile(`[A-Za-z]{3}`)
)

// ParseAmount reads an amount text such as "49.95 EUR", "1.234,56 €" or
// "$12.50" into cents and a currency code (empty when none is given). The last
// separator followed by one or two digits is the decimal mark; anything else
// is a thousands separator.
func ParseAmount(s string) (cents int64, currency string, ok bool) {
	num := amountNumber.FindString(s)
	if num == "" {
		return 0, "", false
	}
	switch {
	case strings.Contains(s, "€"):
		currency = "EUR"
	case strings.Contains(s, "$"):
		currency = "USD"
	case strings.Contains(s, "£"):
		currency = "GBP"
	default:
		currency = strings.ToUpper(amountCode.FindString(s))
	}
	neg := strings.HasPrefix(num, "-")
	num = strings.TrimPrefix(num, "-")
	intPart, frac := num, ""
	if i := strings.LastIndexAny(num, ".,"); i >= 0 {
		if after := num[i+1:]; len(after) == 1 || len(after) == 2 {
			intPart, frac = num[:i], after
		}
	}
	intPart = strings.NewReplacer(".", "", ",", "", "'", "").Replace(intPart)
	whole, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, "", false
	}
	if len(frac) == 1 {
		frac += "0"
	}
	var f int64
	if frac != "" {
		f, _ = strconv.ParseInt(frac, 10, 64)
	}
	cents = whole*100 + f
	if neg {
		cents = -cents
	}
	return cents, currency, true
}
