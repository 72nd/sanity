package presenter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type memPrefs map[string]string

func (m memPrefs) LoadJSON(key string, v any) bool {
	s, ok := m[key]
	return ok && json.Unmarshal([]byte(s), v) == nil
}
func (m memPrefs) SaveJSON(key string, v any) { b, _ := json.Marshal(v); m[key] = string(b) }

func roots(l []RecentProject) []string {
	out := []string{}
	for _, p := range l {
		out = append(out, p.Root)
	}
	return out
}

func TestRecentsOrderDedupeAndLimit(t *testing.T) {
	r := NewRecents(memPrefs{})
	r.Exists = func(string) bool { return true }
	for i := 0; i < 10; i++ {
		r.Add(fmt.Sprintf("/p/%d", i), "n")
	}
	got := roots(r.List())
	if len(got) != MaxRecents || got[0] != "/p/9" || got[MaxRecents-1] != "/p/2" {
		t.Errorf("limit/order: %v", got)
	}
	r.Add("/p/5/", "renamed") // existing (cleaned) moves to the front
	got = roots(r.List())
	if !reflect.DeepEqual(got[:3], []string{"/p/5", "/p/9", "/p/8"}) || len(got) != MaxRecents {
		t.Errorf("dedupe: %v", got)
	}
	if r.List()[0].Name != "renamed" {
		t.Error("name not updated")
	}
	r.Remove("/p/9")
	if got = roots(r.List()); got[1] != "/p/8" || len(got) != MaxRecents-1 {
		t.Errorf("remove: %v", got)
	}
}

func TestRecentsDropMissingProjects(t *testing.T) {
	prefs := memPrefs{}
	r := NewRecents(prefs)
	keep, gone := t.TempDir(), t.TempDir()
	for _, d := range []string{keep, gone} {
		if err := os.MkdirAll(filepath.Join(d, "office"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ProjectFile(d), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		r.Add(d, "x")
	}
	if err := os.Remove(ProjectFile(gone)); err != nil {
		t.Fatal(err)
	}
	if got := roots(r.List()); !reflect.DeepEqual(got, []string{keep}) {
		t.Errorf("List = %v, want only %v", got, keep)
	}
	var stored []RecentProject
	prefs.LoadJSON(RecentsKey, &stored)
	if len(stored) != 1 {
		t.Errorf("pruned list not stored: %v", stored)
	}
	if !HasProject(keep) || HasProject(gone) {
		t.Error("HasProject")
	}
}
