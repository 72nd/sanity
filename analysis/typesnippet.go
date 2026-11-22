package analysis

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/72nd/sanity/domain"
)

// SnippetNote is an explanation from the agent that is shown in the import
// dialog and never stored.
type SnippetNote struct {
	Rule    int    `json:"rule"` // index into the type's rules, -1 when the note is about the type
	Why     string `json:"why,omitempty"`
	Warning string `json:"warning,omitempty"`
}

// weightValue decodes a number or one of the words weak, normal, strong.
type weightValue float64

func (w *weightValue) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "weak":
			*w = WeightWeak
		case "normal":
			*w = WeightNormal
		case "strong":
			*w = WeightStrong
		default:
			return fmt.Errorf("weight %q: use a number or weak, normal, strong", s)
		}
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("weight %s: use a number or weak, normal, strong", b)
	}
	*w = weightValue(f)
	return nil
}

// snippetRule is domain.Rule with Enabled as a pointer, to tell absent from false.
type snippetRule struct {
	Kind    string      `json:"kind"`
	Value   string      `json:"value"`
	Mode    string      `json:"mode"`
	Weight  weightValue `json:"weight"`
	Where   string      `json:"where"`
	Enabled *bool       `json:"enabled"`
}

type snippetType struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	PathTemplate string            `json:"pathTemplate"`
	Rules        []snippetRule     `json:"rules"`
	Threshold    float64           `json:"threshold"`
	UseLearned   *bool             `json:"useLearned"`
	Date         domain.DateConfig `json:"date"`
}

type snippetNote struct {
	Rule    *int   `json:"rule"`
	Why     string `json:"why"`
	Warning string `json:"warning"`
}

func (s snippetType) toDomain() domain.DocumentType {
	t := domain.DocumentType{
		ID: s.ID, Name: s.Name, PathTemplate: s.PathTemplate,
		Threshold: s.Threshold, UseLearned: s.UseLearned == nil || *s.UseLearned, Date: s.Date,
	}
	if t.Threshold == 0 {
		t.Threshold = defaultThreshold
	}
	if t.Date.Source == "" {
		t.Date.Source = "auto"
	}
	t.Rules = []domain.Rule{}
	for _, r := range s.Rules {
		t.Rules = append(t.Rules, domain.Rule{
			Kind: r.Kind, Value: r.Value, Mode: r.Mode, Weight: float64(r.Weight), Where: r.Where,
			Enabled: r.Enabled == nil || *r.Enabled,
		})
	}
	return t
}

// ParseSnippet reads the output of an agent: the envelope {"type":{...},
// "notes":[...]} or a bare document type. A fenced ```json block and prose
// around the single JSON object are tolerated. The import defaults are applied
// (enabled true, useLearned true, threshold 0.8, date source auto); weights may
// be numbers or the words weak, normal, strong. The type is not validated; use
// CheckType for that.
func ParseSnippet(data []byte) (domain.DocumentType, []SnippetNote, error) {
	obj, err := extractObject(data)
	if err != nil {
		return domain.DocumentType{}, nil, err
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(obj, &top); err != nil {
		return domain.DocumentType{}, nil, fmt.Errorf("snippet is not valid JSON: %w", err)
	}
	body := obj
	var notes []SnippetNote
	if raw, ok := top["type"]; ok {
		body = raw
		if rn, ok := top["notes"]; ok {
			var ns []snippetNote
			if err := json.Unmarshal(rn, &ns); err != nil {
				return domain.DocumentType{}, nil, fmt.Errorf("notes: %w", err)
			}
			for _, n := range ns {
				sn := SnippetNote{Rule: -1, Why: n.Why, Warning: n.Warning}
				if n.Rule != nil {
					sn.Rule = *n.Rule
				}
				notes = append(notes, sn)
			}
		}
	}
	var st snippetType
	if err := json.Unmarshal(body, &st); err != nil {
		return domain.DocumentType{}, nil, fmt.Errorf("type: %w", err)
	}
	return st.toDomain(), notes, nil
}

// extractObject returns the first balanced JSON object in data, skipping any
// prose and code fences before it.
func extractObject(data []byte) ([]byte, error) {
	start := bytes.IndexByte(data, '{')
	if start < 0 {
		return nil, errors.New("no JSON object found")
	}
	depth, inStr, esc := 0, false, false
	for i := start; i < len(data); i++ {
		c := data[i]
		switch {
		case inStr:
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
		case c == '"':
			inStr = true
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return data[start : i+1], nil
			}
		}
	}
	return nil, errors.New("JSON object is not closed")
}

// TypeJSON returns the type as indented JSON without an envelope, for "Copy as JSON".
func TypeJSON(t domain.DocumentType) ([]byte, error) {
	if t.Rules == nil {
		t.Rules = []domain.Rule{}
	}
	return json.MarshalIndent(t, "", "  ")
}
