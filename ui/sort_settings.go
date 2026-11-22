package ui

import (
	"fyne.io/fyne/v2"

	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/fine/widgets"
)

// VimSettings is the settings page card with the switch for the Vim layer.
// The preference is read by InstallVim each time the sort page is shown, so
// a change applies on the next show without a restart.
func VimSettings() shell.SettingsSection {
	return shell.SettingsSection{TitleKey: "settings.vim.title", Build: func(sh *shell.Shell) fyne.CanvasObject {
		sw := widgets.NewSwitch(i18n.T("settings.vim.label"), func(on bool) {
			v := "off"
			if on {
				v = "on"
			}
			sh.Prefs.SetString(prefVim, v)
		})
		sw.SetOn(VimEnabled(sh))
		row := widgets.NewFormRow("", sw)
		row.SetHint(i18n.T("settings.vim.desc"))
		return row
	}}
}
