package ui

import (
	"time"

	"fyne.io/fyne/v2"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/sanity/ui/presenter"
)

// prefVim is the preference that switches the Vim layer off ("off").
const prefVim = "sanity.vim"

// VimEnabled reports whether the Vim layer is on (the default).
func VimEnabled(sh *shell.Shell) bool { return sh.Prefs.String(prefVim, "on") != "off" }

// InstallVim sets up the modal layer of the sort page. The shell clears the
// canvas key hooks on every page change, so call it from Page.OnShow.
//
// Insert mode is the default: a field has the focus and letters are text.
// Esc in a field takes the focus away (normal mode); then the canvas hooks
// receive the runes and the parser turns them into commands. i, t and d put
// the focus back into the form.
func InstallVim(sh *shell.Shell, act *SortActions, keys *presenter.Keys) {
	setStatus := func(normal bool) {
		if !VimEnabled(sh) {
			sh.Status.SetLeft("")
			return
		}
		if normal {
			sh.Status.SetLeft(i18n.T("vim.mode.normal"))
		} else {
			sh.Status.SetLeft(i18n.T("vim.mode.insert"))
		}
	}
	act.SetMode = func(normal bool) {
		keys.Reset()
		setStatus(normal)
	}
	setStatus(false)
	if !VimEnabled(sh) {
		return
	}

	run := func(cmd string) {
		switch cmd {
		case "next", "row.down":
			act.Skip()
		case "prev", "row.up":
			act.SkipBack()
		case "first":
			act.First()
		case "last":
			act.Last()
		case "page.next":
			act.ScrollDown()
		case "page.prev":
			act.ScrollUp()
		case "zoom.in":
			act.ZoomIn()
		case "zoom.out":
			act.ZoomOut()
		case "zoom.fit":
			act.ZoomFit()
		case "focus.type":
			act.Focus("type")
		case "focus.date":
			act.Focus("date")
		case "focus.first":
			act.Focus("")
		case "reveal":
			act.Reveal()
		case "undo":
			act.Undo()
		case "copy.rel":
			act.CopyRel()
		case "copy.abs":
			act.CopyAbs()
		case "remove":
			act.Remove()
		case "add":
			act.AddFiles()
		case "search":
			act.Search()
		case "palette":
			sh.Registry.Run("palette.open")
		}
	}
	var timer *time.Timer
	sh.SetOnTypedRune(func(r rune) {
		cmd, _ := keys.Feed(presenter.ContextSort, r)
		if cmd != "" {
			run(cmd)
		}
		for _, res := range keys.Drain() {
			if res.Cmd != "" {
				run(res.Cmd)
			}
		}
		// a lone d waits for a second key; without one it fires after the timeout
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(presenter.SequenceTimeout+20*time.Millisecond, func() {
			fyne.Do(func() {
				if cmd, ok := keys.Expire(); ok {
					run(cmd)
				}
			})
		})
	})
}
