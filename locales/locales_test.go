package locales

import (
	"encoding/json"
	"testing"
)

func keys(t *testing.T, name string) map[string]bool {
	t.Helper()
	b, err := FS.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

func TestCatalogsHaveSameKeys(t *testing.T) {
	en, de := keys(t, "en.json"), keys(t, "de.json")
	for k := range en {
		if !de[k] {
			t.Errorf("de.json lacks %q", k)
		}
	}
	for k := range de {
		if !en[k] {
			t.Errorf("en.json lacks %q", k)
		}
	}
}
