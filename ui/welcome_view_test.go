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

// The project menu lists the open project first, opens a recent project and
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
	sel := ProjectMenu(sh, nav, "/p/one", "One")
	want := []string{"One", "Two", "Open project…", "Overview"}
	if got := sel.Options(); len(got) != len(want) || got[1] != "Two" || got[3] != "Overview" {
		t.Fatalf("options = %q", got)
	}
	sel.SetSelected("Two")
	sel.OnPicked("Two")
	if switched != two || sel.SelectedIndex() != 0 {
		t.Errorf("switched=%q index=%d", switched, sel.SelectedIndex())
	}
	sel.OnPicked("Overview")
	if overview != 1 || sel.SelectedIndex() != 0 {
		t.Errorf("overview=%d index=%d", overview, sel.SelectedIndex())
	}
}
