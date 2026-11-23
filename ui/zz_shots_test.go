package ui

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/72nd/fine/shell"

	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/locales"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
	"github.com/72nd/sanity/ui/presenter"
)

func shotTo(t *testing.T, dir string, sh interface {
	Resize(fyne.Size)
	Canvas() fyne.Canvas
}, name string, w, h float32) {
	sh.Resize(fyne.NewSize(w, h))
	settle()
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_ = png.Encode(f, sh.Canvas().Capture())
}

// TestSortShots writes screenshots of the sort page to $SANITY_SHOTS (dev aid).
func TestSortShots(t *testing.T) {
	dir := os.Getenv("SANITY_SHOTS")
	if dir == "" {
		t.Skip("SANITY_SHOTS not set")
	}
	sh, sp, _ := newShell(t, 2)
	sh.Show("sort")
	settle()
	sh.Toasts.Hide()
	shot := func(name string, w, h float32) { shotTo(t, dir, sh.Win, name, w, h) }
	shot("after_empty_1200", 1200, 760)
	if ts := sp.Types(); len(ts) > 0 {
		lastAct.fields.picker.Preselect(ts[0].ID)
		sp.SetType(ts[0].ID)
		lastAct.fields.picker.OnChanged(ts[0].ID)
	}
	shot("after_typed_1200", 1200, 760)
	shot("after_typed_900", 900, 700)
	sp.SetDate(time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC))
	sp.SetInvoice("R-2026-000123456789")
	lastAct.fields.panel.refreshSingle()
	shot("after_ready_1200", 1200, 760)
	shot("after_ready_900", 900, 700)
	lastAct.fields.queue.SelectKeys([]string{sp.Items()[0].ID, sp.Items()[1].ID})
	lastAct.fields.queue.OnSelectionChanged()
	shot("after_batch_1200", 1200, 760)
	shot("after_batch_900", 900, 700)
	lastAct.fields.panel.batchPicker.OnChanged(sp.Types()[0].ID)
	shot("after_batch_typed_1200", 1200, 760)
}

// TestSortShotsDuplicate: a duplicate with a long path, German UI.
func TestSortShotsDuplicate(t *testing.T) {
	dir := os.Getenv("SANITY_SHOTS")
	if dir == "" {
		t.Skip("SANITY_SHOTS not set")
	}
	st := store.NewFake()
	sh, sp, _ := newShellWith(t, 1, st)
	sum, _ := inbox.FileHash(sp.Items()[0].Path)
	dup := domain.LibraryEntry{ID: "dup-1", TypeID: "x", Path: "receipts/2025/telecom/2025-03-12_telekom_R-88123456789012_old-copy.pdf", SHA256: sum, Date: time.Now(), FiledAt: time.Now()}
	_ = st.AddEntry(context.Background(), dup)
	sp.SetAnalyzer(&inbox.Analyzer{PDF: pdf.NewFake("Some text"), Store: st})
	done := make(chan string, 1)
	sp.Analyse(context.Background(), func(id string) { done <- id })
	<-done
	sh.Show("sort")
	settle()
	sh.Toasts.Hide()
	shot := func(name string, w, h float32) { shotTo(t, dir, sh.Win, name, w, h) }
	shot("after_dup_1200x760", 1200, 760)
	i18n.SetLanguage("de")
	defer i18n.SetLanguage("en")
	sh.Show("library")
	sh.Show("sort")
	settle()
	shot("after_dup_de_900", 900, 700)
}

// TestProjectShots writes screenshots of the project page to $SANITY_SHOTS.
func TestProjectShots(t *testing.T) {
	dir := os.Getenv("SANITY_SHOTS")
	if dir == "" {
		t.Skip("SANITY_SHOTS not set")
	}
	app := test.NewTempApp(t)
	i18n.MustAddFiles(locales.FS, locales.Files...)
	i18n.SetLanguage("en")
	pp := presenter.NewProject(store.NewFake(), i18n.T)
	rec := presenter.NewRecents(testPrefs{})
	rec.Exists = func(string) bool { return true }
	rec.Add("/p/one", "One")
	nav := &ProjectNav{Recents: rec, Switch: func(string) {}}
	sh := shell.New(app, shell.Config{TitleKey: "app.title", Width: 1200, Height: 760,
		Pages: []shell.Page{ProjectPageEntry(pp, nav)}})
	sh.Show("project")
	settle()
	for _, w := range []float32{1400, 1000, 700} {
		shotTo(t, dir, sh.Win, "project_"+strconv.Itoa(int(w)), w, 800)
		shotTo(t, dir, sh.Win, "project_"+strconv.Itoa(int(w))+"b", w, 800)
	}
}
