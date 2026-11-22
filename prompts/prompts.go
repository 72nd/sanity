// Package prompts holds the texts sanity hands to an LLM. The text is embedded
// and its enum values are generated from the analysis package, so it cannot
// drift from the code. Imports no UI.
package prompts

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/72nd/sanity/analysis"
)

//go:embed type-setup.md
var typeSetup string

// Placeholders lists the path template placeholders with their meaning.
var Placeholders = [][2]string{
	{"{date:<layout>}", "document date in a Go layout"},
	{"{type}", "name of the type, made safe for a file name"},
	{"{invoice}", "detected invoice number"},
	{"{amount}", "detected total, for example `49.95`"},
	{"{original}", "original file name without `.pdf`"},
	{"{text:<label>}", "free text that the sort form asks for"},
}

// TypeSetup returns the prompt for setting up a document type. The ids of the
// project's current types are appended so the agent avoids duplicates.
func TypeSetup(existingTypeIDs []string) string {
	s := typeSetup
	s = strings.ReplaceAll(s, "{{enums}}", enumsText())
	s = strings.ReplaceAll(s, "{{weights}}", fmt.Sprintf("`weak` = %v, `normal` = %v, `strong` = %v. A number from 0 to 1 is accepted; the words `weak`, `normal`, `strong` too.",
		analysis.WeightWeak, analysis.WeightNormal, analysis.WeightStrong))
	var pb strings.Builder
	pb.WriteString("Placeholders are written in braces:\n\n| Placeholder | Value |\n|---|---|\n")
	for _, p := range Placeholders {
		fmt.Fprintf(&pb, "| `%s` | %s |\n", p[0], p[1])
	}
	s = strings.ReplaceAll(s, "{{placeholders}}", strings.TrimRight(pb.String(), "\n"))

	s = strings.TrimRight(s, "\n") + "\n\n## Existing types\n\n"
	if len(existingTypeIDs) == 0 {
		return s + "The project has no types yet.\n"
	}
	return s + "The project already has these type ids; your `id` must differ from all of them:\n\n- `" + strings.Join(existingTypeIDs, "`\n- `") + "`\n"
}

func enumsText() string {
	row := func(field string, vals []string) string {
		return fmt.Sprintf("- `%s`: `%s`\n", field, strings.Join(vals, "`, `"))
	}
	return strings.TrimRight(
		row("kind", analysis.RuleKinds)+
			row("mode", analysis.RuleModes)+
			row("where", analysis.RuleWheres)+
			row("date.source", analysis.DateSources), "\n")
}
