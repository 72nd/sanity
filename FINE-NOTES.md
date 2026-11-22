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

## 2026-11-22 (test driver)

- The Fyne test driver runs `fyne.Do` on the calling goroutine. A page that loads in a goroutine and then calls `fyne.Do` therefore lays out concurrently with the test goroutine (`go test -race` reports it; about 1 in 40 runs of `ui.TestLibraryPageShowsRows` panics in `grid.updateHeader`). Production is not affected. `examples/starter` has the same pattern.
- Fyne's own `ColorNameHover`/`ColorNamePressed` are painted as overlays on widgets (for example the primary "Open" button of the file dialog); the kit's opaque `Hover` token hid the button. Fixed in `theme.Color` (translucent text color).
- Fyne passes no modifier keys with scroll events, so `ImageView` has no Ctrl+wheel zoom or pinch; it has buttons, keys and double click.

## 2026-11-22 (Types editor)

- `grid.Column.Options` (drop-down cell) now exists. The rules grid uses it for Kind; mode, strength and where are still typed text and can switch over with `dropdown(...)` in `ui/types_view.go`.
- (Corrected) `widgets.ShowError`, `ShowInfo`, `ShowCustom` and `ShowForm` do exist; the Types agent missed them and used toasts for failures.
- `grid.Grid` has a minimum height of about 4 rows. sanity wraps it in a small `minHeight` layout (`ui/layout.go`); a `widgets.MinHeight` helper in fine would be cleaner.
- `domain` has no sentinel error for a template that escapes the root (`..`); the presenter matches the error text.
- `widgets.Split` had no per-side minimum sizes and the leading side won when space ran short (fixed: `SetMinLeading`/`SetMinTrailing`).
