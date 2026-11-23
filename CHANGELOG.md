# Changelog

## v0.1.0 – Erste nutzbare Version

- FEAT: PDFKit/Vision engine, scan detection and white-margin crop
- FEAT: Recognition, inbox queue, move with undo, learning path with text cache and learner
- FEAT: Sort screen with picker, suggestions, per-file view, multi-page preview, Vim layer, drop and file actions
- FEAT: Types editor with live path preview, rules grid and date settings; learn rules from the current document
- FEAT: Project page, recent projects and runtime project switching
- FEAT: Library page, batch filing, shortcut overview, types import, previews, Finder paste
- FEAT: Free text fields, duplicate link to the library, correction log
- FEAT: Type check (CheckType), snippet parser, LLM prompt and CLI check
- FEAT: Document cache, batch undo, open in viewer
- FEAT: Date stamp app icon and `task icon` / `task package`
- FIX: Dropped files reach the sort page, removed entries are forgotten
- FIX: Paste command wiring, rules grid height, deterministic UI tests
- FIX: OCR default scale 1.5 (measured: lower scales recognise more); layer threshold `minLayerChars` 40
- DOCS: README and user guide
- CHORE: Changelog rules, keep local scan samples out of git

## v0.2.0
- FEAT: Imported documents (drop, file dialog, re-sort) are moved to `office/inbox/`; the origin is kept in `.origins.json` and the inbox survives a restart
- FEAT: Sort: "Put back" (Cmd+Shift+B, Vim `p`) moves a document to where it came from (also cancels a re-sort); "Adopt in place" (Cmd+Shift+A, Vim `a`) adds a document to the library without renaming; delete button
- FEAT: Sort: Cmd++ / Cmd+− zoom, Enter always goes to the next field, new types appear without restart, imports run in the background and report errors
- FEAT: Welcome page at start without argument (icon, recent projects, new, open); project drop-down in the toolbar; version and shortcut button in the footer
- FEAT: Version constant in `version.go`, checked against `FyneApp.toml` and the last changelog heading
- FIX: Copied relative paths (library and sort) are relative to the `office` folder (`../receipts/2026/x.pdf`); path template and stored paths stay project-relative
- FIX: Amount detection reads thousands separators written as no-break, narrow no-break or thin space as one amount
- STYLE: Types settings: date format legend one line per year/month/day, "Choose folder" next to the path preview, Source and Maximum age side by side in wide windows, monospace path fields
- STYLE: Type import dialog: save buttons in the dialog footer, tables scroll sideways, warnings wrap
- STYLE: Shortcut overview in a 720 px dialog with two columns, opened from a footer button
- CHORE: Icon variants for v0.2 in `prj/icons/` (selection pending)
