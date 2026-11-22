package inbox

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/72nd/sanity/pdf"
)

// CacheEntry is what is remembered about one filed PDF.
type CacheEntry struct {
	Text        string         `json:"text"`
	Source      pdf.TextSource `json:"source"`
	TextHash    string         `json:"textHash"`
	Fingerprint []string       `json:"fingerprint,omitempty"`
}

// CachePath is the cache file of a project root.
func CachePath(root string) string {
	return filepath.Join(root, "office", ".sanity", "cache.json")
}

// Cache maps SHA256 to extracted text and fingerprint. It is a pure cache: it
// can always be rebuilt from the PDFs. Safe for concurrent use.
type Cache struct {
	path string
	mu   sync.Mutex
	m    map[string]CacheEntry
}

// OpenCache loads the cache at path. A missing or unreadable file yields an
// empty cache (it is rebuilt from the PDFs).
func OpenCache(path string) *Cache {
	c := &Cache{path: path, m: map[string]CacheEntry{}}
	if b, err := os.ReadFile(path); err == nil {
		var m map[string]CacheEntry
		if json.Unmarshal(b, &m) == nil && m != nil {
			c.m = m
		}
	}
	return c
}

// Get returns the entry for a hash.
func (c *Cache) Get(sha string) (CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[sha]
	return e, ok
}

// Put stores an entry in memory; call Save to persist.
func (c *Cache) Put(sha string, e CacheEntry) {
	c.mu.Lock()
	c.m[sha] = e
	c.mu.Unlock()
}

// Delete removes an entry in memory.
func (c *Cache) Delete(sha string) {
	c.mu.Lock()
	delete(c.m, sha)
	c.mu.Unlock()
}

// Len is the number of entries.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// Save writes the cache atomically (temp file in the same directory, rename).
func (c *Cache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := json.Marshal(c.m)
	if err != nil {
		return err
	}
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "cache-*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(b)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), c.path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
