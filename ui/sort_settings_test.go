package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/widgets"
	"github.com/72nd/sanity/locales"
)

func TestVimSettingsAndPasteCommand(t *testing.T) {
	i18n.MustAddFiles(locales.FS, locales.Files...)
	a := test.NewApp()
	defer a.Quit()
	sh := shell.New(a, shell.Config{TitleKey: "app.title", Width: 800, Height: 600,
		Pages:    []shell.Page{{ID: "x", TitleKey: "app.title", Build: func(*shell.Shell) fyne.CanvasObject { return widgets.NewLabel("x", widgets.LabelBody) }}},
		Settings: []shell.SettingsSection{VimSettings()}})
	if !VimEnabled(sh) {
		t.Fatal("default must be on")
	}
	obj := VimSettings().Build(sh)
	var sw *widgets.Switch
	for _, o := range test.LaidOutObjects(obj) {
		if s, ok := o.(*widgets.Switch); ok {
			sw = s
		}
	}
	if sw == nil || !sw.On() {
		t.Fatal("switch missing or off")
	}
	sw.Tapped(nil)
	if VimEnabled(sh) || sh.Prefs.String(prefVim, "") != "off" {
		t.Error("switch off must store off")
	}
	called := false
	act := NewSortActions()
	act.PasteFiles = func() { called = true }
	for _, c := range Commands(act) {
		if c.ID == "sort.paste" {
			c.Run()
		}
	}
	if !called {
		t.Error("sort.paste must call PasteFiles")
	}
}

func TestVimSettingsHintWrapsWhenNarrow(t *testing.T) {
	i18n.MustAddFiles(locales.FS, locales.Files...)
	a := test.NewApp()
	defer a.Quit()
	sh := shell.New(a, shell.Config{TitleKey: "app.title", Width: 800, Height: 600,
		Pages: []shell.Page{{ID: "x", TitleKey: "app.title", Build: func(*shell.Shell) fyne.CanvasObject { return widgets.NewLabel("x", widgets.LabelBody) }}}})
	row := VimSettings().Build(sh)
	w := test.NewTempWindow(t, row)
	w.Resize(fyne.NewSize(900, 300))
	wide := row.MinSize().Height
	w.Resize(fyne.NewSize(260, 300))
	if narrow := row.MinSize().Height; narrow <= wide {
		t.Errorf("hint must wrap on a narrow page: height %v, wide %v", narrow, wide)
	}
}
