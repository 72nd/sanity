package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

// iconPNG is the app icon built by `task icon`. It is also the icon of the
// packaged app (FyneApp.toml), so a run from source shows the same Dock icon.
//
//go:embed icon/icon.png
var iconPNG []byte

func appIcon() fyne.Resource { return fyne.NewStaticResource("icon.png", iconPNG) }
