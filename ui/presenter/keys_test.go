package presenter

import (
	"testing"
	"time"
)

// fakeClock is a clock that only moves when a test advances it.
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newFakeKeys(t *testing.T) (*Keys, *fakeClock) {
	t.Helper()
	clk := &fakeClock{t: time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)}
	return NewKeys(clk.Now), clk
}

// keyStep is one key press in a sequence test. Wait is the time before the key.
// Enter sends Enter instead of a rune, and Drain is the expected queue after it.
type keyStep struct {
	name     string
	wait     time.Duration
	ctx      KeyContext
	r        rune
	enter    bool
	cmd      string
	consumed bool
	drain    []Result
}

func runSteps(t *testing.T, steps []keyStep) {
	t.Helper()
	k, clk := newFakeKeys(t)
	for _, s := range steps {
		clk.Advance(s.wait)
		var cmd string
		var consumed bool
		if s.enter {
			cmd = k.FeedEnter(s.ctx)
			consumed = cmd != ""
		} else {
			cmd, consumed = k.Feed(s.ctx, s.r)
		}
		if cmd != s.cmd || consumed != s.consumed {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", s.name, cmd, consumed, s.cmd, s.consumed)
		}
		got := k.Drain()
		if !sameResults(got, s.drain) {
			t.Errorf("%s: drain got %v, want %v", s.name, got, s.drain)
		}
	}
}

func sameResults(a, b []Result) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortSingles(t *testing.T) {
	tests := []struct {
		r   rune
		cmd string
	}{
		{'j', "next"}, {'k', "prev"}, {'G', "last"}, {'h', "page.prev"}, {'l', "page.next"},
		{'+', "zoom.in"}, {'-', "zoom.out"}, {'=', "zoom.fit"}, {'t', "focus.type"},
		{'i', "focus.first"}, {'x', "reveal"}, {'u', "undo"}, {'Y', "copy.abs"},
		{'o', "add"}, {'/', "search"}, {':', "palette"},
	}
	for _, tt := range tests {
		t.Run(string(tt.r), func(t *testing.T) {
			k, _ := newFakeKeys(t)
			cmd, consumed := k.Feed(ContextSort, tt.r)
			if cmd != tt.cmd || !consumed {
				t.Errorf("Feed(%q) = (%q, %v), want (%q, true)", tt.r, cmd, consumed, tt.cmd)
			}
		})
	}
}

func TestLibrarySingles(t *testing.T) {
	tests := []struct {
		r   rune
		cmd string
	}{
		{'j', "row.down"}, {'k', "row.up"}, {'G', "row.last"}, {'/', "search"},
		{'n', "hit.next"}, {'N', "hit.prev"}, {'v', "select"}, {'V', "select.rows"},
		{'x', "reveal"}, {'r', "resort"},
	}
	for _, tt := range tests {
		t.Run(string(tt.r), func(t *testing.T) {
			k, _ := newFakeKeys(t)
			cmd, consumed := k.Feed(ContextLibrary, tt.r)
			if cmd != tt.cmd || !consumed {
				t.Errorf("Feed(%q) = (%q, %v), want (%q, true)", tt.r, cmd, consumed, tt.cmd)
			}
		})
	}
}

func TestUnknownRunesNotConsumed(t *testing.T) {
	tests := []struct {
		name string
		ctx  KeyContext
		r    rune
	}{
		{"sort letter", ContextSort, 'z'},
		{"sort digit", ContextSort, '1'},
		{"library tab-only key", ContextLibrary, 't'},
		{"library focus.date key", ContextLibrary, 'd'},
		{"library delete key", ContextLibrary, 'o'},
		{"library upper Y", ContextLibrary, 'Y'},
		{"library upper H", ContextLibrary, 'H'},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, _ := newFakeKeys(t)
			cmd, consumed := k.Feed(tt.ctx, tt.r)
			if cmd != "" || consumed {
				t.Errorf("Feed(%q) = (%q, %v), want (\"\", false)", tt.r, cmd, consumed)
			}
		})
	}
}

func TestSequences(t *testing.T) {
	tests := []struct {
		name  string
		steps []keyStep
	}{
		{
			name: "gg within timeout",
			steps: []keyStep{
				{name: "g", ctx: ContextSort, r: 'g', consumed: true},
				{name: "g again", wait: 300 * time.Millisecond, ctx: ContextSort, r: 'g', cmd: "first", consumed: true},
			},
		},
		{
			name: "gg at exactly the timeout is too late",
			steps: []keyStep{
				{name: "g", ctx: ContextSort, r: 'g', consumed: true},
				{name: "g after 800ms", wait: SequenceTimeout, ctx: ContextSort, r: 'g', consumed: true},
				{name: "third g completes", wait: 10 * time.Millisecond, ctx: ContextSort, r: 'g', cmd: "first", consumed: true},
			},
		},
		{
			name: "gg just inside the timeout",
			steps: []keyStep{
				{name: "g", ctx: ContextSort, r: 'g', consumed: true},
				{name: "g at 799ms", wait: SequenceTimeout - time.Millisecond, ctx: ContextSort, r: 'g', cmd: "first", consumed: true},
			},
		},
		{
			name: "yy copies relative path",
			steps: []keyStep{
				{name: "y", ctx: ContextSort, r: 'y', consumed: true},
				{name: "y again", ctx: ContextSort, r: 'y', cmd: "copy.rel", consumed: true},
			},
		},
		{
			name: "y then Y is not yy",
			steps: []keyStep{
				{name: "y", ctx: ContextSort, r: 'y', consumed: true},
				{name: "Y", ctx: ContextSort, r: 'Y', cmd: "copy.abs", consumed: true},
			},
		},
		{
			name: "yy timeout restarts the sequence",
			steps: []keyStep{
				{name: "y", ctx: ContextSort, r: 'y', consumed: true},
				{name: "y after timeout", wait: SequenceTimeout, ctx: ContextSort, r: 'y', consumed: true},
				{name: "y completes", ctx: ContextSort, r: 'y', cmd: "copy.rel", consumed: true},
			},
		},
		{
			name: "dd removes",
			steps: []keyStep{
				{name: "d", ctx: ContextSort, r: 'd', consumed: true},
				{name: "d again", wait: 100 * time.Millisecond, ctx: ContextSort, r: 'd', cmd: "remove", consumed: true},
			},
		},
		{
			name: "d then d after timeout fires focus.date and starts a new d",
			steps: []keyStep{
				{name: "d", ctx: ContextSort, r: 'd', consumed: true},
				{
					name: "d after timeout", wait: SequenceTimeout, ctx: ContextSort, r: 'd',
					cmd: "focus.date", consumed: true,
					drain: []Result{{Consumed: true}},
				},
				{name: "new d completes remove", ctx: ContextSort, r: 'd', cmd: "remove", consumed: true},
			},
		},
		{
			name: "d then j fires focus.date then next",
			steps: []keyStep{
				{name: "d", ctx: ContextSort, r: 'd', consumed: true},
				{
					name: "j", wait: 200 * time.Millisecond, ctx: ContextSort, r: 'j',
					cmd: "focus.date", consumed: true,
					drain: []Result{{Cmd: "next", Consumed: true}},
				},
			},
		},
		{
			name: "d then unknown key fires focus.date and passes the key on",
			steps: []keyStep{
				{name: "d", ctx: ContextSort, r: 'd', consumed: true},
				{
					name: "z", ctx: ContextSort, r: 'z',
					cmd: "focus.date", consumed: true,
					drain: []Result{{}},
				},
			},
		},
		{
			name: "d then x fires focus.date then reveal",
			steps: []keyStep{
				{name: "d", ctx: ContextSort, r: 'd', consumed: true},
				{
					name: "x", ctx: ContextSort, r: 'x',
					cmd: "focus.date", consumed: true,
					drain: []Result{{Cmd: "reveal", Consumed: true}},
				},
			},
		},
		{
			name: "d then timeout then j fires focus.date then next",
			steps: []keyStep{
				{name: "d", ctx: ContextSort, r: 'd', consumed: true},
				{
					name: "j after timeout", wait: SequenceTimeout, ctx: ContextSort, r: 'j',
					cmd: "focus.date", consumed: true,
					drain: []Result{{Cmd: "next", Consumed: true}},
				},
			},
		},
		{
			name: "lone d waits for a second key",
			steps: []keyStep{
				{name: "d", ctx: ContextSort, r: 'd', consumed: true},
			},
		},
		{
			name: "g then d drops g and starts a d",
			steps: []keyStep{
				{name: "g", ctx: ContextSort, r: 'g', consumed: true},
				{name: "d", ctx: ContextSort, r: 'd', consumed: true},
				{name: "d completes remove", ctx: ContextSort, r: 'd', cmd: "remove", consumed: true},
			},
		},
		{
			name: "g then unknown key drops g and is not consumed",
			steps: []keyStep{
				{name: "g", ctx: ContextSort, r: 'g', consumed: true},
				{name: "z", ctx: ContextSort, r: 'z'},
				{name: "G after the failed g", ctx: ContextSort, r: 'G', cmd: "last", consumed: true},
			},
		},
		{
			name: "g then G is last, not first",
			steps: []keyStep{
				{name: "g", ctx: ContextSort, r: 'g', consumed: true},
				{name: "G", ctx: ContextSort, r: 'G', cmd: "last", consumed: true},
			},
		},
		{
			name: "library gg and G",
			steps: []keyStep{
				{name: "g", ctx: ContextLibrary, r: 'g', consumed: true},
				{name: "g", ctx: ContextLibrary, r: 'g', cmd: "row.first", consumed: true},
				{name: "G", ctx: ContextLibrary, r: 'G', cmd: "row.last", consumed: true},
			},
		},
		{
			name: "library yy copies relative paths",
			steps: []keyStep{
				{name: "y", ctx: ContextLibrary, r: 'y', consumed: true},
				{name: "y", ctx: ContextLibrary, r: 'y', cmd: "copy.rel", consumed: true},
			},
		},
		{
			name: "library y then j drops y and moves down",
			steps: []keyStep{
				{name: "y", ctx: ContextLibrary, r: 'y', consumed: true},
				{name: "j", ctx: ContextLibrary, r: 'j', cmd: "row.down", consumed: true},
			},
		},
		{
			name: "library has no dd",
			steps: []keyStep{
				{name: "d", ctx: ContextLibrary, r: 'd'},
				{name: "d", ctx: ContextLibrary, r: 'd'},
			},
		},
		{
			name: "pending d from sort is dropped in library without firing",
			steps: []keyStep{
				{name: "d in sort", ctx: ContextSort, r: 'd', consumed: true},
				{name: "d in library", ctx: ContextLibrary, r: 'd'},
				{name: "j in library", ctx: ContextLibrary, r: 'j', cmd: "row.down", consumed: true},
			},
		},
		{
			name: "pending g from library is dropped in sort",
			steps: []keyStep{
				{name: "g in library", ctx: ContextLibrary, r: 'g', consumed: true},
				{name: "g in sort", ctx: ContextSort, r: 'g', consumed: true},
				{name: "g in sort completes", ctx: ContextSort, r: 'g', cmd: "first", consumed: true},
			},
		},
		{
			name: "enter in library opens and clears pending g",
			steps: []keyStep{
				{name: "g", ctx: ContextLibrary, r: 'g', consumed: true},
				{name: "enter", ctx: ContextLibrary, enter: true, cmd: "open", consumed: true},
				{name: "g after enter starts over", ctx: ContextLibrary, r: 'g', consumed: true},
			},
		},
		{
			name: "enter in sort drops a pending d without firing focus.date",
			steps: []keyStep{
				{name: "d", ctx: ContextSort, r: 'd', consumed: true},
				{name: "enter", ctx: ContextSort, enter: true},
				{name: "j after enter", ctx: ContextSort, r: 'j', cmd: "next", consumed: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSteps(t, tt.steps)
		})
	}
}

func TestExpire(t *testing.T) {
	tests := []struct {
		name  string
		steps func(k *Keys, clk *fakeClock) (string, bool)
		cmd   string
		ok    bool
	}{
		{
			name: "nothing pending",
			steps: func(k *Keys, clk *fakeClock) (string, bool) {
				clk.Advance(time.Hour)
				return k.Expire()
			},
		},
		{
			name: "lone d before timeout",
			steps: func(k *Keys, clk *fakeClock) (string, bool) {
				k.Feed(ContextSort, 'd')
				clk.Advance(SequenceTimeout - time.Millisecond)
				return k.Expire()
			},
		},
		{
			name: "lone d after timeout fires focus.date",
			steps: func(k *Keys, clk *fakeClock) (string, bool) {
				k.Feed(ContextSort, 'd')
				clk.Advance(SequenceTimeout)
				return k.Expire()
			},
			cmd: "focus.date", ok: true,
		},
		{
			name: "expired d is cleared, so a second expire is empty",
			steps: func(k *Keys, clk *fakeClock) (string, bool) {
				k.Feed(ContextSort, 'd')
				clk.Advance(SequenceTimeout)
				k.Expire()
				return k.Expire()
			},
		},
		{
			name: "pending g has no single to fire",
			steps: func(k *Keys, clk *fakeClock) (string, bool) {
				k.Feed(ContextSort, 'g')
				clk.Advance(SequenceTimeout)
				return k.Expire()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, clk := newFakeKeys(t)
			cmd, ok := tt.steps(k, clk)
			if cmd != tt.cmd || ok != tt.ok {
				t.Errorf("Expire() = (%q, %v), want (%q, %v)", cmd, ok, tt.cmd, tt.ok)
			}
		})
	}
}

func TestResetDropsPendingWithoutFiring(t *testing.T) {
	k, _ := newFakeKeys(t)
	k.Feed(ContextSort, 'd')
	k.Reset()
	cmd, consumed := k.Feed(ContextSort, 'j')
	if cmd != "next" || !consumed {
		t.Errorf("after Reset, j = (%q, %v), want (\"next\", true)", cmd, consumed)
	}
	if got := k.Drain(); len(got) != 0 {
		t.Errorf("after Reset, drain = %v, want none (no focus.date)", got)
	}
}

func TestResetDropsPendingG(t *testing.T) {
	k, _ := newFakeKeys(t)
	k.Feed(ContextLibrary, 'g')
	k.Reset()
	if cmd, consumed := k.Feed(ContextLibrary, 'G'); cmd != "row.last" || !consumed {
		t.Errorf("after Reset, G = (%q, %v), want (\"row.last\", true)", cmd, consumed)
	}
}

func TestEnterCommands(t *testing.T) {
	tests := []struct {
		ctx KeyContext
		cmd string
	}{
		{ContextSort, ""},
		{ContextLibrary, "open"},
	}
	for _, tt := range tests {
		k, _ := newFakeKeys(t)
		if got := k.FeedEnter(tt.ctx); got != tt.cmd {
			t.Errorf("FeedEnter(%d) = %q, want %q", tt.ctx, got, tt.cmd)
		}
	}
}

func TestDrainIsCleared(t *testing.T) {
	k, clk := newFakeKeys(t)
	k.Feed(ContextSort, 'd')
	clk.Advance(SequenceTimeout)
	k.Feed(ContextSort, 'j')
	if got := k.Drain(); len(got) != 1 {
		t.Fatalf("first drain = %v, want one result", got)
	}
	if got := k.Drain(); got != nil {
		t.Errorf("second drain = %v, want nil", got)
	}
}
