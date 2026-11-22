package inbox

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/72nd/sanity/analysis"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
)

type learnedDoc struct {
	typeID, text string
}

// Learner owns the learned index: every filed document teaches the
// classifier. It is safe for concurrent use; Index() is the read side for
// Analyzer worker goroutines.
type Learner struct {
	root  string
	st    store.Store
	eng   pdf.Engine
	cache *Cache

	mu     sync.RWMutex
	idx    *analysis.Index
	ignore []string
	docs   map[string]learnedDoc // by sha256
}

// NewLearner creates an empty learner; call Rebuild to fill it.
func NewLearner(root string, st store.Store, eng pdf.Engine) *Learner {
	l := &Learner{root: root, st: st, eng: eng, cache: OpenCache(CachePath(root)), docs: map[string]learnedDoc{}}
	if p, err := st.Project(context.Background()); err == nil {
		l.ignore = p.IgnoreTokens
	}
	l.idx = analysis.NewIndex(l.ignore)
	return l
}

// Rebuild indexes every library entry. The text comes from the cache, else
// from the filed PDF (same pipeline as Analyzer). Unreadable files are skipped.
func (l *Learner) Rebuild(ctx context.Context) error {
	pr, err := l.st.Project(ctx)
	if err != nil {
		return err
	}
	built := map[string]learnedDoc{}
	dirty := false
	for _, e := range pr.Library {
		if err := ctx.Err(); err != nil {
			return err
		}
		if e.SHA256 == "" {
			continue
		}
		ce, ok := l.cache.Get(e.SHA256)
		if !ok || ce.Text == "" {
			ex, err := ExtractText(ctx, l.eng, filepath.Join(l.root, filepath.FromSlash(e.Path)))
			if err != nil || ex.Text == "" {
				continue
			}
			ce = l.entryFor(ex.Text, ex.Source)
			l.cache.Put(e.SHA256, ce)
			dirty = true
		}
		built[e.SHA256] = learnedDoc{e.TypeID, ce.Text}
	}
	l.mu.Lock()
	for sha, d := range l.docs { // learned while rebuilding
		if _, ok := built[sha]; !ok {
			built[sha] = d
		}
	}
	l.ignore = pr.IgnoreTokens
	l.docs = built
	l.reindexLocked()
	l.mu.Unlock()
	if dirty {
		return l.cache.Save()
	}
	return nil
}

// Learn adds a filed document to the index and the cache and saves the cache.
func (l *Learner) Learn(typeID, sha, text string) error {
	if text == "" || sha == "" {
		return nil
	}
	l.cache.Put(sha, l.entryFor(text, pdf.SourceLayer))
	l.mu.Lock()
	if old, ok := l.docs[sha]; ok && old.typeID != typeID {
		l.docs[sha] = learnedDoc{typeID, text}
		l.reindexLocked()
	} else if !ok {
		l.docs[sha] = learnedDoc{typeID, text}
		l.idx.Add(typeID, text)
	}
	l.mu.Unlock()
	return l.cache.Save()
}

// Forget removes a document (undo of a filing).
func (l *Learner) Forget(sha string) error {
	l.cache.Delete(sha)
	l.mu.Lock()
	if _, ok := l.docs[sha]; ok {
		delete(l.docs, sha)
		l.reindexLocked()
	}
	l.mu.Unlock()
	return l.cache.Save()
}

// Index returns the thread-safe read side of the learned index.
func (l *Learner) Index() analysis.LearnedIndex { return lockedIndex{l} }

func (l *Learner) entryFor(text string, src pdf.TextSource) CacheEntry {
	l.mu.RLock()
	ign := l.ignore
	l.mu.RUnlock()
	return CacheEntry{Text: text, Source: src, TextHash: analysis.TextHash(text), Fingerprint: analysis.Fingerprint(text, ign)}
}

func (l *Learner) reindexLocked() {
	x := analysis.NewIndex(l.ignore)
	for _, d := range l.docs {
		x.Add(d.typeID, d.text)
	}
	l.idx = x
}

type lockedIndex struct{ l *Learner }

func (x lockedIndex) Score(text string) map[string]float64 {
	x.l.mu.RLock()
	defer x.l.mu.RUnlock()
	return x.l.idx.Score(text)
}

func (x lockedIndex) Filed(typeID string) int {
	x.l.mu.RLock()
	defer x.l.mu.RUnlock()
	return x.l.idx.Filed(typeID)
}
