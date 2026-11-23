package presenter

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/store"
)

// Problems holds the localized validation messages of the project form; an
// empty field is valid.
type Problems struct {
	Name, Inbox, Ignore string
}

// Any reports whether a field is invalid.
func (p Problems) Any() bool { return p.Name != "" || p.Inbox != "" || p.Ignore != "" }

// ErrInvalidProject is returned by Save while a field is invalid.
var ErrInvalidProject = errors.New("project settings are invalid")

// Project is the presenter of the project page: it edits the name, the inbox
// folder and the own identifiers of the open project.
type Project struct {
	store store.Store
	T     Translator

	root   string
	draft  projectForm
	saved  projectForm
	loaded bool
}

type projectForm struct {
	Name, Inbox, Ignore string // Ignore is one token per line
	Statement           string // statement keywords, one per line
}

// NewProject creates the presenter.
func NewProject(s store.Store, t Translator) *Project { return &Project{store: s, T: t} }

// Load fetches the project; unsaved edits are discarded. Call it from a goroutine.
func (p *Project) Load(ctx context.Context) error {
	pr, err := p.store.Project(ctx)
	if err != nil {
		return err
	}
	p.root = pr.Root
	inbox := pr.Inbox
	if inbox == "" {
		inbox = domain.DefaultInbox
	}
	p.draft = projectForm{Name: pr.Name, Inbox: inbox, Ignore: strings.Join(pr.IgnoreTokens, "\n"),
		Statement: strings.Join(pr.StatementWords(), "\n")}
	p.saved = p.draft
	p.loaded = true
	return nil
}

// Root is the absolute project folder (read-only).
func (p *Project) Root() string { return p.root }

// Name, Inbox and IgnoreText are the draft values; Inbox is relative to Root.
func (p *Project) Name() string       { return p.draft.Name }
func (p *Project) Inbox() string      { return p.draft.Inbox }
func (p *Project) IgnoreText() string { return p.draft.Ignore }

// StatementText is the draft of the statement keywords, one per line.
func (p *Project) StatementText() string { return p.draft.Statement }

// SetStatementText changes the statement keywords; commas and line breaks
// separate them.
func (p *Project) SetStatementText(s string) { p.draft.Statement = s }

// Keywords returns the statement keywords of the draft: trimmed, without
// empty entries and duplicates, in order.
func (p *Project) Keywords() []string { return splitKeywords(p.draft.Statement) }

func splitKeywords(text string) []string {
	return splitTokens(strings.ReplaceAll(text, ",", "\n"))
}

// InboxPath is the absolute inbox folder of the draft.
func (p *Project) InboxPath() string { return filepath.Join(p.root, filepath.FromSlash(p.draft.Inbox)) }

// SetName, SetInbox and SetIgnoreText change the draft.
func (p *Project) SetName(s string)       { p.draft.Name = s }
func (p *Project) SetInbox(s string)      { p.draft.Inbox = s }
func (p *Project) SetIgnoreText(s string) { p.draft.Ignore = s }

// SetInboxFolder sets the inbox from an absolute folder chosen in the folder
// picker. A folder outside the root yields ErrOutsideRoot and changes nothing.
func (p *Project) SetInboxFolder(abs string) (string, error) {
	rel, err := filepath.Rel(p.root, abs)
	if err != nil || !filepath.IsLocal(rel) && rel != "." {
		return "", ErrOutsideRoot
	}
	rel = filepath.ToSlash(rel)
	if rel == "." { // the root itself would watch the whole project
		return "", ErrOutsideRoot
	}
	p.draft.Inbox = rel
	return rel, nil
}

var reIBANLike = regexp.MustCompile(`^[A-Za-z]{2}\d{2}[A-Za-z0-9 ]{11,30}$`)

// Tokens returns the own identifiers of the draft: trimmed, without empty
// lines and duplicates, in order.
func (p *Project) Tokens() []string { return splitTokens(p.draft.Ignore) }

func splitTokens(text string) []string {
	out := []string{}
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	return out
}

// Problems validates the draft. The name must not be empty, the inbox must be a
// relative folder inside the root, and a token that looks like an IBAN must
// pass the mod 97 check.
func (p *Project) Problems() Problems {
	var pr Problems
	if strings.TrimSpace(p.draft.Name) == "" {
		pr.Name = p.T("project.error.name")
	}
	in := strings.TrimSpace(p.draft.Inbox)
	if in != "" && (filepath.IsAbs(in) || !filepath.IsLocal(filepath.FromSlash(in)) || filepath.Clean(in) == ".") {
		pr.Inbox = p.T("project.error.inbox")
	}
	for i, l := range strings.Split(p.draft.Ignore, "\n") {
		l = strings.TrimSpace(l)
		if reIBANLike.MatchString(l) && !analysis.IBANValid(l) {
			pr.Ignore = p.T("project.error.iban", "Line", strconv.Itoa(i+1), "Value", l)
			break
		}
	}
	return pr
}

// Dirty reports unsaved edits.
func (p *Project) Dirty() bool { return p.loaded && p.normalized(p.draft) != p.normalized(p.saved) }

func (p *Project) normalized(f projectForm) projectForm {
	f.Name = strings.TrimSpace(f.Name)
	f.Inbox = strings.TrimSpace(f.Inbox)
	f.Ignore = strings.Join(splitTokens(f.Ignore), "\n")
	f.Statement = strings.Join(splitKeywords(f.Statement), "\n")
	return f
}

// StateText is the localized "saved / unsaved changes" line.
func (p *Project) StateText() string {
	if !p.loaded {
		return ""
	}
	if p.Dirty() {
		return p.T("project.state.dirty")
	}
	return p.T("project.state.saved")
}

// Revert drops unsaved edits.
func (p *Project) Revert() { p.draft = p.saved }

// Save validates and writes the draft to the store. inboxChanged tells the
// caller that the folder to watch is another one now. Call it from a goroutine.
func (p *Project) Save(ctx context.Context) (inboxChanged bool, err error) {
	if p.Problems().Any() {
		return false, ErrInvalidProject
	}
	f := p.normalized(p.draft)
	if f.Inbox == "" {
		f.Inbox = domain.DefaultInbox
	}
	f.Inbox = filepath.ToSlash(filepath.Clean(filepath.FromSlash(f.Inbox)))
	if err := p.store.SaveProject(ctx, f.Name, f.Inbox, splitTokens(f.Ignore)); err != nil {
		return false, err
	}
	if err := p.store.SaveStatementKeywords(ctx, splitKeywords(f.Statement)); err != nil {
		return false, err
	}
	inboxChanged = f.Inbox != p.normalized(p.saved).Inbox
	p.draft = f
	p.saved = f
	return inboxChanged, nil
}
