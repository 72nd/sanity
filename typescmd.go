package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

// extractSample returns the normalized text of a PDF. Tests replace it.
var extractSample = func(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	x, err := inbox.ExtractText(ctx, pdf.NewEngine(), path)
	return x.Text, err
}

const typesCheckUsage = "usage: sanity types check <snippet.json> <sample.pdf>... [--project <root>]"

// runTypesCheck implements `sanity types check`: it parses the snippet, dry-runs
// it against the samples and the project's library and prints the report as
// JSON. Exit code: 0 no errors, 1 errors (or a failure), 2 usage.
func runTypesCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("types check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	project := fs.String("project", "", "project root (default: the current directory)")
	// Flags may follow the positional arguments, so parse in rounds.
	var pos []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		pos = append(pos, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	if len(pos) < 1 {
		fmt.Fprintln(stderr, typesCheckUsage)
		return 2
	}
	snippetPath, samplePaths := pos[0], pos[1:]

	root := *project
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(stderr, "sanity:", err)
			return 1
		}
		root = wd
	}
	if _, err := os.Stat(filepath.Join(root, "office", "sanity.json")); err != nil {
		fmt.Fprintf(stderr, "sanity: no project at %s (office/sanity.json is missing); use --project\n", root)
		return 1
	}

	data, err := os.ReadFile(snippetPath)
	if err != nil {
		fmt.Fprintln(stderr, "sanity:", err)
		return 1
	}
	typ, _, err := analysis.ParseSnippet(data)
	if err != nil {
		fmt.Fprintln(stderr, "sanity: snippet:", err)
		return 1
	}

	st, err := store.OpenJSON(root)
	if err != nil {
		fmt.Fprintln(stderr, "sanity:", err)
		return 1
	}
	ctx := context.Background()
	prj, err := st.Project(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "sanity:", err)
		return 1
	}
	cache := inbox.OpenCache(inbox.CachePath(prj.Root))
	// The learned index comes from the cached texts of the filed documents.
	idx := analysis.NewIndex(prj.IgnoreTokens)
	for _, e := range prj.Library {
		if ce, ok := cache.Get(e.SHA256); ok && ce.Text != "" {
			idx.Add(e.TypeID, ce.Text)
		}
	}

	in := analysis.CheckInput{
		Type:         typ,
		Existing:     prj.Types,
		Learned:      idx,
		Library:      prj.Library,
		IgnoreTokens: prj.IgnoreTokens,
		Text: func(sha string) (string, bool) {
			ce, ok := cache.Get(sha)
			return ce.Text, ok
		},
	}
	for _, p := range samplePaths {
		text, err := extractSample(p)
		if err != nil {
			fmt.Fprintf(stderr, "sanity: sample %s: %v\n", p, err)
			return 1
		}
		in.Samples = append(in.Samples, analysis.Sample{Name: filepath.Base(p), FileName: filepath.Base(p), Text: text})
	}

	rep := analysis.CheckType(in)
	out, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, "sanity:", err)
		return 1
	}
	fmt.Fprintln(stdout, string(out))
	if rep.HasErrors() {
		return 1
	}
	return 0
}
