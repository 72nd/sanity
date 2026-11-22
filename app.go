package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/72nd/fine/shell/i18n"
	"github.com/72nd/sanity/domain"
	"github.com/72nd/sanity/inbox"
	"github.com/72nd/sanity/pdf"
	"github.com/72nd/sanity/store"
	"github.com/72nd/sanity/ui"
	"github.com/72nd/sanity/ui/presenter"
)

// newEngine creates the PDF engine of a project; tests replace it.
var newEngine = pdf.NewEngine

// services is everything that belongs to one open project: store, inbox queue
// and watcher, filer, analyzer, learner and the presenters. Close stops it so
// another project can take over.
type services struct {
	Root string
	Name string

	st      store.Store
	queue   *inbox.Queue
	stop    func() // stops the inbox watcher
	cancel  context.CancelFunc
	lib     *presenter.Library
	sort    *presenter.Sort
	types   *presenter.Types
	project *presenter.Project
}

// openServices builds the services for root. With demo set it runs on the
// fake store and never writes to root. act and keys outlive the services:
// the commands and the Vim layer are registered once.
func openServices(root string, demo bool, act *ui.SortActions) (*services, error) {
	var st store.Store
	if demo {
		st = store.NewFake()
	} else {
		js, err := store.OpenJSON(root)
		if err != nil {
			return nil, err
		}
		st = js
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	pr, err := st.Project(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	inboxRel := pr.Inbox
	if inboxRel == "" {
		inboxRel = domain.DefaultInbox
	}

	q := inbox.NewQueue(nil)
	stop, err := inbox.Watch(q, filepath.Join(root, filepath.FromSlash(inboxRel)), 400*time.Millisecond)
	if err != nil {
		cancel()
		return nil, err
	}

	engine := newEngine()
	// Vision warms up in the background so the first scan is not slow; open
	// documents are released with the project.
	go engine.OCRAvailable()
	context.AfterFunc(ctx, func() {
		if c, ok := engine.(pdf.Closer); ok {
			c.Close()
		}
	})
	learner := inbox.NewLearner(root, st, engine)
	go func() { // fill the learned index from the library in the background
		if err := learner.Rebuild(ctx); err != nil && ctx.Err() == nil {
			fmt.Fprintln(os.Stderr, "sanity: learning:", err)
		}
	}()
	filer := inbox.NewFiler(root, st)
	filer.OnFiled = func(e domain.LibraryEntry, text string) {
		if err := learner.Learn(e.TypeID, e.SHA256, text); err != nil {
			fmt.Fprintln(os.Stderr, "sanity: learning:", err)
		}
	}
	filer.OnUndone = func(e domain.LibraryEntry) {
		if err := learner.Forget(e.SHA256); err != nil {
			fmt.Fprintln(os.Stderr, "sanity: learning:", err)
		}
	}
	sortP := presenter.NewSort(st, q, filer, engine, i18n.T)
	sortP.SetAnalyzer(&inbox.Analyzer{PDF: engine, Store: st, Learned: learner.Index()})

	return &services{
		Root: root, Name: pr.Name, st: st, queue: q, stop: stop, cancel: cancel,
		lib: presenter.NewLibrary(st, i18n.T), sort: sortP,
		types: presenter.NewTypes(st, i18n.T), project: presenter.NewProject(st, i18n.T),
	}, nil
}

// Close stops the watcher and cancels background work. It is safe to call twice.
func (s *services) Close() {
	if s.stop != nil {
		s.stop()
		s.stop = nil
	}
	s.cancel()
}
