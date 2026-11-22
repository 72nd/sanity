package prompts

import (
	"strings"
	"testing"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/domain"
)

func TestEnumsMatchCode(t *testing.T) {
	p := TypeSetup(nil)
	if strings.Contains(p, "{{") {
		t.Fatal("unreplaced marker")
	}
	for field, vals := range map[string][]string{
		"kind": analysis.RuleKinds, "mode": analysis.RuleModes, "where": analysis.RuleWheres, "date.source": analysis.DateSources,
	} {
		want := "- `" + field + "`: `" + strings.Join(vals, "`, `") + "`"
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	for _, w := range []string{"0.2", "0.5", "0.9"} {
		if !strings.Contains(p, w) {
			t.Errorf("weight %s missing", w)
		}
	}
	for _, ph := range Placeholders {
		if !strings.Contains(p, ph[0]) {
			t.Errorf("placeholder %s missing", ph[0])
		}
	}
}

func TestPlaceholdersParse(t *testing.T) {
	for _, ph := range Placeholders {
		s := strings.NewReplacer("<layout>", "2006", "<label>", "Project").Replace(ph[0])
		if _, err := domain.ParseTemplate("a/" + s + ".pdf"); err != nil {
			t.Errorf("%s: %v", ph[0], err)
		}
	}
}

func TestExistingIDs(t *testing.T) {
	p := TypeSetup([]string{"a-b", "c"})
	if !strings.Contains(p, "- `a-b`\n- `c`\n") {
		t.Fatal(p[len(p)-200:])
	}
	if !strings.Contains(TypeSetup(nil), "no types yet") {
		t.Fatal("empty case")
	}
}

// workedExample returns the first fenced json block after the heading.
func workedExample(t *testing.T) string {
	t.Helper()
	_, rest, ok := strings.Cut(TypeSetup(nil), "## Worked example")
	if !ok {
		t.Fatal("no worked example")
	}
	_, rest, ok = strings.Cut(rest, "```json\n")
	if !ok {
		t.Fatal("no json block")
	}
	body, _, ok := strings.Cut(rest, "```")
	if !ok {
		t.Fatal("unclosed block")
	}
	return body
}

func TestWorkedExamplePasses(t *testing.T) {
	ty, notes, err := analysis.ParseSnippet([]byte(workedExample(t)))
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 3 || notes[2].Warning == "" {
		t.Fatalf("%+v", notes)
	}
	r := analysis.CheckType(analysis.CheckInput{Type: ty})
	if r.HasErrors() {
		t.Fatalf("%+v", r.Findings)
	}
	for _, f := range r.Findings {
		if f.Level == analysis.LevelWarning {
			t.Errorf("warning in the example: %+v", f)
		}
	}
	// and it works on a text like the described sample
	r = analysis.CheckType(analysis.CheckInput{Type: ty, Samples: []analysis.Sample{{Name: "s", Text: "Hetzner Online\nwww.hetzner.com\nRechnung\n\nUSt-IdNr. DE 812871812"}}})
	if !r.Samples[0].Confident {
		t.Fatalf("%+v", r.Samples)
	}
}
