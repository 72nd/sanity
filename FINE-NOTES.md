# Notes for fine

Gaps, bugs and surprises found while building sanity. Newest first.

## 2026-11-22 (F1 ImageView, F2 DateEntry)

- `ImageView.Zoom()` is relative to the fit scale (1.0 = fitted), steps x1.25, range 0.1 to 16.
- `ImageView.ScrollHalfPage(down)` returns false at the end, so the sort screen can switch pages.
- Fyne `KeyEvent` carries no modifier state: Shift/Alt are tracked by hand from KeyDown/KeyUp.
- Fyne 2.8.2 Entry maps neither Ctrl+A nor Ctrl+X on macOS (Cmd is the shortcut modifier), so DateEntry takes them for +/-1 day. On Windows/Linux they stay select-all/cut. Checked in the source, not on hardware.
- Alt+Shift+digit on macOS may also arrive as a rune; `dateField.TypedRune` drops runes while Alt is held. Untested on a real Mac.
- A subclassed Entry does not get `Wrapping = TextTruncate` from `widget.NewEntry`; a wrapping VBox around a field shifts compact goldens by 1px.
- `GALLERY_UPDATE=1` regenerates all goldens; revert unrelated diffs.
- Worktree isolation of the Agent tool uses the repo of the working directory (sanity), not fine, so fine agents edit fine directly.
