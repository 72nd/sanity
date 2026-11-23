package presenter

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/domain"
)

// Collision choices for an imported type whose id already exists.
const (
	CollisionNewID   = "newID"
	CollisionReplace = "replace"
)

// codeIDCollision is the finding code that the collision choice resolves.
const codeIDCollision = "id.collision"

// ImportFuncs are the headless pieces the import flow calls. NewTypes fills
// them with the real implementations; tests replace them with fakes.
type ImportFuncs struct {
	Parse    func(data []byte) (domain.DocumentType, []analysis.SnippetNote, error)
	Check    func(in analysis.CheckInput) analysis.Report
	TypeJSON func(t domain.DocumentType) ([]byte, error)
	Prompt   func(existingTypeIDs []string) string
	// Extract returns the normalized text of a sample PDF.
	Extract func(ctx context.Context, path string) (string, error)
	// LibraryText returns the cached-text lookup of the project at root; nil
	// skips the library check.
	LibraryText func(root string) analysis.LibraryText
	// Learned is the learned index for the dry run; may be nil.
	Learned analysis.LearnedIndex
}

// ImportRuleRow is one rule of the imported type with the note of the snippet.
type ImportRuleRow struct {
	Index int
	Rule  domain.Rule
	Why   string
}

// DryRunRow is one line of the dry-run table, formatted for display.
type DryRunRow struct {
	Sample    string
	Score     string // "0.85"
	Confident bool
	Others    string // "Name 0.42, Name 0.10", empty when none
	Date      string // suggested date with the new type, empty when none
	Amount    string // suggested amount with the new type, empty when none
}

// LibraryHitRow is a filed document that now also scores the new type.
type LibraryHitRow struct {
	Path     string
	TypeName string
	Score    string
}

// importSession is the state of one run of the import dialog.
type importSession struct {
	snippet   string
	samples   []analysis.Sample
	typ       domain.DocumentType
	hasType   bool
	notes     []analysis.SnippetNote
	report    analysis.Report
	checked   bool
	collision string
}

// ImportReset starts a new import session (an empty dialog).
func (p *Types) ImportReset() { p.imp = importSession{collision: CollisionNewID} }

// SetSnippet sets the pasted snippet text. A change invalidates the last check.
func (p *Types) SetSnippet(s string) {
	if s != p.imp.snippet {
		p.imp.snippet = s
		p.imp.checked, p.imp.hasType = false, false
		p.imp.report = analysis.Report{}
	}
}

// Snippet returns the snippet text.
func (p *Types) Snippet() string { return p.imp.snippet }

// AddSample extracts the text of a sample PDF for the dry run. A change
// invalidates the last check.
func (p *Types) AddSample(ctx context.Context, path string) error {
	if p.Import.Extract == nil {
		return errors.New(p.T("types.import.err.noextract"))
	}
	text, err := p.Import.Extract(ctx, path)
	if err != nil {
		return err
	}
	name := filepath.Base(path)
	p.imp.samples = append(p.imp.samples, analysis.Sample{Name: name, FileName: name, Text: text})
	p.imp.checked = false
	return nil
}

// RemoveSample drops sample i.
func (p *Types) RemoveSample(i int) {
	if i >= 0 && i < len(p.imp.samples) {
		p.imp.samples = slices.Delete(p.imp.samples, i, i+1)
		p.imp.checked = false
	}
}

// Samples returns the sample documents of the session.
func (p *Types) Samples() []analysis.Sample { return slices.Clone(p.imp.samples) }

// ParseSnippet parses the snippet (envelope or bare type) into the imported
// type and its notes. On error nothing is kept.
func (p *Types) ParseSnippet() error {
	p.imp.hasType, p.imp.checked = false, false
	if strings.TrimSpace(p.imp.snippet) == "" {
		return errors.New(p.T("types.import.err.empty"))
	}
	t, notes, err := p.Import.Parse([]byte(p.imp.snippet))
	if err != nil {
		return fmt.Errorf("%s: %w", p.T("types.import.err.parse"), err)
	}
	p.imp.typ, p.imp.notes, p.imp.hasType = cloneDocType(t), notes, true
	if p.imp.collision == "" {
		p.imp.collision = CollisionNewID
	}
	return nil
}

// Check parses the snippet and runs the checks and the dry run against the
// samples, the existing types and the library. The report replaces the last.
func (p *Types) Check(ctx context.Context) error {
	if err := p.ParseSnippet(); err != nil {
		return err
	}
	pr, err := p.store.Project(ctx)
	if err != nil {
		return err
	}
	p.root, p.types = pr.Root, pr.Types
	lib, err := p.store.Library(ctx)
	if err != nil {
		return err
	}
	in := analysis.CheckInput{
		Type: cloneDocType(p.imp.typ), Existing: slices.Clone(pr.Types), Samples: slices.Clone(p.imp.samples),
		Learned: p.Import.Learned, Library: lib, IgnoreTokens: slices.Clone(pr.IgnoreTokens),
		StatementKeywords: pr.StatementWords(),
	}
	if p.Import.LibraryText != nil {
		in.Text = p.Import.LibraryText(pr.Root)
	}
	p.imp.report, p.imp.checked = p.Import.Check(in), true
	return nil
}

// Checked reports whether the report belongs to the current snippet.
func (p *Types) Checked() bool { return p.imp.checked }

// Report returns the last report.
func (p *Types) Report() analysis.Report { return p.imp.report }

// ImportedType returns the parsed type of the last check.
func (p *Types) ImportedType() (domain.DocumentType, bool) {
	return cloneDocType(p.imp.typ), p.imp.hasType
}

// ImportRules returns the imported rules with the "why" notes.
func (p *Types) ImportRules() []ImportRuleRow {
	rows := make([]ImportRuleRow, len(p.imp.typ.Rules))
	for i, r := range p.imp.typ.Rules {
		rows[i] = ImportRuleRow{Index: i, Rule: r}
		for _, n := range p.imp.notes {
			if n.Why != "" && n.Warning == "" && n.Rule == i {
				rows[i].Why = n.Why
			}
		}
	}
	return rows
}

// ImportWarnings are the notes of the snippet that are warnings of the agent.
func (p *Types) ImportWarnings() []string {
	var out []string
	for _, n := range p.imp.notes {
		if n.Warning != "" {
			out = append(out, n.Warning)
		}
	}
	return out
}

// ImportPreview is the example path of the imported type.
func (p *Types) ImportPreview() Preview { return p.previewOf(p.imp.typ) }

// FindingText is the localized text of a finding: the message id
// "types.check.<code>" with Argument and Message as values, or the English
// Message when the catalog has no text for the code.
func (p *Types) FindingText(f analysis.Finding) string {
	id := "types.check." + f.Code
	if s := p.T(id, "Argument", f.Argument, "Message", f.Message, "Rule", strconv.Itoa(f.RuleIdx+1)); !strings.HasPrefix(s, id) {
		return s
	}
	return f.Message
}

// DryRun returns the dry-run table.
func (p *Types) DryRun() []DryRunRow {
	rows := make([]DryRunRow, len(p.imp.report.Samples))
	for i, s := range p.imp.report.Samples {
		others := make([]string, 0, len(s.Others))
		for _, o := range s.Others {
			others = append(others, p.typeName(o.TypeID)+" "+score(o.Score))
		}
		rows[i] = DryRunRow{Sample: s.Name, Score: score(s.Score), Confident: s.Confident, Others: strings.Join(others, ", "), Date: s.Date, Amount: s.Amount}
	}
	return rows
}

// LibraryHits returns the filed documents that now also score the new type.
func (p *Types) LibraryHits() []LibraryHitRow {
	rows := make([]LibraryHitRow, len(p.imp.report.Library))
	for i, h := range p.imp.report.Library {
		rows[i] = LibraryHitRow{Path: h.Path, TypeName: p.typeName(h.TypeID), Score: score(h.Score)}
	}
	return rows
}

func score(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func (p *Types) typeName(id string) string {
	if i := p.indexOf(id); i >= 0 && p.types[i].Name != "" {
		return p.types[i].Name
	}
	return id
}

// Collides reports whether the imported id already exists in the project.
func (p *Types) Collides() bool { return p.imp.hasType && p.indexOf(p.imp.typ.ID) >= 0 }

// CollisionChoices are the options when the id exists: a new id or replace.
func (p *Types) CollisionChoices() []Choice {
	return p.choices("types.import.collision.", CollisionNewID, CollisionReplace)
}

// SetCollision sets the collision choice (id or label).
func (p *Types) SetCollision(s string) error {
	id, err := p.parseChoice(p.CollisionChoices(), s)
	if err != nil {
		return err
	}
	p.imp.collision = id
	return nil
}

// Collision is the chosen collision handling.
func (p *Types) Collision() string { return p.imp.collision }

// SavedID is the id the imported type would be stored under.
func (p *Types) SavedID() string {
	if p.Collides() && p.imp.collision == CollisionNewID {
		return p.uniqueID(p.imp.typ.ID)
	}
	return p.imp.typ.ID
}

func (p *Types) uniqueID(base string) string {
	if s := slug(base); s != "" {
		base = s
	} else {
		base = "type"
	}
	id := base
	for n := 2; p.indexOf(id) >= 0; n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}

// CanSave reports whether the checked snippet may be stored: a report exists
// and has no error. The id collision is no blocker, the choice resolves it.
func (p *Types) CanSave() bool {
	if !p.imp.checked || !p.imp.hasType {
		return false
	}
	for _, f := range p.imp.report.Findings {
		if f.Level == analysis.LevelError && f.Code != codeIDCollision {
			return false
		}
	}
	return true
}

// ImportType stores the checked type with Store.SaveType (under a new unique
// id when the collision choice is "newID") and selects it in the editor. A
// draft with unsaved edits is replaced; ask the user first (Dirty). andEdit is
// for the caller: it reports that the editor should take focus afterwards.
func (p *Types) ImportType(ctx context.Context, andEdit bool) (string, error) {
	_ = andEdit
	if !p.CanSave() {
		return "", errors.New(p.T("types.import.err.blocked"))
	}
	t := cloneDocType(p.imp.typ)
	t.ID = p.SavedID()
	if strings.TrimSpace(t.Name) == "" {
		t.Name = t.ID
	}
	if err := p.store.SaveType(ctx, t); err != nil {
		return "", err
	}
	if i := p.indexOf(t.ID); i >= 0 {
		p.types[i] = cloneDocType(t)
	} else {
		p.types = append(p.types, cloneDocType(t))
	}
	p.isNew = false
	p.Select(t.ID)
	return t.ID, nil
}

// PrivacyHits lists the project's own identifiers (Project.IgnoreTokens) that
// occur in the snippet or in a sample text, each once. Spaces and case are
// ignored, so a spaced IBAN is found.
func (p *Types) PrivacyHits(ctx context.Context) []string {
	pr, err := p.store.Project(ctx)
	if err != nil {
		return nil
	}
	texts := []string{compact(p.imp.snippet)}
	for _, s := range p.imp.samples {
		texts = append(texts, compact(s.Text))
	}
	var hits []string
	for _, tok := range pr.IgnoreTokens {
		c := compact(tok)
		if c == "" || slices.Contains(hits, tok) {
			continue
		}
		for _, t := range texts {
			if strings.Contains(t, c) {
				hits = append(hits, tok)
				break
			}
		}
	}
	return hits
}

func compact(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), ""))
}

// TypeJSONOf is the JSON of the stored type with the given id (the bare
// DocumentType, to refine it with an LLM).
func (p *Types) TypeJSONOf(id string) (string, error) {
	i := p.indexOf(id)
	if i < 0 {
		if p.isNew && p.draft.ID == id {
			return p.typeJSON(p.draft)
		}
		return "", ErrNoType
	}
	return p.typeJSON(p.types[i])
}

// CopyTypeJSON is the JSON of the selected type (as stored; an unsaved new
// draft as typed).
func (p *Types) CopyTypeJSON() (string, error) {
	if !p.hasCur {
		return "", ErrNoType
	}
	return p.TypeJSONOf(p.draft.ID)
}

func (p *Types) typeJSON(t domain.DocumentType) (string, error) {
	b, err := p.Import.TypeJSON(t)
	return string(b), err
}

// PromptText is the LLM prompt with the project's current type ids appended.
func (p *Types) PromptText() string {
	ids := make([]string, len(p.types))
	for i, t := range p.types {
		ids[i] = t.ID
	}
	return p.Import.Prompt(ids)
}
