package ui

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/fine/widgets/grid"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/locales"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
	"github.com/72nd/sanity/ui/presenter"
)

// rootStore is the fake store with a real project root, so library paths
// point into a temp folder.
type rootStore struct {
	*store.Fake
	root string
}

func (r rootStore) Project(ctx context.Context) (domain.Project, error) {
	p, err := r.Fake.Project(ctx)
	p.Root = r.root
	return p, err
}

type libEnv struct {
	sh   *shell.Shell
	lib  *presenter.Library
	act  *SortActions
	q    *inbox.Queue
	root string
	clip *string
}

func newLibEnv(t *testing.T) *libEnv {
	t.Helper()
	app := test.NewTempApp(t)
	i18n.MustAddFiles(locales.FS, locales.Files...)
	i18n.SetLanguage("en")
	root := t.TempDir()
	st := rootStore{Fake: store.NewFake(), root: root}
	q := inbox.NewQueue(nil)
	lib := presenter.NewLibrary(st, i18n.T)
	if err := lib.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	sp := presenter.NewSort(st, q, inbox.NewFiler(root, st), pdf.NewFake(), i18n.T)
	act := NewSortActions()
	sh := shell.New(app, shell.Config{TitleKey: "app.title", Pages: Pages(lib, sp, q, act, presenter.NewKeys(nil), presenter.NewTypes(st, i18n.T)), Width: 1200, Height: 760})
	for _, c := range Commands(act) {
		if err := sh.Registry.Register(c); err != nil {
			t.Fatal(err)
		}
	}
	clip := new(string)
	old := setClipboard
	setClipboard = func(_ *shell.Shell, text string) { *clip = text }
	t.Cleanup(func() { setClipboard = old })
	return &libEnv{sh: sh, lib: lib, act: act, q: q, root: root, clip: clip}
}

func (e *libEnv) typeRunes(s string) {
	for _, r := range s {
		e.act.Library.Rune(r)
	}
}

func lastStatus(sh *shell.Shell) string {
	r := sh.Status.Right()
	if len(r) == 0 {
		return ""
	}
	return r[len(r)-1]
}

// The quick filter text of the presenter filters the grid, and v with j
// selects a range whose amounts add up in the status bar.
func TestLibraryFilterAndSelectionTotal(t *testing.T) {
	e := newLibEnv(t)
	e.lib.Query = "telekom"
	e.sh.Show("library")
	if got := e.sh.Status.Right()[0]; !strings.HasPrefix(got, "6 of ") {
		t.Fatalf("rows status = %q", got)
	}
	e.typeRunes("jvjj") // rows 0..2, 49.95 EUR each in the newest order: 54.95 x3
	last := lastStatus(e.sh)
	want := "3 selected, total " + e.lib.Totals(e.lib.Visible("telekom")[:3])[0].String()
	if !strings.Contains(last, want) {
		t.Errorf("status = %q (all: %q)", last, e.sh.Status.Right())
	}
	e.typeRunes("k") // visual range shrinks
	if got := lastStatus(e.sh); !strings.Contains(got, "2 selected, total "+e.lib.Totals(e.lib.Visible("telekom")[:2])[0].String()) {
		t.Errorf("after k: %q", got)
	}
}

func TestLibraryCopyPathsToClipboard(t *testing.T) {
	e := newLibEnv(t)
	e.sh.Show("library")
	e.typeRunes("jvj") // two rows
	e.typeRunes("yy")
	lines := strings.Split(*e.clip, "\n")
	if len(lines) != 2 || strings.HasPrefix(lines[0], "/") || !strings.HasSuffix(lines[0], ".pdf") {
		t.Fatalf("relative copy = %q", *e.clip)
	}
	// Cmd+Alt+C is the sort shortcut: on the library page it copies absolute paths.
	if !e.sh.Registry.Run("sort.copyabs") {
		t.Fatal("command missing")
	}
	abs := strings.Split(*e.clip, "\n")
	if len(abs) != 2 || abs[0] != filepath.Join(e.root, strings.TrimPrefix(lines[0], "../")) {
		t.Errorf("absolute copy = %q", *e.clip)
	}
}

// The shared shortcut acts on the library only while the library is shown.
func TestSharedShortcutsArePageAware(t *testing.T) {
	e := newLibEnv(t)
	e.sh.Show("library")
	e.sh.Registry.Run("sort.search")
	if _, ok := e.sh.Win.Canvas().Focused().(*widgets.Entry); !ok {
		t.Errorf("Cmd+F on the library must focus the filter, focus is %T", e.sh.Win.Canvas().Focused())
	}
	e.sh.Show("sort")
	called := false
	e.act.Library.CopyRel = func() { called = true }
	e.sh.Registry.Run("sort.copyrel")
	if called {
		t.Error("Cmd+Shift+C on the sort page reached the library")
	}
	e.sh.Registry.Run("library.resort") // no-op away from the library
}

// r sends the filed document back: its absolute path arrives in the sort queue.
func TestLibraryResortImportsIntoQueue(t *testing.T) {
	e := newLibEnv(t)
	row := e.lib.Visible("")[0]
	path := e.lib.AbsPath(row)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.sh.Show("library")
	e.typeRunes("jr")
	settle()
	items := e.q.Items()
	if len(items) != 1 || items[0].Path != path {
		t.Fatalf("queue = %+v, want %s", items, path)
	}
	if e.sh.Current() != "sort" {
		t.Errorf("page = %q", e.sh.Current())
	}
}

func TestLibraryInboxHintAndGoTo(t *testing.T) {
	e := newLibEnv(t)
	dir := filepath.Join(e.root, "inbox")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "a.pdf"), []byte("%PDF"), 0o644)
	e.q.Add(dir)
	if h, ok := e.lib.InboxHint(); !ok || !strings.Contains(h, "1 ") {
		t.Errorf("hint = %q %v", h, ok)
	}
	e.sh.Show("library")
}

func TestLibraryNavigation(t *testing.T) {
	e := newLibEnv(t)
	e.sh.Show("library")
	e.typeRunes("G")
	if got := lastStatus(e.sh); got == "" {
		t.Fatal("no status")
	}
	rows := e.lib.Visible("")
	e.typeRunes("yy")
	if want := domain.OfficeRel(rows[len(rows)-1].Entry.Path); *e.clip != want {
		t.Errorf("G then yy copied %q, want the last row %q", *e.clip, want)
	}
	e.typeRunes("ggyy")
	if *e.clip != domain.OfficeRel(rows[0].Entry.Path) {
		t.Errorf("gg then yy copied %q", *e.clip)
	}
	e.typeRunes("Vyy")
	if n := len(strings.Split(*e.clip, "\n")); n != len(rows) {
		t.Errorf("V selected %d rows of %d", n, len(rows))
	}
}

// libGrid is the library grid of the page that was just built.
func (e *libEnv) libGrid(t *testing.T) *grid.Grid[presenter.Row] {
	t.Helper()
	g, ok := e.act.Library.focus.(*grid.Grid[presenter.Row])
	if !ok {
		t.Fatalf("focus is %T", e.act.Library.focus)
	}
	return g
}

// Value filters count into the status bar: "n of m" shows the shown items
// against the whole library, also while grouped.
func TestLibraryStatusCountsGridFilter(t *testing.T) {
	e := newLibEnv(t)
	e.sh.Show("library")
	g := e.libGrid(t)
	total := e.lib.Total()
	if got := e.sh.Status.Right()[0]; !strings.HasPrefix(got, strconv.Itoa(total)+" of "+strconv.Itoa(total)) {
		t.Fatalf("status = %q", got)
	}
	g.SetValueFilter(1, &grid.ValueFilter{Col: "type", Only: true, Values: []string{e.lib.Visible("")[0].TypeName}})
	shown := g.ItemCount()
	if shown == 0 || shown >= total {
		t.Fatalf("filter shows %d of %d", shown, total)
	}
	if got := e.sh.Status.Right()[0]; !strings.HasPrefix(got, strconv.Itoa(shown)+" of "+strconv.Itoa(total)) {
		t.Errorf("status after filter = %q, want %d of %d", got, shown, total)
	}
	g.SetGroupBy([]grid.GroupBy{{Col: "date", Key: presenter.GroupYear}})
	if g.Len() <= g.ItemCount() {
		t.Fatalf("grouping adds header rows: len %d items %d", g.Len(), g.ItemCount())
	}
	if got := e.sh.Status.Right()[0]; !strings.HasPrefix(got, strconv.Itoa(shown)+" of ") {
		t.Errorf("status when grouped = %q", got)
	}
	// Vim movement across group headers keeps working and never copies a header.
	e.typeRunes("gg")
	e.typeRunes("yy")
	e.typeRunes("Vyy")
	if n := len(strings.Split(*e.clip, "\n")); n != shown {
		t.Errorf("V selected %d rows, want %d", n, shown)
	}
}

// The grid state is saved in the preferences and restored when the page is
// rebuilt, before the first rows are shown.
func TestLibraryGridStatePersists(t *testing.T) {
	e := newLibEnv(t)
	e.sh.Show("library")
	g := e.libGrid(t)
	g.SetSortKeys([]grid.SortKey{{Col: "amount", Desc: true}})
	g.SetGroupBy([]grid.GroupBy{{Col: "date", Key: presenter.GroupMonth}})
	g.SetConditions(grid.Conditions{Items: []grid.Condition{{Col: "amount", Op: grid.OpGt, Values: []string{"10"}}}})
	if !e.sh.Prefs.LoadJSON(libraryGridKey, new(grid.State)) {
		t.Fatal("state was not saved")
	}
	e.sh.Show("types")
	e.sh.Show("library")
	g2 := e.libGrid(t)
	if g2 == g {
		t.Fatal("page was not rebuilt")
	}
	if keys := g2.SortKeys(); len(keys) != 1 || keys[0].Col != "amount" || !keys[0].Desc {
		t.Errorf("sort = %+v", keys)
	}
	if gb := g2.GroupBy(); len(gb) != 1 || gb[0].Col != "date" || gb[0].Key != presenter.GroupMonth {
		t.Errorf("group = %+v", gb)
	}
	if c := g2.Conditions(); len(c.Items) != 1 || c.Items[0].Op != grid.OpGt {
		t.Errorf("conditions = %+v", c)
	}
	if g2.ItemCount() != g.ItemCount() {
		t.Errorf("shown %d, before %d", g2.ItemCount(), g.ItemCount())
	}
	e.sh.Registry.Run("library.clearfilters")
	if g2.FilterActive() {
		t.Error("clear all filters left a filter")
	}
	e.sh.Registry.Run("library.ungroup")
	if len(g2.GroupBy()) != 0 {
		t.Error("ungroup left a grouping")
	}
}
