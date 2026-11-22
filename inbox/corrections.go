package inbox

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/domain"
)

// SuggestedType is the top type suggestion of the analysis.
type SuggestedType struct {
	TypeID    string  `json:"typeID"`
	Score     float64 `json:"score"`
	Confident bool    `json:"confident"`
}

// Correction is one line of the correction log: what the analysis suggested
// for a document and what the user filed. It exists so that thresholds can be
// tuned later. An undo appends a line with Undone set and the same SHA256.
type Correction struct {
	At               time.Time     `json:"at"`
	SHA256           string        `json:"sha256"`
	TypeID           string        `json:"typeID,omitempty"`
	Suggested        SuggestedType `json:"suggested"`
	DateSuggested    string        `json:"dateSuggested,omitempty"` // 2006-01-02
	DateChosen       string        `json:"dateChosen,omitempty"`
	InvoiceSuggested string        `json:"invoiceSuggested,omitempty"`
	InvoiceChosen    string        `json:"invoiceChosen,omitempty"`
	AmountSuggested  string        `json:"amountSuggested,omitempty"`
	AmountChosen     string        `json:"amountChosen,omitempty"`
	Overrides        []string      `json:"overrides,omitempty"`
	Undone           bool          `json:"undone,omitempty"`
}

const dateLayout = "2006-01-02"

// FormatAmount writes an amount as the form shows it, for example "49.95 EUR".
func FormatAmount(a analysis.Amount) string {
	s := fmt.Sprintf("%d.%02d", a.Cents/100, a.Cents%100)
	if a.Currency != "" {
		s += " " + a.Currency
	}
	return s
}

// NewCorrection builds the log line of one filing. res is the analysis of the
// document (has is false when there is none, then nothing counts as
// suggested); v are the values the document was filed with.
func NewCorrection(res Result, has bool, typeID string, v domain.Values, sha string, at time.Time, overrides []string) Correction {
	c := Correction{At: at, SHA256: sha, TypeID: typeID, Overrides: overrides,
		InvoiceChosen: v.Invoice, AmountChosen: v.Amount}
	if !v.Date.IsZero() {
		c.DateChosen = v.Date.Format(dateLayout)
	}
	if !has {
		return c
	}
	if len(res.Suggestions) > 0 {
		s := res.Suggestions[0]
		c.Suggested = SuggestedType{TypeID: s.TypeID, Score: s.Score, Confident: s.Confident}
	}
	if len(res.Dates) > 0 {
		c.DateSuggested = res.Dates[0].Date.Format(dateLayout)
	}
	if len(res.Invoices) > 0 {
		c.InvoiceSuggested = res.Invoices[0].Value
	}
	if len(res.Amounts) > 0 {
		c.AmountSuggested = FormatAmount(res.Amounts[0])
	}
	return c
}

// CorrectionLog appends corrections to <root>/office/.sanity/corrections.jsonl.
// It is safe for concurrent use.
type CorrectionLog struct {
	mu   sync.Mutex
	path string
}

// CorrectionsPath is the log file of a project root.
func CorrectionsPath(root string) string {
	return filepath.Join(root, "office", ".sanity", "corrections.jsonl")
}

// NewCorrectionLog returns the log of the project at root. The file is created
// on the first Append.
func NewCorrectionLog(root string) *CorrectionLog {
	return &CorrectionLog{path: CorrectionsPath(root)}
}

// Append writes one line. The line goes out in a single write on a file opened
// for appending, so lines never interleave.
func (l *CorrectionLog) Append(c Correction) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.Write(b)
	return errors.Join(werr, f.Close())
}

// ReadAll reads the log of the project at root, oldest first. A missing file is
// an empty log. A line that does not parse (for example one torn by a crash) is
// skipped, so one bad line never hides the rest.
func ReadAll(root string) ([]Correction, error) {
	f, err := os.Open(CorrectionsPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Correction
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var c Correction
		if json.Unmarshal(sc.Bytes(), &c) == nil && (c.SHA256 != "" || c.TypeID != "") {
			out = append(out, c)
		}
	}
	return out, sc.Err()
}

// TypeStats counts the filings of one type (the type that was filed).
type TypeStats struct {
	Total int // documents filed with this type
	Hits  int // of them, the suggested type was this one
}

// Report summarises a correction log.
type Report struct {
	Total          int // filings that were not undone
	TypeHits       int // top suggestion equals the type filed
	ConfidentTotal int // filings whose top suggestion was confident
	ConfidentHits  int // of them, the suggestion was right
	DateTotal      int // filings with a suggested date
	DateHits       int // of them, the filed date is the suggested one
	ByType         map[string]TypeStats
}

// Rate is hits/total, 0 when total is 0.
func Rate(hits, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// TypeRate is the share of filings where the suggested type was the type filed.
func (r Report) TypeRate() float64 { return Rate(r.TypeHits, r.Total) }

// ConfidentPrecision is the share of confident suggestions that were right.
func (r Report) ConfidentPrecision() float64 { return Rate(r.ConfidentHits, r.ConfidentTotal) }

// DateRate is the share of suggested dates that were filed unchanged.
func (r Report) DateRate() float64 { return Rate(r.DateHits, r.DateTotal) }

// Stats summarises corrections in log order. An undo line removes the latest
// earlier filing with the same hash from the count, together with itself.
func Stats(cs []Correction) Report {
	var live []Correction
	for _, c := range cs {
		if !c.Undone {
			live = append(live, c)
			continue
		}
		for i := len(live) - 1; i >= 0; i-- {
			if live[i].SHA256 == c.SHA256 {
				live = append(live[:i], live[i+1:]...)
				break
			}
		}
	}
	r := Report{ByType: map[string]TypeStats{}}
	for _, c := range live {
		hit := c.Suggested.TypeID != "" && c.Suggested.TypeID == c.TypeID
		r.Total++
		ts := r.ByType[c.TypeID]
		ts.Total++
		if hit {
			r.TypeHits++
			ts.Hits++
		}
		r.ByType[c.TypeID] = ts
		if c.Suggested.Confident {
			r.ConfidentTotal++
			if hit {
				r.ConfidentHits++
			}
		}
		if c.DateSuggested != "" {
			r.DateTotal++
			if c.DateSuggested == c.DateChosen {
				r.DateHits++
			}
		}
	}
	return r
}
