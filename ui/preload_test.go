package ui

import (
	"context"
	"testing"

	"github.com/72nd/sanity/ui/presenter"
)

// preload fills the presenter before its page is built. The Fyne test driver
// runs fyne.Do on the calling goroutine, so a page that loads in the
// background would lay itself out concurrently with the test.
func preload(t *testing.T, tp *presenter.Types) {
	t.Helper()
	if err := tp.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
}
