package domain

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode"
)

// Errors returned by template parsing, evaluation and validation. Use errors.Is to test them.
var (
	// ErrInvalidTemplate marks a template that does not parse.
	ErrInvalidTemplate = errors.New("invalid path template")
	// ErrMissingValue marks a placeholder that has no usable value.
	ErrMissingValue = errors.New("missing value")
	// ErrUnsafePath marks an evaluated path that is absolute, escapes the root or contains unsafe characters.
	ErrUnsafePath = errors.New("unsafe path")
)

// Values holds the data a path template is evaluated with.
type Values struct {
	Date     time.Time
	Type     string
	Invoice  string
	Amount   string
	Original string
	Text     map[string]string // keyed by the label of {text:<label>}
}

// Template is a parsed path template, for example
// receipts/{date:2006}/telecom/{date:2006-01-02}_telekom_{invoice}.pdf.
type Template struct {
	raw      string
	segments []segment
}

type placeholderKind int

const (
	phLiteral placeholderKind = iota
	phDate
	phType
	phInvoice
	phAmount
	phOriginal
	phText
)

// segment is either literal text or a placeholder. For phDate, text is the Go layout;
// for phText, it is the label; for literals, it is the literal text itself.
type segment struct {
	kind placeholderKind
	text string
}

// simplePlaceholders maps the placeholders that take no argument to their kind.
var simplePlaceholders = map[string]placeholderKind{
	"type":     phType,
	"invoice":  phInvoice,
	"amount":   phAmount,
	"original": phOriginal,
}

// unsafeNameChars are characters that a file name must not contain. The path separator "/"
// is handled separately, because it is allowed in template literals and date layouts.
const unsafeNameChars = `\:*?"<>|`

// ParseTemplate parses a path template. Braces must be balanced, and every placeholder must be known.
func ParseTemplate(s string) (Template, error) {
	var segs []segment
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			segs = append(segs, segment{kind: phLiteral, text: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '}':
			return Template{}, fmt.Errorf("%w: unbalanced } at byte %d", ErrInvalidTemplate, i)
		case '{':
			end := strings.IndexAny(s[i+1:], "{}")
			if end < 0 {
				return Template{}, fmt.Errorf("%w: unclosed { at byte %d", ErrInvalidTemplate, i)
			}
			end += i + 1
			if s[end] == '{' {
				return Template{}, fmt.Errorf("%w: nested { at byte %d", ErrInvalidTemplate, end)
			}
			seg, err := parsePlaceholder(s[i+1 : end])
			if err != nil {
				return Template{}, err
			}
			flush()
			segs = append(segs, seg)
			i = end
		default:
			lit.WriteByte(s[i])
		}
	}
	flush()
	return Template{raw: s, segments: segs}, nil
}

// parsePlaceholder parses the text between the braces, for example "date:2006" or "invoice".
func parsePlaceholder(body string) (segment, error) {
	name, arg, hasArg := strings.Cut(body, ":")
	switch name {
	case "date":
		if !hasArg || arg == "" {
			return segment{}, fmt.Errorf("%w: {date} needs a Go layout, for example {date:2006-01-02}", ErrInvalidTemplate)
		}
		return segment{kind: phDate, text: arg}, nil
	case "text":
		if !hasArg || arg == "" {
			return segment{}, fmt.Errorf("%w: {text} needs a label, for example {text:Project}", ErrInvalidTemplate)
		}
		return segment{kind: phText, text: arg}, nil
	}
	kind, ok := simplePlaceholders[name]
	if !ok {
		return segment{}, fmt.Errorf("%w: unknown placeholder {%s}", ErrInvalidTemplate, body)
	}
	if hasArg {
		return segment{}, fmt.Errorf("%w: {%s} takes no argument", ErrInvalidTemplate, name)
	}
	return segment{kind: kind}, nil
}

// String returns the template as it was written.
func (t Template) String() string {
	return t.raw
}

// Evaluate fills the template with v and returns the relative path, always with "/" as separator.
// It fails with ErrMissingValue when a placeholder has no value, and with ErrUnsafePath when the
// result is absolute, escapes the root or contains unsafe characters.
func (t Template) Evaluate(v Values) (string, error) {
	if missing := t.Missing(v); len(missing) > 0 {
		return "", fmt.Errorf("%w: %s", ErrMissingValue, strings.Join(missing, ", "))
	}
	p := t.render(v)
	if err := checkPath(p); err != nil {
		return "", err
	}
	return p, nil
}

// Missing lists the placeholders that have no value in v, for example "invoice" or "text:Project".
// Each name appears once, in the order of the template.
func (t Template) Missing(v Values) []string {
	var out []string
	seen := make(map[string]bool)
	for _, s := range t.segments {
		if s.kind == phLiteral || !s.empty(v) {
			continue
		}
		name := s.name()
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// Validate checks the template for paths that can never be valid: absolute paths, ".." segments,
// empty segments and unsafe characters. It evaluates the template with sample values to do so.
func (t Template) Validate() error {
	parsed, err := ParseTemplate(t.raw)
	if err != nil {
		return err
	}
	if len(parsed.segments) == 0 {
		return fmt.Errorf("%w: empty template", ErrInvalidTemplate)
	}
	_, err = parsed.Evaluate(sampleValues(parsed))
	return err
}

// render substitutes all placeholders. It does not check the result; Evaluate does.
func (t Template) render(v Values) string {
	var b strings.Builder
	for _, s := range t.segments {
		switch s.kind {
		case phLiteral:
			b.WriteString(s.text)
		case phDate:
			b.WriteString(sanitize(v.Date.Format(s.text), true))
		case phType:
			b.WriteString(safeValue(v.Type))
		case phInvoice:
			b.WriteString(safeValue(v.Invoice))
		case phAmount:
			b.WriteString(safeValue(v.Amount))
		case phOriginal:
			b.WriteString(safeValue(v.Original))
		case phText:
			b.WriteString(safeValue(v.Text[s.text]))
		}
	}
	return b.String()
}

// empty reports whether the placeholder has no usable value in v.
func (s segment) empty(v Values) bool {
	switch s.kind {
	case phDate:
		return v.Date.IsZero()
	case phType:
		return safeValue(v.Type) == ""
	case phInvoice:
		return safeValue(v.Invoice) == ""
	case phAmount:
		return safeValue(v.Amount) == ""
	case phOriginal:
		return safeValue(v.Original) == ""
	case phText:
		return safeValue(v.Text[s.text]) == ""
	}
	return false
}

// name is the placeholder as written in the template, without braces.
func (s segment) name() string {
	switch s.kind {
	case phDate:
		return "date:" + s.text
	case phText:
		return "text:" + s.text
	case phType:
		return "type"
	case phInvoice:
		return "invoice"
	case phAmount:
		return "amount"
	case phOriginal:
		return "original"
	}
	return ""
}

// sampleValues returns values that fill every placeholder of t, used by Validate.
func sampleValues(t Template) Values {
	v := Values{
		Date:     time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
		Type:     "x",
		Invoice:  "x",
		Amount:   "x",
		Original: "x",
		Text:     make(map[string]string),
	}
	for _, s := range t.segments {
		if s.kind == phText {
			v.Text[s.text] = "x"
		}
	}
	return v
}

// safeValue makes a placeholder value safe to use as part of a file name. Slashes and other
// unsafe characters become "_"; spaces and dots at both ends are trimmed.
func safeValue(s string) string {
	return strings.Trim(sanitize(s, false), " .")
}

// sanitize replaces unsafe characters and control characters with "_". With allowSlash, "/" is kept.
func sanitize(s string, allowSlash bool) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '/' && allowSlash:
			return r
		case r == '/', unicode.IsControl(r), strings.ContainsRune(unsafeNameChars, r):
			return '_'
		}
		return r
	}, s)
}

// checkPath rejects evaluated paths that are not clean relative paths.
func checkPath(p string) error {
	switch {
	case p == "":
		return fmt.Errorf("%w: empty path", ErrUnsafePath)
	case strings.HasPrefix(p, "/"):
		return fmt.Errorf("%w: absolute path %q", ErrUnsafePath, p)
	}
	for _, seg := range strings.Split(p, "/") {
		switch {
		case seg == "":
			return fmt.Errorf("%w: empty segment in %q", ErrUnsafePath, p)
		case seg == "..":
			return fmt.Errorf("%w: %q escapes the project root", ErrUnsafePath, p)
		}
		for _, r := range seg {
			if unicode.IsControl(r) || strings.ContainsRune(unsafeNameChars, r) {
				return fmt.Errorf("%w: unsafe character %q in %q", ErrUnsafePath, r, p)
			}
		}
	}
	return nil
}

// UniquePath returns rel if exists reports false for it. Otherwise it inserts _2, _3, ... before
// the extension until it finds a free name. The file is never overwritten.
func UniquePath(rel string, exists func(string) bool) string {
	if !exists(rel) {
		return rel
	}
	ext := path.Ext(rel)
	base := strings.TrimSuffix(rel, ext)
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s_%d%s", base, n, ext)
		if !exists(candidate) {
			return candidate
		}
	}
}
