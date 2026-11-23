package presenter

import (
	"os"
	"path/filepath"
	"slices"
)

// RecentsKey is the preference key of the recent projects list.
const RecentsKey = "sanity.recent"

// MaxRecents is how many recent projects are remembered.
const MaxRecents = 8

// PrefStore is the part of shell.Prefs the recents list needs; *shell.Prefs
// satisfies it.
type PrefStore interface {
	LoadJSON(key string, v any) bool
	SaveJSON(key string, v any)
}

// RecentProject is one remembered project.
type RecentProject struct {
	Root string `json:"root"`
	Name string `json:"name"`
}

// Recents is the list of recently opened projects, most recent first.
type Recents struct {
	prefs PrefStore
	// Exists reports whether root still holds a project; tests replace it.
	Exists func(root string) bool
}

// NewRecents creates the list on top of the preferences.
func NewRecents(p PrefStore) *Recents {
	return &Recents{prefs: p, Exists: HasProject}
}

// ProjectFile returns the path of sanity.json below root.
func ProjectFile(root string) string { return filepath.Join(root, "office", "sanity.json") }

// HasProject reports whether root holds office/sanity.json.
func HasProject(root string) bool {
	st, err := os.Stat(ProjectFile(root))
	return err == nil && !st.IsDir()
}

func (r *Recents) load() []RecentProject {
	var list []RecentProject
	if !r.prefs.LoadJSON(RecentsKey, &list) {
		return nil
	}
	return list
}

// List returns the recent projects, most recent first. Folders that no longer
// have office/sanity.json are dropped, and the pruned list is stored.
func (r *Recents) List() []RecentProject {
	all := r.load()
	kept := make([]RecentProject, 0, len(all))
	for _, p := range all {
		if p.Root != "" && r.Exists(p.Root) {
			kept = append(kept, p)
		}
	}
	if len(kept) != len(all) {
		r.prefs.SaveJSON(RecentsKey, kept)
	}
	return kept
}

// Add moves root to the front (updating its name), removes duplicates and
// keeps at most MaxRecents entries.
func (r *Recents) Add(root, name string) {
	root = filepath.Clean(root)
	list := []RecentProject{{Root: root, Name: name}}
	for _, p := range r.load() {
		if filepath.Clean(p.Root) != root {
			list = append(list, p)
		}
	}
	if len(list) > MaxRecents {
		list = slices.Clone(list[:MaxRecents])
	}
	r.prefs.SaveJSON(RecentsKey, list)
}

// Remove forgets root.
func (r *Recents) Remove(root string) {
	root = filepath.Clean(root)
	list := slices.DeleteFunc(r.load(), func(p RecentProject) bool { return filepath.Clean(p.Root) == root })
	r.prefs.SaveJSON(RecentsKey, list)
}

// MenuKind says what a row of the project menu does.
type MenuKind int

const (
	MenuCurrent  MenuKind = iota // the open project (first row)
	MenuRecent                   // another recent project: open Root
	MenuOpen                     // choose a folder
	MenuOverview                 // back to the welcome page
)

// MenuEntry is one row of the project menu in the toolbar.
type MenuEntry struct {
	Kind  MenuKind
	Label string
	Root  string // for MenuCurrent and MenuRecent
}

// Menu returns the rows of the project menu: the open project, the other
// recent projects, then "open project" and "overview". Labels are unique:
// projects with the same name get their folder appended.
func (r *Recents) Menu(curRoot, curName string, t Translator) []MenuEntry {
	curRoot = filepath.Clean(curRoot)
	items := []RecentProject{{Root: curRoot, Name: curName}}
	for _, p := range r.List() {
		if filepath.Clean(p.Root) != curRoot {
			items = append(items, p)
		}
	}
	count := map[string]int{}
	for _, p := range items {
		count[p.Name]++
	}
	out := make([]MenuEntry, 0, len(items)+2)
	for i, p := range items {
		label := p.Name
		if count[p.Name] > 1 {
			label += "  (" + p.Root + ")"
		}
		kind := MenuRecent
		if i == 0 {
			kind = MenuCurrent
		}
		out = append(out, MenuEntry{Kind: kind, Label: label, Root: p.Root})
	}
	return append(out,
		MenuEntry{Kind: MenuOpen, Label: t("project.menu.open")},
		MenuEntry{Kind: MenuOverview, Label: t("project.menu.overview")})
}
