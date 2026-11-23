package ui

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/sanity/locales"
	"github.com/72nd/sanity/ui/presenter"
)

// The project menu lists the open project first (checked), opens a recent project and
// goes to the overview; afterwards it shows the open project again.
func TestProjectMenuPicks(t *testing.T) {
	i18n.MustAddFiles(locales.FS, locales.Files...)
	i18n.SetLanguage("en")
	rec := presenter.NewRecents(testPrefs{})
	rec.Exists = func(string) bool { return true }
	two := t.TempDir()
	if err := os.MkdirAll(filepath.Join(two, "office"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(presenter.ProjectFile(two), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec.Add(two, "Two")
	rec.Add("/p/one", "One")
	var switched string
	overview := 0
	nav := &ProjectNav{Recents: rec, Switch: func(r string) { switched = r }, Overview: func() { overview++ }}
	sh := shell.New(test.NewTempApp(t), shell.Config{TitleKey: "app.title", Width: 800, Height: 600,
		Pages: []shell.Page{{ID: "welcome", TitleKey: "welcome.nav",
			Build: func(sh *shell.Shell) fyne.CanvasObject { return WelcomePage(sh, nil, nav) }}}})
	if ProjectMenu(sh, nav, "/p/one", "One") == nil {
		t.Fatal("no menu button")
	}
	items := projectMenuItems(sh, nav, "/p/one", "One")
	var labels []string
	for _, it := range items {
		labels = append(labels, it.Label)
		if !it.Separator && it.Icon == "" {
			t.Errorf("%q has no icon", it.Label)
		}
	}
	want := []string{"One", "Two", "", "Open project…", "Overview"}
	if len(labels) != len(want) {
		t.Fatalf("items = %q, want %q", labels, want)
	}
	if !items[0].Disabled || !items[2].Separator || items[4].Label != "Overview" {
		t.Errorf("items = %+v", items)
	}
	items[1].Action()
	if switched != two {
		t.Errorf("switched = %q", switched)
	}
	items[4].Action()
	if overview != 1 {
		t.Errorf("overview = %d", overview)
	}
}
