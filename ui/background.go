package ui

import "time"

// background runs f off the UI goroutine, so a page never blocks on the store
// while it is built. Tests swap it for a synchronous call (SyncBackground):
// the Fyne test driver runs fyne.Do on the calling goroutine, so a real
// goroutine would lay the page out concurrently with the test.
var background = func(f func()) { go f() }

// toastOverride, when set, replaces every toast duration. Tests make toasts
// outlive the test, because a toast timer that fires in the middle of another
// test lays the toast host out concurrently with it.
var toastOverride time.Duration

// toastDur is how long a toast stays.
func toastDur(d time.Duration) time.Duration {
	if toastOverride > 0 {
		return toastOverride
	}
	return d
}

// SyncBackground makes the pages' load-on-build work run inline and toasts
// stay for an hour. It is for tests only; call it from TestMain.
func SyncBackground() {
	background = func(f func()) { f() }
	toastOverride = time.Hour
}
