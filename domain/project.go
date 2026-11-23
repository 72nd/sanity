// Package domain holds sanity's entities. It imports no UI.
package domain

import "time"

// Project is one company's accounting: a root folder with office/sanity.json.
type Project struct {
	Name         string   `json:"name"`
	Root         string   `json:"-"`            // folder that contains office/sanity.json
	Inbox        string   `json:"inbox"`        // relative to Root, default "office/inbox"
	IgnoreTokens []string `json:"ignoreTokens"` // own IBAN, VAT ID, address lines
	// StatementKeywords are project-wide words (Kontoauszug, ...) that mark a
	// bank or card statement. A document containing one is not classified as
	// a type without DocumentType.IsStatement. nil means the defaults.
	StatementKeywords []string       `json:"statementKeywords"`
	Types             []DocumentType `json:"types"`
	Library           []LibraryEntry `json:"library"`
}

// DefaultInbox is the inbox folder relative to the project root.
const DefaultInbox = "office/inbox"

// DocumentType names a kind of document with its target path and recognition rules.
type DocumentType struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	PathTemplate string       `json:"pathTemplate"`
	Rules        []Rule       `json:"rules"`
	Threshold    float64      `json:"threshold"`  // preselect from this score on, default 0.8
	UseLearned   bool         `json:"useLearned"` // default true
	Date         DateConfig   `json:"date"`
	Amount       AmountConfig `json:"amount"`
	// Currency is the ISO code (EUR, CHF, USD, GBP) of amounts that carry
	// none; empty means none.
	Currency string `json:"currency,omitempty"`
	// IsStatement marks a statement type (Kontoauszug): its date is the latest
	// date of the month named most often, and documents with a statement
	// keyword are classified as statement types only.
	IsStatement bool `json:"statement,omitempty"`
}

// Rule is one recognition rule of a document type.
type Rule struct {
	Kind    string  `json:"kind"` // keyword | regex | iban | vatId | customerNo | domain | filename
	Value   string  `json:"value"`
	Mode    string  `json:"mode"`   // should | must | not
	Weight  float64 `json:"weight"` // weak 0.2, normal 0.5, strong 0.9
	Where   string  `json:"where"`  // any | header | footer | filename
	Enabled bool    `json:"enabled"`
}

// DateConfig tunes date detection per type.
type DateConfig struct {
	PreferredLabels []string `json:"preferredLabels"`
	IgnoreLabels    []string `json:"ignoreLabels"`
	Source          string   `json:"source"` // auto | text | filename | metadata
	MaxAgeYears     int      `json:"maxAgeYears"`
}

// AmountConfig tunes amount detection per type.
type AmountConfig struct {
	// PreferredLabels are words such as "Betrag" or "Total" that mark the
	// wanted amount; a value whose label window contains one scores higher.
	PreferredLabels []string `json:"preferredLabels"`
}

// DefaultStatementKeywords are the statement keywords of a new project.
var DefaultStatementKeywords = []string{"Kontoauszug", "Buchungsnachweis", "Buchungsdetails", "Account statement"}

// StatementWords returns the statement keywords of the project, the defaults
// when none were stored (nil). An empty non-nil list switches the feature off.
func (p Project) StatementWords() []string {
	if p.StatementKeywords == nil {
		return DefaultStatementKeywords
	}
	return p.StatementKeywords
}

// LibraryEntry is a filed document.
type LibraryEntry struct {
	ID           string    `json:"id"`
	TypeID       string    `json:"typeId"`
	Path         string    `json:"path"`         // relative to Root, with /
	OriginalPath string    `json:"originalPath"` // absolute, for undo and history
	Date         time.Time `json:"date"`
	Invoice      string    `json:"invoice,omitempty"`
	Amount       string    `json:"amount,omitempty"`
	SHA256       string    `json:"sha256"`
	TextHash     string    `json:"textHash,omitempty"`
	FiledAt      time.Time `json:"filedAt"`
	Overrides    []string  `json:"overrides,omitempty"` // corrected suggestions, for tuning
}

// TypeByID returns the document type with the given id.
func (p *Project) TypeByID(id string) (DocumentType, bool) {
	for _, t := range p.Types {
		if t.ID == id {
			return t, true
		}
	}
	return DocumentType{}, false
}
