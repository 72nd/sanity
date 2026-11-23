package analysis

import "time"

// StatementLabel is the Label of the candidate StatementDate produces.
const StatementLabel = "statement month"

// StatementDate picks the date of a bank or card statement from the dates
// found in it. The calendar month (year and month) that the document names
// most often wins, counting every occurrence; on a tie the later month wins.
// The result is the latest date inside that month that occurs in the document.
// So booking dates and value dates of April outweigh the print date of the
// statement in October. Fallback dates (not from the text) are ignored. ok is
// false when there is no date to choose from.
func StatementDate(cands []DateCandidate) (c DateCandidate, ok bool) {
	type ym struct {
		y int
		m time.Month
	}
	less := func(a, b ym) bool { return a.y < b.y || a.y == b.y && a.m < b.m }
	count := map[ym]int{}
	for _, d := range cands {
		if d.Fallback || d.Date.IsZero() {
			continue
		}
		n := d.Count
		if n < 1 {
			n = 1
		}
		count[ym{d.Date.Year(), d.Date.Month()}] += n
	}
	var best ym
	bestN := 0
	for k, n := range count {
		if n > bestN || n == bestN && less(best, k) {
			best, bestN = k, n
		}
	}
	if bestN == 0 {
		return DateCandidate{}, false
	}
	for _, d := range cands {
		if d.Fallback || d.Date.IsZero() || (ym{d.Date.Year(), d.Date.Month()}) != best {
			continue
		}
		if !ok || d.Date.After(c.Date) {
			c, ok = d, true
		}
	}
	c.Label = StatementLabel
	if c.Score < 0.85 {
		c.Score = 0.85
	}
	c.Period = false
	return c, ok
}
