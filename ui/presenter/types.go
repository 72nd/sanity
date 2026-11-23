package presenter

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/store"
)

// Errors of the types editor.
var (
	// ErrOutsideRoot marks a chosen folder that is not inside the project root.
	ErrOutsideRoot = errors.New("folder is outside the project root")
	// ErrNoType marks an action that needs a selected type.
	ErrNoType = errors.New("no type selected")
)

// Rule weights of the strength choices.
const (
	WeightWeak   = 0.2
	WeightNormal = 0.5
	WeightStrong = 0.9
)

// Choice is one option of a closed set: a stable id and its localized label.
type Choice struct {
	ID    string
	Label string
}

// TypeRow is one line of the type list.
type TypeRow struct {
	ID   string
	Name string
}

// RuleRow is one rule with its position, as the rule grid shows it.
type RuleRow struct {
	Index int
	Rule  domain.Rule
}

// LegendItem explains one Go layout token of a {date:...} placeholder.
type LegendItem struct {
	Token   string // for example "2006"
	Meaning string // localized
	Example string // the token applied to the example date
}

// Example holds the values the live preview evaluates the template with.
type Example struct {
	Date     time.Time
	Invoice  string
	Amount   string
	Original string
}

// Preview is the evaluated example path of the current template.
type Preview struct {
	Path    string   // the evaluated relative path, empty when Invalid
	Invalid bool     // the template does not parse, is empty or yields an unsafe path
	Escapes bool     // the path leaves the project root ("..")
	Missing []string // placeholders without a value in the example
	Message string   // localized explanation, empty when the path is fine
}

// Types is the presenter of the types editor. It edits a draft copy of one
// type; Save writes it back to the store.
type Types struct {
	store store.Store
	T     Translator

	root     string
	types    []domain.DocumentType // as stored
	draft    domain.DocumentType
	saved    domain.DocumentType // the draft as last loaded or saved; Dirty compares with it
	loadedAt time.Time
	isNew    bool // the draft is not in the store yet
	hasCur   bool
	example  Example

	// Import holds the headless pieces of the import flow (types_import.go).
	Import ImportFuncs
	imp    importSession
}

// NewTypes creates the presenter.
func NewTypes(s store.Store, t Translator) *Types {
	return &Types{store: s, T: t, Import: defaultImportFuncs(), example: Example{
		Date:     time.Date(2025, time.March, 12, 0, 0, 0, 0, time.UTC),
		Invoice:  "RE-2025-0042",
		Amount:   "49.95",
		Original: "scan_0042",
	}}
}

// Fresh reports whether the types were loaded a moment ago, so a page shown
// again right away needs no reload.
func (p *Types) Fresh() bool { return !p.loadedAt.IsZero() && time.Since(p.loadedAt) < 2*time.Second }

// Load fetches the project. Call it from a goroutine. The selection survives
// when the type still exists; otherwise the first type is selected. Unsaved
// edits are discarded.
func (p *Types) Load(ctx context.Context) error {
	pr, err := p.store.Project(ctx)
	if err != nil {
		return err
	}
	p.root, p.types, p.loadedAt = pr.Root, pr.Types, time.Now()
	switch {
	case p.isNew:
		// keep the unsaved draft
	case p.hasCur && p.indexOf(p.draft.ID) >= 0:
		p.Select(p.draft.ID)
	case len(p.types) > 0:
		p.Select(p.types[0].ID)
	default:
		p.hasCur = false
		p.draft, p.saved = domain.DocumentType{}, domain.DocumentType{}
	}
	return nil
}

// Root is the absolute project folder, the start of the folder picker.
func (p *Types) Root() string { return p.root }

// List returns the stored types and, last, the unsaved new draft.
func (p *Types) List() []TypeRow {
	rows := make([]TypeRow, 0, len(p.types)+1)
	for _, t := range p.types {
		rows = append(rows, TypeRow{ID: t.ID, Name: t.Name})
	}
	if p.isNew {
		rows = append(rows, TypeRow{ID: p.draft.ID, Name: p.displayName()})
	} else if i := p.indexOf(p.draft.ID); i >= 0 {
		rows[i].Name = p.displayName() // show the name while it is typed
	}
	return rows
}

func (p *Types) displayName() string {
	if strings.TrimSpace(p.draft.Name) == "" {
		return p.T("types.unnamed")
	}
	return p.draft.Name
}

func (p *Types) indexOf(id string) int {
	return slices.IndexFunc(p.types, func(t domain.DocumentType) bool { return t.ID == id })
}

// Select makes the type with the given id the draft. It reports false for an
// unknown id. Unsaved edits are discarded; ask the user first (Dirty).
func (p *Types) Select(id string) bool {
	if p.isNew && id == p.draft.ID {
		return true
	}
	i := p.indexOf(id)
	if i < 0 {
		return false
	}
	p.draft, p.saved = cloneDocType(p.types[i]), cloneDocType(p.types[i])
	p.isNew, p.hasCur = false, true
	return true
}

// Current returns the draft.
func (p *Types) Current() (domain.DocumentType, bool) { return cloneDocType(p.draft), p.hasCur }

// CurrentID is the id of the draft, "" without one.
func (p *Types) CurrentID() string {
	if !p.hasCur {
		return ""
	}
	return p.draft.ID
}

// Dirty reports unsaved edits (a new type counts as dirty until it is saved).
func (p *Types) Dirty() bool {
	return p.hasCur && (p.isNew || !reflect.DeepEqual(normType(p.draft), normType(p.saved)))
}

// StateText is the localized "saved / unsaved changes" line.
func (p *Types) StateText() string {
	if !p.hasCur {
		return ""
	}
	if p.Dirty() {
		return p.T("types.state.dirty")
	}
	return p.T("types.state.saved")
}

// Revert drops unsaved edits. A new, unsaved type is removed.
func (p *Types) Revert() {
	if p.isNew {
		p.isNew, p.hasCur = false, false
		if len(p.types) > 0 {
			p.Select(p.types[0].ID)
		}
		return
	}
	p.Select(p.draft.ID)
}

// Add starts a new, empty type as the draft. It is stored by Save.
func (p *Types) Add() {
	p.startDraft(domain.DocumentType{
		Name:       "",
		Threshold:  0.8,
		UseLearned: true,
		Date:       domain.DateConfig{Source: "auto", MaxAgeYears: 3},
	})
}

// Duplicate starts a new draft that copies the current type.
func (p *Types) Duplicate() error {
	if !p.hasCur {
		return ErrNoType
	}
	c := cloneDocType(p.draft)
	c.Name = p.T("types.copy.name", "Name", p.draft.Name)
	p.startDraft(c)
	return nil
}

func (p *Types) startDraft(t domain.DocumentType) {
	t.ID = p.freshID(t.Name)
	p.draft, p.saved = t, domain.DocumentType{}
	p.isNew, p.hasCur = true, true
}

// freshID derives an unused id from name.
func (p *Types) freshID(name string) string {
	base := slug(name)
	if base == "" {
		base = "type"
	}
	id := base
	for n := 2; p.indexOf(id) >= 0; n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}

// Delete removes the current type from the store (an unsaved draft is just
// dropped). Library entries of the type are kept.
func (p *Types) Delete(ctx context.Context) error {
	if !p.hasCur {
		return ErrNoType
	}
	if p.isNew {
		p.Revert()
		return nil
	}
	id := p.draft.ID
	if err := p.store.DeleteType(ctx, id); err != nil {
		return err
	}
	if i := p.indexOf(id); i >= 0 {
		p.types = slices.Delete(p.types, i, i+1)
	}
	p.hasCur = false
	if len(p.types) > 0 {
		p.Select(p.types[0].ID)
	} else {
		p.draft, p.saved = domain.DocumentType{}, domain.DocumentType{}
	}
	return nil
}

// Save validates the draft and stores it. It returns the first problem as a
// localized error.
func (p *Types) Save(ctx context.Context) error {
	if !p.hasCur {
		return ErrNoType
	}
	if err := p.Validate(); err != nil {
		return err
	}
	t := cloneDocType(p.draft)
	t.Name = strings.TrimSpace(t.Name)
	if p.isNew { // the id follows the name the user chose
		t.ID = p.freshID(t.Name)
	}
	if err := p.store.SaveType(ctx, t); err != nil {
		return err
	}
	if i := p.indexOf(t.ID); i >= 0 {
		p.types[i] = cloneDocType(t)
	} else {
		p.types = append(p.types, cloneDocType(t))
	}
	p.draft, p.saved, p.isNew = t, cloneDocType(t), false
	return nil
}

// Validate checks the draft: a name, a valid path template and valid rules.
func (p *Types) Validate() error {
	if strings.TrimSpace(p.draft.Name) == "" {
		return errors.New(p.T("types.err.name"))
	}
	if pv := p.Preview(); pv.Invalid {
		return errors.New(pv.Message)
	}
	for i, r := range p.draft.Rules {
		if err := p.checkRuleValue(r.Kind, r.Value); err != nil {
			return fmt.Errorf("%s %d: %w", p.T("types.rule"), i+1, err)
		}
	}
	return nil
}

// SetName edits the name.
func (p *Types) SetName(s string) { p.draft.Name = s }

// SetPathTemplate edits the path template.
func (p *Types) SetPathTemplate(s string) { p.draft.PathTemplate = s }

// SetThreshold edits the preselect threshold, clamped to 0..1.
func (p *Types) SetThreshold(v float64) { p.draft.Threshold = min(max(v, 0), 1) }

// SetUseLearned switches the learned score on or off.
func (p *Types) SetUseLearned(on bool) { p.draft.UseLearned = on }

// SetPreferredLabels sets the preferred date labels from comma separated text.
func (p *Types) SetPreferredLabels(s string) { p.draft.Date.PreferredLabels = SplitList(s) }

// SetAmountLabels sets the preferred amount labels from comma separated text.
func (p *Types) SetAmountLabels(s string) { p.draft.Amount.PreferredLabels = SplitList(s) }

// SetStatement marks the type as a statement type (Kontoauszug).
func (p *Types) SetStatement(on bool) { p.draft.IsStatement = on }

// SetCurrency sets the default currency by code or label; "" or the label of
// "none" clears it.
func (p *Types) SetCurrency(s string) error {
	id, err := p.parseChoice(p.CurrencyChoices(), s)
	if err != nil {
		return err
	}
	p.draft.Currency = id
	return nil
}

// CurrencyChoices are "none" and the supported currency codes.
func (p *Types) CurrencyChoices() []Choice {
	out := []Choice{{ID: "", Label: p.T("types.currency.none")}}
	for _, c := range analysis.Currencies {
		out = append(out, Choice{ID: c, Label: c})
	}
	return out
}

// SetIgnoreLabels sets the ignored date labels from comma separated text.
func (p *Types) SetIgnoreLabels(s string) { p.draft.Date.IgnoreLabels = SplitList(s) }

// SetDateSource sets the date source by id or localized label.
func (p *Types) SetDateSource(s string) error {
	id, err := p.parseChoice(p.SourceChoices(), s)
	if err != nil {
		return err
	}
	p.draft.Date.Source = id
	return nil
}

// SetMaxAgeYears sets the maximum plausible document age, never negative.
func (p *Types) SetMaxAgeYears(n int) { p.draft.Date.MaxAgeYears = max(n, 0) }

// JoinList is the comma separated form of a label list.
func JoinList(l []string) string { return strings.Join(l, ", ") }

// SplitList splits comma separated text into trimmed, non-empty entries.
func SplitList(s string) []string {
	out := []string{}
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// SourceChoices are the date sources.
func (p *Types) SourceChoices() []Choice {
	return p.choices("types.source.", "auto", "text", "filename", "metadata")
}

// KindChoices are the rule kinds.
func (p *Types) KindChoices() []Choice {
	return p.choices("types.kind.", "keyword", "regex", "iban", "vatId", "customerNo", "domain", "filename")
}

// ModeChoices are the rule modes.
func (p *Types) ModeChoices() []Choice { return p.choices("types.mode.", "should", "must", "not") }

// StrengthChoices are the rule strengths.
func (p *Types) StrengthChoices() []Choice {
	return p.choices("types.strength.", "weak", "normal", "strong")
}

// WhereChoices are the places a rule looks at.
func (p *Types) WhereChoices() []Choice {
	return p.choices("types.where.", "any", "header", "footer", "filename")
}

func (p *Types) choices(prefix string, ids ...string) []Choice {
	out := make([]Choice, len(ids))
	for i, id := range ids {
		out[i] = Choice{ID: id, Label: p.T(prefix + id)}
	}
	return out
}

// LabelOf returns the label of the choice with the given id (the id itself
// when unknown).
func LabelOf(cs []Choice, id string) string {
	for _, c := range cs {
		if c.ID == id {
			return c.Label
		}
	}
	return id
}

func (p *Types) parseChoice(cs []Choice, s string) (string, error) {
	s = strings.TrimSpace(s)
	for _, c := range cs {
		if strings.EqualFold(c.ID, s) || strings.EqualFold(c.Label, s) {
			return c.ID, nil
		}
	}
	labels := make([]string, len(cs))
	for i, c := range cs {
		labels[i] = c.Label
	}
	return "", errors.New(p.T("types.err.choice", "Options", strings.Join(labels, ", ")))
}

// StrengthID maps a weight to weak, normal or strong (the nearest preset).
func StrengthID(w float64) string {
	switch {
	case w < (WeightWeak+WeightNormal)/2:
		return "weak"
	case w < (WeightNormal+WeightStrong)/2:
		return "normal"
	}
	return "strong"
}

// StrengthWeight maps weak, normal or strong to the rule weight (normal for
// anything else).
func StrengthWeight(id string) float64 {
	switch id {
	case "weak":
		return WeightWeak
	case "strong":
		return WeightStrong
	}
	return WeightNormal
}

// Rules returns the draft's rules with their positions.
func (p *Types) Rules() []RuleRow {
	rows := make([]RuleRow, len(p.draft.Rules))
	for i, r := range p.draft.Rules {
		rows[i] = RuleRow{Index: i, Rule: r}
	}
	return rows
}

// AddRule appends an enabled, empty keyword rule and returns its index.
func (p *Types) AddRule() int {
	p.draft.Rules = append(p.draft.Rules, domain.Rule{
		Kind: "keyword", Mode: "should", Weight: WeightNormal, Where: "any", Enabled: true,
	})
	return len(p.draft.Rules) - 1
}

// RemoveRule deletes the rule at index i.
func (p *Types) RemoveRule(i int) {
	if i >= 0 && i < len(p.draft.Rules) {
		p.draft.Rules = slices.Delete(p.draft.Rules, i, i+1)
	}
}

// SetRuleKind sets the kind (id or label). A regex value that no longer
// fits is reported by Save, not here.
func (p *Types) SetRuleKind(i int, s string) error {
	return p.setRuleChoice(i, p.KindChoices(), s, func(r *domain.Rule, id string) { r.Kind = id })
}

// SetRuleMode sets should, must or not (id or label).
func (p *Types) SetRuleMode(i int, s string) error {
	return p.setRuleChoice(i, p.ModeChoices(), s, func(r *domain.Rule, id string) { r.Mode = id })
}

// SetRuleStrength sets weak, normal or strong (id or label) and maps it to Weight.
func (p *Types) SetRuleStrength(i int, s string) error {
	return p.setRuleChoice(i, p.StrengthChoices(), s, func(r *domain.Rule, id string) { r.Weight = StrengthWeight(id) })
}

// SetRuleWhere sets where the rule looks (id or label).
func (p *Types) SetRuleWhere(i int, s string) error {
	return p.setRuleChoice(i, p.WhereChoices(), s, func(r *domain.Rule, id string) { r.Where = id })
}

func (p *Types) setRuleChoice(i int, cs []Choice, s string, set func(*domain.Rule, string)) error {
	if i < 0 || i >= len(p.draft.Rules) {
		return ErrNoType
	}
	id, err := p.parseChoice(cs, s)
	if err != nil {
		return err
	}
	set(&p.draft.Rules[i], id)
	return nil
}

// SetRuleEnabled switches a rule on or off.
func (p *Types) SetRuleEnabled(i int, on bool) {
	if i >= 0 && i < len(p.draft.Rules) {
		p.draft.Rules[i].Enabled = on
	}
}

// SetRuleValue sets the rule's value. A regex rule must compile, and no value
// may be empty; on error the value is not changed.
func (p *Types) SetRuleValue(i int, s string) error {
	if i < 0 || i >= len(p.draft.Rules) {
		return ErrNoType
	}
	s = strings.TrimSpace(s)
	if err := p.checkRuleValue(p.draft.Rules[i].Kind, s); err != nil {
		return err
	}
	p.draft.Rules[i].Value = s
	return nil
}

func (p *Types) checkRuleValue(kind, v string) error {
	if strings.TrimSpace(v) == "" {
		return errors.New(p.T("types.err.rule.empty"))
	}
	if kind == "regex" {
		if _, err := regexp.Compile(v); err != nil {
			return errors.New(p.T("types.err.rule.regex", "Error", err.Error()))
		}
	}
	return nil
}

// SetExample replaces the example values of the preview.
func (p *Types) SetExample(e Example) { p.example = e }

// ExampleValues returns the example values of the preview.
func (p *Types) ExampleValues() Example { return p.example }

// Preview evaluates the draft's path template with the example values.
func (p *Types) Preview() Preview { return p.previewOf(p.draft) }

// previewOf evaluates the path template of t with the example values.
func (p *Types) previewOf(t domain.DocumentType) Preview {
	tpl, err := domain.ParseTemplate(t.PathTemplate)
	if err != nil {
		return Preview{Invalid: true, Message: p.T("types.preview.invalid", "Reason", reason(err))}
	}
	if err := tpl.Validate(); err != nil {
		return p.failed(err, nil)
	}
	v := domain.Values{
		Date: p.example.Date, Type: t.Name, Invoice: p.example.Invoice,
		Amount: p.example.Amount, Original: p.example.Original, Text: map[string]string{},
	}
	for _, m := range tpl.Missing(v) {
		if label, ok := strings.CutPrefix(m, "text:"); ok {
			v.Text[label] = label // sample text for the form-asked fields
		}
	}
	missing := tpl.Missing(v)
	rel, err := tpl.Evaluate(v)
	if errors.Is(err, domain.ErrMissingValue) {
		return Preview{Invalid: true, Missing: missing,
			Message: p.T("types.preview.missing", "Fields", strings.Join(missing, ", "))}
	}
	if err != nil {
		return p.failed(err, missing)
	}
	return Preview{Path: rel}
}

func (p *Types) failed(err error, missing []string) Preview {
	pv := Preview{Invalid: true, Missing: missing}
	if errors.Is(err, domain.ErrUnsafePath) && strings.Contains(err.Error(), "escapes") {
		pv.Escapes = true
		pv.Message = p.T("types.preview.escapes")
		return pv
	}
	pv.Message = p.T("types.preview.invalid", "Reason", reason(err))
	return pv
}

// reason strips the sentinel prefix of a domain error ("invalid path template: ").
func reason(err error) string {
	s := err.Error()
	for _, e := range []error{domain.ErrInvalidTemplate, domain.ErrUnsafePath, domain.ErrMissingValue} {
		s = strings.TrimPrefix(s, e.Error()+": ")
	}
	return s
}

// Legend lists the Go layout tokens for {date:...}, with an example each.
func (p *Types) Legend() []LegendItem {
	items := []struct{ token, id string }{
		{"2006", "year"}, {"06", "year2"}, {"01", "month"}, {"1", "month1"}, {"Jan", "monthShort"},
		{"January", "monthLong"}, {"02", "day"}, {"2", "day1"}, {"Mon", "weekdayShort"}, {"Monday", "weekdayLong"},
	}
	out := make([]LegendItem, len(items))
	for i, it := range items {
		out[i] = LegendItem{Token: it.token, Meaning: p.T("types.legend." + it.id), Example: p.example.Date.Format(it.token)}
	}
	return out
}

// LegendText is the legend on one line, for example "2006 = 2025, 01 = 03".
func (p *Types) LegendText() string {
	parts := []string{}
	for _, it := range p.Legend() {
		parts = append(parts, fmt.Sprintf("%s %s (%s)", it.Token, it.Meaning, it.Example))
	}
	return strings.Join(parts, " · ")
}

// InsertFolder replaces the folder part of the template with the absolute
// folder abs, made relative to the project root. The file name part (after the
// last "/" outside placeholders) is kept. It returns the new template.
func (p *Types) InsertFolder(abs string) (string, error) {
	rel, err := filepath.Rel(p.root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrOutsideRoot
	}
	rel = filepath.ToSlash(rel)
	file := fileName(p.draft.PathTemplate)
	if rel != "." {
		file = path.Join(rel, file)
	}
	p.draft.PathTemplate = file
	return file, nil
}

// fileName returns the part of the template after the last "/" that is not
// inside braces (a date layout may contain "/").
func fileName(tpl string) string {
	depth, cut := 0, -1
	for i := 0; i < len(tpl); i++ {
		switch tpl[i] {
		case '{':
			depth++
		case '}':
			depth = max(depth-1, 0)
		case '/':
			if depth == 0 {
				cut = i
			}
		}
	}
	return tpl[cut+1:]
}

// slug makes a lower-case id from a name.
func slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		default:
			dash = true
		}
	}
	return b.String()
}

func cloneDocType(t domain.DocumentType) domain.DocumentType {
	t.Rules = slices.Clone(t.Rules)
	t.Date.PreferredLabels = slices.Clone(t.Date.PreferredLabels)
	t.Date.IgnoreLabels = slices.Clone(t.Date.IgnoreLabels)
	t.Amount.PreferredLabels = slices.Clone(t.Amount.PreferredLabels)
	return t
}

// normType makes nil and empty slices equal for the dirty check.
func normType(t domain.DocumentType) domain.DocumentType {
	t = cloneDocType(t)
	if len(t.Rules) == 0 {
		t.Rules = nil
	}
	if len(t.Date.PreferredLabels) == 0 {
		t.Date.PreferredLabels = nil
	}
	if len(t.Date.IgnoreLabels) == 0 {
		t.Date.IgnoreLabels = nil
	}
	if len(t.Amount.PreferredLabels) == 0 {
		t.Amount.PreferredLabels = nil
	}
	return t
}
