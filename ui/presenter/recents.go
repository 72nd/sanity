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
