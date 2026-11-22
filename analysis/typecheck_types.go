package analysis

import "github.com/72nd/sanity/domain"

// Level of a finding of CheckType.
type Level string

const (
	LevelError   Level = "error"   // blocks saving the type
	LevelWarning Level = "warning" // shown, does not block
	LevelOK      Level = "ok"      // a check that passed (shown with an icon, not colour only)
)

// Finding is one result line of CheckType.
type Finding struct {
	Level    Level
	Code     string // stable id, for example "template.invalid", "rule.regex", "id.collision"
	Message  string // English text for logs and the CLI; the UI maps Code to localized text
	RuleIdx  int    // index into DocumentType.Rules, or -1 when the finding is about the type
	Argument string // the offending value (a regex, a placeholder, the colliding id), may be empty
}

// Sample is the text of a sample document for the dry run.
type Sample struct {
	Name     string // display name, usually the file name
	FileName string // for filename rules
	Text     string // normalized text of the document
}

// OtherScore is how well another existing type matches a sample.
type OtherScore struct {
	TypeID string
	Score  float64
}

// SampleResult is the dry run of the new type against one sample.
type SampleResult struct {
	Name      string
	Score     float64
	Confident bool
	Others    []OtherScore // existing types with a score above 0, best first
}

// LibraryHit is a filed document whose cached text now also scores the new
// type above the false-positive limit.
type LibraryHit struct {
	EntryID string
	Path    string
	TypeID  string // the type the entry has today
	Score   float64
}

// Report is the result of CheckType.
type Report struct {
	Findings []Finding
	Samples  []SampleResult
	Library  []LibraryHit
	// Skipped is the number of library entries without cached text that could
	// not be checked.
	Skipped int
}

// HasErrors reports whether any finding is an error (saving is blocked).
func (r Report) HasErrors() bool {
	for _, f := range r.Findings {
		if f.Level == LevelError {
			return true
		}
	}
	return false
}

// LibraryText returns the cached text of a filed document by its SHA-256 (see
// inbox.Cache); ok is false when there is none.
type LibraryText func(sha256 string) (text string, ok bool)

// CheckInput is everything CheckType needs.
type CheckInput struct {
	Type     domain.DocumentType
	Existing []domain.DocumentType // the project's current types
	Samples  []Sample
	Learned  LearnedIndex // may be nil
	Library  []domain.LibraryEntry
	Text     LibraryText // may be nil: the library check is then skipped
	// IgnoreTokens are the project's own identifiers; a rule value that
	// contains one is a warning (the learning ignores them, the rule would not).
	IgnoreTokens []string
}
