package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/store"
)

// runCorrections implements `sanity corrections [--project <root>]`: it reads
// the correction log of the project and prints how often the suggestions were
// right. Exit code: 0 ok, 1 failure, 2 usage.
func runCorrections(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("corrections", flag.ContinueOnError)
	fs.SetOutput(stderr)
	project := fs.String("project", "", "project root (default: the current directory)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: sanity corrections [--project <root>]")
		return 2
	}
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
	cs, err := inbox.ReadAll(root)
	if err != nil {
		fmt.Fprintln(stderr, "sanity:", err)
		return 1
	}
	names := map[string]string{} // type id -> name, when the project can be read
	if st, err := store.OpenJSON(root); err == nil {
		if ts, err := st.Types(context.Background()); err == nil {
			for _, t := range ts {
				names[t.ID] = t.Name
			}
		}
	}
	r := inbox.Stats(cs)
	if r.Total == 0 {
		fmt.Fprintln(stdout, "No filings in the correction log yet.")
		return 0
	}
	pct := func(hits, total int) string {
		if total == 0 {
			return "-"
		}
		return fmt.Sprintf("%d/%d (%.0f%%)", hits, total, 100*inbox.Rate(hits, total))
	}
	w := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "Filings\t%d\n", r.Total)
	fmt.Fprintf(w, "Type hit rate\t%s\n", pct(r.TypeHits, r.Total))
	fmt.Fprintf(w, "Confident precision\t%s\n", pct(r.ConfidentHits, r.ConfidentTotal))
	fmt.Fprintf(w, "Date hit rate\t%s\n", pct(r.DateHits, r.DateTotal))
	w.Flush()
	ids := make([]string, 0, len(r.ByType))
	for id := range r.ByType {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fmt.Fprintln(stdout)
	w = tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "Type\tFiled\tSuggested right")
	for _, id := range ids {
		ts := r.ByType[id]
		name := names[id]
		if name == "" {
			name = id
		}
		fmt.Fprintf(w, "%s\t%d\t%s\n", name, ts.Total, pct(ts.Hits, ts.Total))
	}
	w.Flush()
	return 0
}
