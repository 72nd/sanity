package inbox

import (
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch imports the PDFs already in dir and every PDF that appears later.
// A new file is added after it has been quiet for settle (it may still be
// written). It creates dir if missing. Call the returned stop function to end it.
func Watch(q *Queue, dir string, settle time.Duration) (stop func(), err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, err
	}
	q.Add(dir)
	done := make(chan struct{})
	go func() {
		pending := map[string]*time.Timer{}
		for {
			select {
			case <-done:
				for _, t := range pending {
					t.Stop()
				}
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				p := filepath.Clean(ev.Name)
				switch {
				case ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename):
					if t := pending[p]; t != nil {
						t.Stop()
						delete(pending, p)
					}
					q.Prune()
				case ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write):
					if !IsPDF(p) {
						continue
					}
					if t := pending[p]; t != nil {
						t.Reset(settle)
						continue
					}
					pending[p] = time.AfterFunc(settle, func() { q.Add(p) })
				}
			case <-w.Errors:
			}
		}
	}()
	return func() { close(done); w.Close() }, nil
}
