package main

import (
	"testing"

	"github.com/72nd/fine/archcheck"
)

// TestArchitecture enforces the dependency rules of AGENTS.md. Keep the
// directory lists in sync when you add packages.
func TestArchitecture(t *testing.T) {
	archcheck.Run(t, archcheck.Rules{
		Root:          ".",
		NoUIDirs:      []string{"domain", "store"},
		PresenterDirs: []string{"ui/presenter"},
		AppDirs:       []string{"."},
		ThemeDir:      "theme", // this repo has none, so no color literals are allowed anywhere
	})
}
