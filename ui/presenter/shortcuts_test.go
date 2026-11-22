package presenter

import "testing"

func ident(s string) string { return s }

var testCmds = []ShortcutCmd{
	{ID: "palette.open", Title: "Command palette", Shortcut: "⌘K"},
	{ID: "sort.file", Title: "File document", Shortcut: "⌘Return"},
	{ID: "sort.undobatch", Title: "Undo batch"},
	{ID: "shortcuts.show", Title: "Show shortcuts", Shortcut: "⌘/"},
}

func find(gs []ShortcutGroup, id string) bool {
	for _, g := range gs {
		for _, r := range g.Rows {
			if r.ID == id {
				return true
			}
		}
	}
	return false
}

func TestShortcutsEveryShortcutAppears(t *testing.T) {
	gs := NewShortcuts(testCmds, true, ident).Groups()
	for _, c := range testCmds {
		if c.Shortcut != "" && !find(gs, c.ID) {
			t.Errorf("command %s missing", c.ID)
		}
	}
	for _, ctx := range []KeyContext{ContextSort, ContextLibrary} {
		for _, b := range KeyBindings(ctx) {
			found := false
			for _, g := range gs {
				for _, r := range g.Rows {
					found = found || (r.Keys == b.Keys && r.ID == b.Cmd)
				}
			}
			if !found {
				t.Errorf("binding %q -> %s missing", b.Keys, b.Cmd)
			}
		}
	}
	if len(gs) != 4 {
		t.Errorf("groups = %d, want 4", len(gs))
	}
}

func TestShortcutsVimOff(t *testing.T) {
	gs := NewShortcuts(testCmds, false, ident).Groups()
	if len(gs) != 2 || find(gs, "next") {
		t.Errorf("vim groups still present: %+v", gs)
	}
	for _, g := range gs {
		for _, r := range g.Rows {
			if r.Keys == "gg" {
				t.Error("vim row present")
			}
		}
	}
}

func TestShortcutsFilter(t *testing.T) {
	s := NewShortcuts(testCmds, true, ident)
	if gs := s.Filter("PALETTE"); !find(gs, "palette.open") || !find(gs, "palette") {
		t.Errorf("filter by title/id: %+v", gs)
	}
	if gs := s.Filter("gg"); !find(gs, "first") || find(gs, "palette.open") {
		t.Errorf("filter by keys: %+v", gs)
	}
	if gs := s.Filter("zzzz"); len(gs) != 0 {
		t.Errorf("no match should give no groups: %+v", gs)
	}
}

func TestKeyBindingsMatchParser(t *testing.T) {
	for _, ctx := range []KeyContext{ContextSort, ContextLibrary} {
		for _, b := range KeyBindings(ctx) {
			rs := []rune(b.Keys)
			if b.Keys == "Enter" {
				k, _ := newFakeKeys(t)
				if got := k.FeedEnter(ctx); got != b.Cmd {
					t.Errorf("Enter = %q, want %q", got, b.Cmd)
				}
				continue
			}
			k, _ := newFakeKeys(t)
			var got string
			for _, r := range rs {
				got, _ = k.Feed(ctx, r)
			}
			if got == "" && len(rs) == 1 { // a prefix key such as d fires after its timeout
				k2, clk := newFakeKeys(t)
				k2.Feed(ctx, rs[0])
				clk.Advance(SequenceTimeout)
				got, _ = k2.Expire()
			}
			if got != b.Cmd {
				t.Errorf("%q = %q, want %q", b.Keys, got, b.Cmd)
			}
		}
	}
}

func TestQuestionMarkOpensShortcuts(t *testing.T) {
	k, _ := newFakeKeys(t)
	if cmd, ok := k.Feed(ContextSort, '?'); cmd != "shortcuts" || !ok {
		t.Errorf("? = %q %v", cmd, ok)
	}
}
