# Notes for fine

Gaps, bugs and surprises found while building sanity. Newest first.

## 2026-11-23 (grid latency)

- (Fixed in fine) Sorting and the filter funnel reacted only after 0.5 to 0.8 s with 20 rows. Not CPU: a profile of the real app showed 2 s CPU in 40 s. Fyne delays `Tapped` by the system double-click interval (`DoubleTapDelay`, 0.5 s on macOS by default) on every widget that also implements `DoubleTapped`, and `Grid` does (open a row). Header clicks now act in `MouseUp`; `Tapped` only covers drivers without mouse events. Any kit widget with both interfaces has the same trap.
- Sorting the amount column parsed every amount on every comparison (about 50 to 170 ms for 3,000 rows); `presenter.Row` now caches the parsed amount from `Load`.
- `SANITY_CPUPROFILE=<file>` (optional `SANITY_CPUPROFILE_SECS`, default 30) records a CPU profile of the running app (`profile.go`).

## 2026-11-23 (Sort side panel)

- (Fixed in fine) `widgets.Picker`: after `SetItems` or a re-rank (typing, `Preselect`) some rows kept the width of an earlier layout (rows 3 to 5 were 148 px wide in a 294 px list), so labels were truncated. `pickerRenderer.Layout` now resizes every row to the box width (`fitRows`).
- No vertical gap helper: the batch plan separates its groups with a `canvas.Rectangle` in the surface color sized by `SizePagePad`; a `widgets.Gap(token)` would be cleaner.
- `widgets.Label` has no ellipsis truncation (`truncateText` is unexported), so the document name in the panel header wraps instead.
- A wrapping Label cannot break a long path before its first character: a leading "→ " ended up alone on a line, so the target path is shown without the arrow.
- (Fixed in fine) Toasts used to overlay the bottom right, covering the pinned footer actions of a right-hand panel. `ToastHost` now stacks in the bottom-left over the status bar.

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

## 2026-11-23 (v0.2.0, wave 2)

- `wrapPair` (`ui/layout.go`): a Fyne `Layout` does not know its width in `MinSize`, so it remembers the mode of the last `Layout` call. A generic `widgets.ResponsivePair` would be cleaner. The break point 560 is an estimate.
- No `widgets.HScroll`; `widgets.Scroll` scrolls both ways and fits when the height is set by the caller.
- `DialogButton` has no `Disabled`/`SetEnabled` (workaround: `KeepOpen` and a guard in the callback); `widgets.Dialog` exposes no footer buttons for tests.
- `shell.New` builds the first page immediately, before `sh.Prefs` exists (Recents on the welcome page). A config hook or a `Prefs` instance before `New` would be cleaner.
- `widgets.Select` has no icon and no separator; "Open project…" and "Overview" are plain options. `ShowFolderOpen` cannot create a folder.
- Fyne gives no modifiers in `KeyEvent`: Shift+Enter arrives as `TypedKey(Return)` at the focused widget. Solved with `Entry.OnSubmitBack`, `DateEntry.OnSubmitBack`, `Picker.OnCommitBack` (fields count Shift via KeyDown/KeyUp); untested with the real GLFW key order.
- A `Command` has one shortcut; Cmd++ needs three commands (`+`, `=`, Shift+`=`) that show up several times in the palette and the shortcut dialog. `Command.Aliases` or a `Hidden` flag would be cleaner.
- `Sort.LoadTypes` writes `p.types` from a goroutine (latent race, as at the first load).
- After "Overview" only the welcome page commands are active; Library/Sort shortcuts are not checked for nil safety without a project.

## 2026-11-23 (v0.3.0 filters)

Library page now uses the grid's value filters, conditions, multi-key sort and grouping.

- Fyne 2.8.2's file dialog has no multi-select. fine therefore got `widgets.ShowFilesOpen` via zenity (native, non-modal); Sort -> "Add files" uses it.
- `ItemAt(v)` returns nil for group header rows and `Len()` counts them. Every loop over `0..Len()` needs a nil check; a `ForEachRow` or `RowAt(v) (T, bool)` helper would be safer. sanity builds a view-aligned slice with zero Rows as headers (`viewRows`) and `NextHit` skips rows without an entry id.
- No getter for the active (keyboard) column. The "Filter column" command opens the picker by replaying Alt+Down (`KeyDown(AltLeft)`, `TypedKey(Down)`, `KeyUp`). A public `ActiveColumn()` or `OpenFilterActive()` would remove the hack.
- `SelectView(v)` / vim movement may land on a group header; selecting a header selects nothing, so actions must ignore it (`selection()` returns no rows there).
- The conditions button label needs the number of active conditions: `len(g.Conditions().Items)` also counts rows the dialog keeps but the grid dropped, if any; there is no `ConditionCount()`.
- `OnViewChanged` fires for every `SetItems`, also during page build before widgets that the callback touches exist; define the callback after those widgets.
- Date columns: the value picker lists the formatted `Value` text sorted by `Less`, so a date picker shows "03.04.2026" entries, not year/month. Groupings cover that; a picker with date ranges would be better.
- `Aggregate` receives the items of the whole group at all levels; no way to show a count next to the label of the header (only the aggregate cell per column).
- `ShowConditions` operates on the grid's own columns; conditions on `Kind` number ignore the currency (sanity's amount column compares cents/100 only).

