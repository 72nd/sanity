package inbox

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Status of an inbox item.
type Status int

const (
	StatusQueued    Status = iota // imported, analysis not started
	StatusAnalysing               // analysis running
	StatusReady                   // analysis done
	StatusFailed                  // analysis failed
)

// Item is an imported PDF waiting to be filed. It stays where it is until filed.
type Item struct {
	ID      string // the absolute path
	Path    string
	Name    string
	AddedAt time.Time
	Status  Status
	Err     string
}

// Queue is the ordered set of inbox items. It is safe for concurrent use.
type Queue struct {
	mu       sync.Mutex
	items    []*Item
	onChange func()
	now      func() time.Time
}

// NewQueue creates an empty queue. onChange (may be nil) is called after every
// change, outside the lock, possibly from another goroutine.
func NewQueue(onChange func()) *Queue { return &Queue{onChange: onChange, now: time.Now} }

// IsPDF reports whether the path looks like a PDF file.
func IsPDF(path string) bool { return strings.EqualFold(filepath.Ext(path), ".pdf") }

// Add imports files and folders (recursively, PDFs only, hidden entries
// skipped). Known paths are ignored. It returns the number of new items.
func (q *Queue) Add(paths ...string) int {
	var found []string
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if !st.IsDir() {
			if IsPDF(p) {
				found = append(found, p)
			}
			continue
		}
		_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") && path != p {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() && IsPDF(path) {
				found = append(found, path)
			}
			return nil
		})
	}
	sort.Strings(found)
	n := 0
	q.mu.Lock()
	for _, p := range found {
		abs, err := filepath.Abs(p)
		if err != nil || q.indexLocked(abs) >= 0 {
			continue
		}
		q.items = append(q.items, &Item{ID: abs, Path: abs, Name: filepath.Base(abs), AddedAt: q.now()})
		n++
	}
	q.mu.Unlock()
	if n > 0 {
		q.changed()
	}
	return n
}

// Remove drops an item from the queue (the file is not touched).
func (q *Queue) Remove(id string) bool {
	q.mu.Lock()
	i := q.indexLocked(id)
	if i >= 0 {
		q.items = append(q.items[:i], q.items[i+1:]...)
	}
	q.mu.Unlock()
	if i >= 0 {
		q.changed()
	}
	return i >= 0
}

// SetStatus updates an item's status.
func (q *Queue) SetStatus(id string, s Status, errText string) {
	q.mu.Lock()
	i := q.indexLocked(id)
	if i >= 0 {
		q.items[i].Status, q.items[i].Err = s, errText
	}
	q.mu.Unlock()
	if i >= 0 {
		q.changed()
	}
}

// Items returns a snapshot in queue order.
func (q *Queue) Items() []Item {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Item, len(q.items))
	for i, it := range q.items {
		out[i] = *it
	}
	return out
}

// SetOnChange replaces the change callback.
func (q *Queue) SetOnChange(f func()) {
	q.mu.Lock()
	q.onChange = f
	q.mu.Unlock()
}

// Len is the number of items.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Get returns the item with the given id.
func (q *Queue) Get(id string) (Item, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if i := q.indexLocked(id); i >= 0 {
		return *q.items[i], true
	}
	return Item{}, false
}

// Prune drops items whose file no longer exists (moved or deleted outside).
func (q *Queue) Prune() int {
	q.mu.Lock()
	kept := q.items[:0]
	n := 0
	for _, it := range q.items {
		if _, err := os.Stat(it.Path); err != nil {
			n++
			continue
		}
		kept = append(kept, it)
	}
	q.items = kept
	q.mu.Unlock()
	if n > 0 {
		q.changed()
	}
	return n
}

func (q *Queue) indexLocked(id string) int {
	for i, it := range q.items {
		if it.ID == id {
			return i
		}
	}
	return -1
}

func (q *Queue) changed() {
	q.mu.Lock()
	f := q.onChange
	q.mu.Unlock()
	if f != nil {
		f()
	}
}
