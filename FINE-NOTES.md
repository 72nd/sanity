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

- `grid.Column.Options` (drop-down cell) now exists. The rules grid uses it for Kind, Mode, Strength and Where (`dropdown(...)` in `ui/types_view.go`).
- (Corrected) `widgets.ShowError`, `ShowInfo`, `ShowCustom` and `ShowForm` do exist; the Types agent missed them and used toasts for failures.
- `grid.Grid` has a minimum height of about 4 rows. sanity wraps it in a small `minHeight` layout (`ui/layout.go`); a `widgets.MinHeight` helper in fine would be cleaner.
- `domain` has no sentinel error for a template that escapes the root (`..`); the presenter matches the error text.
- `widgets.Split` had no per-side minimum sizes and the leading side won when space ran short (fixed: `SetMinLeading`/`SetMinTrailing`).

- Import dialog (types): `widgets.ShowCustom` has one close button and no way to add action buttons, so Save and Save and edit sit inside the content. A `ShowDialog(win, title, content, buttons...)` would be cleaner. `widgets.Label` does not wrap, so long finding messages run out of the dialog.

## 2026-11-23 (v0.2.0, wave 1)

- Fyne's `Entry` word/line navigation (Alt/Ctrl/Cmd+arrows, word delete) are `desktop.CustomShortcut`s. The kit's `Entry.TypedShortcut` forwarded every custom shortcut to the canvas, so they never reached the field (fixed for `Entry`/`TextArea` via `textnav.go`). `numberField` and `comboEntry` in `widgets/shortcut.go` still forward them. The test canvas has no shortcut sink, so tests do not show this; only the real window does.
- `selectToLineEdge` (Cmd+Shift+Left/Right) replays Fyne key handling because the selection fields are private. A held Shift can reset `selectKeyDown`, so a directly following Shift+arrow selection needs a new Shift press.
- `LabelMono` is text-coloured. A muted mono style (`LabelMonoMuted`) is missing; the palette draws `canvas.Text` directly in `shell`.
- The shell rebuilds the `StatusBar` in `build()` (language change, `SetPages`). Use `Shell.SetFooterTrailing`; a direct `Status.SetTrailing` replaces the version label until the next `build()`.
- `StatusBar.SetTrailing` has no overflow protection: below the sum of the minimum sizes the `SetRight` texts overlap the trailing objects.
- Fyne has no height-for-width layout: `Label.MinSize().Height` depends on its current width. `ShowDialog` sets the content to its final width before measuring; a wrapping label in an `HBox` never wraps.
- `widgets.ShowDialog` has no height limit. A big text area inside it has to scroll by itself.
- The menu inset (`menuInset = 4`) is a size literal without a theme token.
- `date.candidates.hint` was extended with Ctrl+J/K; the `DateEntry` highlight is not refreshed by a calendar pick until the next key.
- The `docs/screenshots/demo_invoices_contextmenu_*` screenshot shows the old menu padding; regenerate before the next release.

## 2026-11-23 (v0.2.0, sanity findings)

- Undo (`Undo`, `UndoBatch`) of a filing forgets the origin because `File` drops it when it removes the item from the queue; the document is then in the inbox but cannot be put back.
- Origins are absolute paths: after moving the project folder "put back" targets the old directory and `AdoptInPlace` refuses such documents.
- `fmt.Sprintf("%d.%02d", c/100, c%100)` is duplicated in `inbox/corrections.go` (`FormatAmount`), `ui/presenter/suggest.go` and `ui/presenter/sort_batch.go`; negative cents below 100 print garbage (`-50` becomes `0.-50`). `ParseAmount` (`library_ops.go`) does not know no-break-space grouping.
- The leading dot in `.39000.00 EUR` could not be reproduced in `analysis.FindAmount`; needs the original line as synthetic text.
