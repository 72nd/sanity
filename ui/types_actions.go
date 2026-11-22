package ui

import (
	"github.com/72nd/fine/shell"
	"github.com/72nd/fine/shell/i18n"
)

// TypesActions are the Types page's actions for the command palette. The page
// fills Import and CopyPrompt when it is built; until then they do nothing.
type TypesActions struct {
	Import     func() // open the "Import type…" dialog
	CopyPrompt func() // copy the LLM prompt
	show       func() // brings the Types page to the front
}

// NewTypesActions returns actions that do nothing until the page exists.
func NewTypesActions() *TypesActions {
	return &TypesActions{Import: nop, CopyPrompt: nop, show: nop}
}

var sharedTypesActions = NewTypesActions()

// SharedTypesActions returns the actions the Types page binds to. Pages builds
// the page without parameters for them, so they live here.
func SharedTypesActions() *TypesActions { return sharedTypesActions }

// Attach makes the commands show the Types page first, so they work from every
// page. It returns a.
func (a *TypesActions) Attach(sh *shell.Shell) *TypesActions {
	a.show = func() { sh.Show("types") }
	return a
}

// TypesCommands returns the commands "types.import" and "types.copyprompt".
// They have no shortcut; the palette lists them.
func TypesCommands(a *TypesActions) []shell.Command {
	t := func(id string) func() string { return func() string { return i18n.T(id) } }
	return []shell.Command{
		{ID: "types.import", Title: t("cmd.types.import"), Run: func() { a.show(); a.Import() }},
		{ID: "types.copyprompt", Title: t("cmd.types.copyprompt"), Run: func() { a.show(); a.CopyPrompt() }},
	}
}
