package presenter

import (
	"sort"
	"time"
)

// SequenceTimeout is how long a pending key waits for its second key. A second
// key that arrives later starts over, as in Vim's timeoutlen.
const SequenceTimeout = 800 * time.Millisecond

// KeyContext is the screen that has the focus when a key arrives. The same rune
// can mean different commands in different contexts.
type KeyContext int

const (
	// ContextSort is the sort screen with the queue and the preview.
	ContextSort KeyContext = iota
	// ContextLibrary is the library grid.
	ContextLibrary
)

// Result is one command produced by a key. Consumed is false for keys that are
// not part of the vim layer, so the caller passes them on unchanged.
type Result struct {
	Cmd      string
	Consumed bool
}

// keymap holds the normal mode bindings of one context.
type keymap struct {
	// singles maps a rune to a command that fires at once. A rune that is also
	// a sequence prefix fires its single only when the sequence fails.
	singles map[rune]string
	// seqs maps a two-rune sequence to its command. The first rune of each
	// sequence starts a pending key.
	seqs map[string]string
}

// keymaps are the normal mode tables from PLAN.md section 2.
//
// Choice for `d` in Sort: `d` alone is focus.date, but `dd` is remove. A `d`
// therefore waits for SequenceTimeout. A second `d` gives remove. Any other key,
// or the timeout (seen by the next key, Expire or Reset), fires focus.date first,
// and then that key is processed as normal. The focus.date of a lone `d` is
// delayed by up to SequenceTimeout, which is the price of having dd.
var keymaps = map[KeyContext]keymap{
	ContextSort: {
		singles: map[rune]string{
			'j': "next",
			'k': "prev",
			'G': "last",
			'h': "page.prev",
			'l': "page.next",
			'+': "zoom.in",
			'-': "zoom.out",
			'=': "zoom.fit",
			't': "focus.type",
			'd': "focus.date",
			'i': "focus.first",
			'x': "reveal",
			'u': "undo",
			'Y': "copy.abs",
			'o': "add",
			'/': "search",
			':': "palette",
			'?': "shortcuts",
		},
		seqs: map[string]string{
			"gg": "first",
			"yy": "copy.rel",
			"dd": "remove",
		},
	},
	ContextLibrary: {
		singles: map[rune]string{
			'j': "row.down",
			'k': "row.up",
			'G': "row.last",
			'/': "search",
			'n': "hit.next",
			'N': "hit.prev",
			'v': "select",
			'V': "select.rows",
			'x': "reveal",
			'r': "resort",
		},
		seqs: map[string]string{
			"gg": "row.first",
			"yy": "copy.rel",
		},
	},
}

// enterCmds are the commands for Enter, which is not a rune. Sort has no plain
// Enter command, because filing stays Cmd+Enter.
var enterCmds = map[KeyContext]string{
	ContextSort:    "",
	ContextLibrary: "open",
}

// Keys is the headless vim parser. It keeps the pending first key of a
// two-key sequence and the time it was pressed. The zero value is not usable;
// create it with NewKeys.
type Keys struct {
	now        func() time.Time
	pending    rune // 0 when no key is pending
	pendingCtx KeyContext
	since      time.Time
	queue      []Result
}

// NewKeys returns a parser that reads the time from now. A nil now uses
// time.Now.
func NewKeys(now func() time.Time) *Keys {
	if now == nil {
		now = time.Now
	}
	return &Keys{now: now}
}

// Feed handles one typed rune in the given context. It returns the first command
// that the key produced, and whether the key was consumed. Results beyond the
// first, if the key also completed a pending key, are returned by Drain.
func (k *Keys) Feed(ctx KeyContext, r rune) (cmd string, consumed bool) {
	res := k.step(ctx, r)
	k.queue = res[1:]
	return res[0].Cmd, res[0].Consumed
}

// Drain returns the results that the last Feed or FeedEnter produced after its
// first one, in order, and clears them. It returns nil when there are none.
func (k *Keys) Drain() []Result {
	out := k.queue
	k.queue = nil
	return out
}

// FeedEnter handles the Enter key, which is not a rune. Enter drops a pending
// key without firing its single. This departs from the flush rule for runes: a
// lone `d` followed by Enter must not open the date field in Sort.
func (k *Keys) FeedEnter(ctx KeyContext) string {
	k.Reset()
	return enterCmds[ctx]
}

// Expire fires a pending key whose timeout has passed without a second key. It
// returns the single of that key, for example focus.date for a lone `d`. It is
// meant for a timer in the shell, so that a lone `d` does not wait for the next
// key. It returns false when nothing is pending or the timeout has not passed.
func (k *Keys) Expire() (cmd string, ok bool) {
	if k.pending == 0 || k.now().Sub(k.since) < SequenceTimeout {
		return "", false
	}
	p, pctx := k.pending, k.pendingCtx
	k.pending = 0
	if cmd, ok := keymaps[pctx].singles[p]; ok {
		return cmd, true
	}
	return "", false
}

// Reset drops any pending key and any queued results, without firing them. Call
// it on a mode change or when the focus moves away from the grid or queue.
func (k *Keys) Reset() {
	k.pending = 0
	k.queue = nil
}

// step resolves one rune and returns one or two results. Two results appear
// only when a pending key fails or times out and its single fires first.
func (k *Keys) step(ctx KeyContext, r rune) []Result {
	km := keymaps[ctx]
	var out []Result

	if p := k.pending; p != 0 {
		pctx, since := k.pendingCtx, k.since
		k.pending = 0
		// A pending key from another context is dropped without firing.
		if pctx == ctx {
			if k.now().Sub(since) < SequenceTimeout {
				if cmd, ok := km.seqs[string([]rune{p, r})]; ok {
					return []Result{{Cmd: cmd, Consumed: true}}
				}
			}
			if cmd, ok := km.singles[p]; ok {
				out = append(out, Result{Cmd: cmd, Consumed: true})
			}
		}
	}

	if km.isPrefix(r) {
		k.pending = r
		k.pendingCtx = ctx
		k.since = k.now()
		return append(out, Result{Consumed: true})
	}
	if cmd, ok := km.singles[r]; ok {
		return append(out, Result{Cmd: cmd, Consumed: true})
	}
	return append(out, Result{})
}

// isPrefix reports whether r starts a two-rune sequence.
func (km keymap) isPrefix(r rune) bool {
	for seq := range km.seqs {
		if []rune(seq)[0] == r {
			return true
		}
	}
	return false
}

// KeyBinding is one normal mode key (or two-key sequence) and its command id.
type KeyBinding struct {
	Keys string // for example "gg", "j" or "Enter"
	Cmd  string // for example "first"
}

// KeyBindings lists the normal mode bindings of ctx, sorted by Keys. It reads
// the same tables as the parser, so it cannot drift from it.
func KeyBindings(ctx KeyContext) []KeyBinding {
	km := keymaps[ctx]
	var out []KeyBinding
	for r, cmd := range km.singles {
		out = append(out, KeyBinding{Keys: string(r), Cmd: cmd})
	}
	for seq, cmd := range km.seqs {
		out = append(out, KeyBinding{Keys: seq, Cmd: cmd})
	}
	if cmd := enterCmds[ctx]; cmd != "" {
		out = append(out, KeyBinding{Keys: "Enter", Cmd: cmd})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Keys < out[j].Keys })
	return out
}

// keyDescriptions are English fallback descriptions of the vim commands, used
// when the catalog has no text for them.
var keyDescriptions = map[string]string{
	"next": "Next document", "prev": "Previous document", "first": "First document", "last": "Last document",
	"page.next": "Next page", "page.prev": "Previous page",
	"zoom.in": "Zoom in", "zoom.out": "Zoom out", "zoom.fit": "Fit to window",
	"focus.type": "Edit type", "focus.date": "Edit date", "focus.first": "Edit first empty field",
	"reveal": "Show in Finder", "undo": "Undo last filing", "copy.abs": "Copy absolute path",
	"copy.rel": "Copy relative path", "remove": "Remove from queue", "add": "Add files",
	"search": "Search", "palette": "Command palette", "shortcuts": "Show shortcuts",
	"row.down": "Row down", "row.up": "Row up", "row.last": "Last row", "row.first": "First row",
	"hit.next": "Next search hit", "hit.prev": "Previous search hit", "select": "Select row",
	"select.rows": "Select rows", "resort": "Sort again", "open": "Open document",
}

// KeyDescription returns the English fallback description of a vim command id,
// or the id itself when there is none.
func KeyDescription(cmd string) string {
	if d, ok := keyDescriptions[cmd]; ok {
		return d
	}
	return cmd
}
