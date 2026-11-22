package presenter

import "strings"

// ShortcutCmd is a registered command as the shortcut overview needs it.
// Shortcut is the display label ("" when the command has none).
type ShortcutCmd struct {
	ID, Title, Shortcut string
}

// ShortcutRow is one line of the overview.
type ShortcutRow struct {
	Keys  string // the key label, for example "⌘/" or "gg"
	Title string
	ID    string // command id
}

// ShortcutGroup is a titled block of rows.
type ShortcutGroup struct {
	Title string
	Rows  []ShortcutRow
}

// Shortcuts is the headless model of the shortcut overview.
type Shortcuts struct {
	groups []ShortcutGroup
}

// NewShortcuts builds the overview from the registered commands and the key
// tables. t translates message ids (an id it does not know comes back
// unchanged, which selects the English fallback). Vim groups are left out when
// vim is false. Commands without a shortcut are not listed; commands with an id
// starting with "sort." go to the Sort group, all others to Everywhere.
func NewShortcuts(cmds []ShortcutCmd, vim bool, t func(string) string) *Shortcuts {
	every := ShortcutGroup{Title: t("shortcuts.group.everywhere")}
	sortCmd := ShortcutGroup{Title: t("shortcuts.group.sort.cmd")}
	for _, c := range cmds {
		if c.Shortcut == "" {
			continue
		}
		row := ShortcutRow{Keys: c.Shortcut, Title: c.Title, ID: c.ID}
		if strings.HasPrefix(c.ID, "sort.") {
			sortCmd.Rows = append(sortCmd.Rows, row)
		} else {
			every.Rows = append(every.Rows, row)
		}
	}
	groups := []ShortcutGroup{every, sortCmd}
	if vim {
		groups = append(groups,
			vimGroup(t("shortcuts.group.sort.vim"), ContextSort, t),
			vimGroup(t("shortcuts.group.library.vim"), ContextLibrary, t))
	}
	return &Shortcuts{groups: nonEmpty(groups)}
}

func vimGroup(title string, ctx KeyContext, t func(string) string) ShortcutGroup {
	g := ShortcutGroup{Title: title}
	for _, b := range KeyBindings(ctx) {
		desc := t("shortcuts.vim." + b.Cmd)
		if desc == "shortcuts.vim."+b.Cmd || desc == "" {
			desc = KeyDescription(b.Cmd)
		}
		g.Rows = append(g.Rows, ShortcutRow{Keys: b.Keys, Title: desc, ID: b.Cmd})
	}
	return g
}

func nonEmpty(gs []ShortcutGroup) []ShortcutGroup {
	var out []ShortcutGroup
	for _, g := range gs {
		if len(g.Rows) > 0 {
			out = append(out, g)
		}
	}
	return out
}

// Groups returns all groups.
func (s *Shortcuts) Groups() []ShortcutGroup { return s.Filter("") }

// Filter returns the rows whose title, keys or command id contain q (case
// insensitive). Groups without a match are dropped.
func (s *Shortcuts) Filter(q string) []ShortcutGroup {
	q = strings.ToLower(strings.TrimSpace(q))
	var out []ShortcutGroup
	for _, g := range s.groups {
		ng := ShortcutGroup{Title: g.Title}
		for _, r := range g.Rows {
			if q == "" || strings.Contains(strings.ToLower(r.Title), q) ||
				strings.Contains(strings.ToLower(r.Keys), q) || strings.Contains(strings.ToLower(r.ID), q) {
				ng.Rows = append(ng.Rows, r)
			}
		}
		if len(ng.Rows) > 0 {
			out = append(out, ng)
		}
	}
	return out
}
