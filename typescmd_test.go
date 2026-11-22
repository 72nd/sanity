package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/store"
)

const goodSnippet = `{"type":{"id":"hetzner","name":"Hetzner","pathTemplate":"r/{date:2006}/{original}.pdf",
"rules":[{"kind":"domain","value":"hetzner.com","mode":"should","weight":"strong","where":"header"}]}}`

func setup(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	st, err := store.OpenJSON(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveType(t.Context(), domain.DocumentType{ID: "other", Name: "Other", PathTemplate: "o/{original}.pdf"}); err != nil {
		t.Fatal(err)
	}
	return root
}

func run(t *testing.T, args ...string) (int, analysis.Report, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runTypesCheck(args, &out, &errb)
	var rep analysis.Report
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatalf("stdout is not a report: %v\n%s", err, out.String())
		}
	}
	return code, rep, errb.String()
}

func TestTypesCheckOK(t *testing.T) {
	root := setup(t)
	extractSample = func(string) (string, error) { return "Hetzner Online\nwww.hetzner.com\nRechnung", nil }
	snip := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(snip, []byte(goodSnippet), 0o644)
	// flags after positionals work
	code, rep, errs := run(t, snip, "sample.pdf", "--project", root)
	if code != 0 {
		t.Fatalf("code %d %s %+v", code, errs, rep.Findings)
	}
	if len(rep.Samples) != 1 || !rep.Samples[0].Confident || rep.Samples[0].Name != "sample.pdf" {
		t.Fatalf("%+v", rep.Samples)
	}
}

func TestTypesCheckErrorsAndCollision(t *testing.T) {
	root := setup(t)
	extractSample = func(string) (string, error) { return "x", nil }
	snip := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(snip, []byte(`{"id":"other","name":"N","pathTemplate":"../x.pdf","rules":[]}`), 0o644)
	code, rep, _ := run(t, "--project", root, snip)
	if code != 1 || !rep.HasErrors() {
		t.Fatalf("code %d %+v", code, rep.Findings)
	}
	var coll bool
	for _, f := range rep.Findings {
		coll = coll || (f.Code == "id.collision" && f.Argument == "other-2")
	}
	if !coll {
		t.Fatalf("%+v", rep.Findings)
	}
}

func TestTypesCheckLibraryFromCache(t *testing.T) {
	root := setup(t)
	st, _ := store.OpenJSON(root)
	st.AddEntry(t.Context(), domain.LibraryEntry{ID: "e1", TypeID: "other", Path: "o/a.pdf", SHA256: "sha1"})
	c := inbox.OpenCache(inbox.CachePath(root))
	c.Put("sha1", inbox.CacheEntry{Text: "mail from hetzner.com"})
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	snip := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(snip, []byte(goodSnippet), 0o644)
	code, rep, errs := run(t, snip, "--project", root)
	if code != 0 || len(rep.Library) != 1 || rep.Library[0].EntryID != "e1" {
		t.Fatalf("code %d %s %+v", code, errs, rep)
	}
}

func TestTypesCheckUsageAndFailures(t *testing.T) {
	if code, _, errs := run(t); code != 2 || errs == "" {
		t.Fatalf("no args: %d %q", code, errs)
	}
	if code, _, _ := run(t, "--bogus"); code != 2 {
		t.Fatal("bad flag")
	}
	root := setup(t)
	if code, _, errs := run(t, "missing.json", "--project", root); code != 1 || errs == "" {
		t.Fatalf("missing snippet: %d %q", code, errs)
	}
	snip := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(snip, []byte("not json"), 0o644)
	if code, _, _ := run(t, snip, "--project", root); code != 1 {
		t.Fatal("bad snippet")
	}
	if code, _, errs := run(t, snip, "--project", t.TempDir()); code != 1 || errs == "" {
		t.Fatal("no project")
	}
}
