package presenter

import "strings"

// FieldLabels turns placeholder names from domain.Template.Missing into the
// localized form labels, each once, in order.
func FieldLabels(t Translator, names []string) []string {
	var out []string
	seen := make(map[string]bool, len(names))
	add := func(label string) {
		if seen[label] {
			return
		}
		seen[label] = true
		out = append(out, label)
	}
	for _, n := range names {
		switch {
		case strings.HasPrefix(n, "date:"):
			add(t("sort.date"))
		case strings.HasPrefix(n, "text:"):
			add(strings.TrimPrefix(n, "text:"))
		case n == "invoice":
			add(t("sort.invoice"))
		case n == "amount":
			add(t("sort.amount"))
		case n == "type":
			add(t("sort.type"))
		case n == "original":
			add(t("sort.original"))
		default:
			add(n)
		}
	}
	return out
}

// MissingLabels returns the localized labels of the fields that still need a
// value (see Missing and FieldLabels).
func (p *Sort) MissingLabels() []string { return FieldLabels(p.T, p.Missing()) }
