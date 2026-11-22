package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/store"
)

func TestCorrectionsPrintsStats(t *testing.T) {
	root := setup(t) // project with the type "Other" (id other)
	st, _ := store.OpenJSON(root)
	if err := st.SaveType(t.Context(), domain.DocumentType{ID: "tk", Name: "Telekom", PathTemplate: "t/{original}.pdf"}); err != nil {
		t.Fatal(err)
	}
	l := inbox.NewCorrectionLog(root)
	for _, c := range []inbox.Correction{
		{SHA256: "1", TypeID: "tk", Suggested: inbox.SuggestedType{TypeID: "tk", Confident: true}, DateSuggested: "2025-01-02", DateChosen: "2025-01-02"},
		{SHA256: "2", TypeID: "tk", Suggested: inbox.SuggestedType{TypeID: "other", Confident: true}, DateSuggested: "2025-01-02", DateChosen: "2025-01-05"},
		{SHA256: "3", TypeID: "other"},
	} {
		if err := l.Append(c); err != nil {
			t.Fatal(err)
		}
	}
	var out, errb bytes.Buffer
	if code := runCorrections([]string{"--project", root}, &out, &errb); code != 0 {
		t.Fatalf("code %d: %s", code, errb.String())
	}
	got := out.String()
	for _, want := range []string{"Filings", "3", "1/3 (33%)", "1/2 (50%)", "Telekom", "Other"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
}

func TestCorrectionsEmptyAndNoProject(t *testing.T) {
	root := setup(t)
	var out, errb bytes.Buffer
	if code := runCorrections([]string{"--project", root}, &out, &errb); code != 0 || !strings.Contains(out.String(), "No filings") {
		t.Errorf("empty log: %d %q %q", code, out.String(), errb.String())
	}
	errb.Reset()
	if code := runCorrections([]string{"--project", t.TempDir()}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "no project") {
		t.Errorf("no project: %d %q", code, errb.String())
	}
	if code := runCorrections([]string{"extra"}, &out, &errb); code != 2 {
		t.Errorf("usage code %d", code)
	}
}
