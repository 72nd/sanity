# sanity: implementation plan

Status: agreed (2026-11-22). Source: `../prj/draft.md`. User decisions are recorded in "Decisions" at the end.

Guiding rule from the draft: **every decision serves filing speed.** The target is that a well-known
receipt is filed in a single key press (`Cmd+Enter`). An unknown one should take about five keys:
type a few letters of the type, `Enter`, type the date as `1203`, `Enter`, `Cmd+Enter`.

## 1. Concepts

| Term | Meaning |
|---|---|
| Project | One company's accounting. It has a root folder, a name and an inbox at `<root>/office/inbox/`. Its config lives in `<root>/office/sanity.json`. |
| Document type (DT) | A name, a path template, recognition rules and date settings. |
| Inbox item | An imported PDF waiting to be filed. It carries the analysis result (text, suggestions). |
| Library entry | A filed document: target path, type, date, extracted fields, hashes and a fingerprint. Over time the library becomes the document library and the learning base. |

### Path template

The path is relative to the project root. Placeholders are in braces. A date takes a Go layout after a colon,
because the draft allows Go syntax and it is the most flexible:

```
receipts/{date:2006}/telecom/{date:2006-01-02}_telekom_{invoice}.pdf
```

| Placeholder | Value |
|---|---|
| `{date:<layout>}` | document date in a Go layout (`2006` year, `01` month, `02` day, `Jan` month name) |
| `{type}` | DT name, made safe for a file name |
| `{invoice}` | detected invoice number |
| `{amount}` | detected total, for example `49.95` |
| `{original}` | original file name without `.pdf` |
| `{text:<label>}` | free text that the sort form asks for |

- The editor shows a short legend for the Go layout tokens.
- The editor always shows the evaluated path for an example date (and the example values) live. It turns red when the
  template is invalid or escapes the root (`..`).
- A "Choose folder" button opens the folder picker in the project root. It inserts the relative folder and keeps the
  file name part.
- Collisions get a suffix (`_2`, `_3`). The file is never overwritten.
- A placeholder that has no value (no invoice number found) becomes a required field in the sort form.

## 2. UI and keyboard flow

### Pages (shell)

1. **Library** (`Cmd+1`): the start page, always. It is a grid of all filed documents. When the inbox is not empty it
   shows a hint with the count, and `Enter` on the hint opens Sort.
2. **Sort** (`Cmd+2`): the work screen. Its sidebar title shows the inbox count.
3. **Types** (`Cmd+3`): the DT editor.
4. **Project**: root, inbox and own identifiers (IBAN and VAT ID, ignored by the learning). There is also a project
   switcher (recent projects in prefs, `Cmd+Shift+P`).
5. The shell settings page and the command palette (`Cmd+K`) come from fine.

### Sort screen (core)

```
┌ Queue (narrow) ┬ Preview (wide) ─────────────────┬ Form ─────────────────────┐
│ ● telekom.pdf  │                                 │ Type   [tel____________]  │
│ ○ scan_004.pdf │      rendered PDF page          │  1 Telekom Invoice  0.92 ✓│
│ ◌ analysing…   │      (fit width, zoom,          │  2 Telekom Mobile   0.41  │
│                │       detected date and         │  3 Vodafone         0.12  │
│                │       invoice no. highlighted)  │ Date   [12.03.2025]       │
│                │                                 │  ⌥1 12.03.25 Invoice date │
│                │                                 │  ⌥2 26.03.25 due date     │
│                │                                 │ Invoice [R-88123]         │
│                │                                 │ → receipts/2025/telecom/… │
│ 3 open · 12 today                                │ [File  ⌘↩]  [Skip  ⌘]]   │
└────────────────┴─────────────────────────────────┴───────────────────────────┘
```

**Focus flow.**
- The type field is focused when a document opens.
- `Enter` accepts the top match and moves to the date. `Enter` moves on to the next empty field.
  On the last field `Enter` does nothing except show the hint "`Cmd+Enter` to file".
- **Only `Cmd+Enter` files a document** (user decision: a plain `Enter` is pressed too quickly). It works at any time when
  everything is valid. Neither the form nor the Vim normal mode files on `Enter`.
- After filing, the next document opens at once. Its type field is focused and the next preview is pre-rendered.

**Type picker.**
- The list stays visible in the form, not in a popup.
- Ranking:
  - With an empty search: the detection score, then MRU (recently used types).
  - When typing: fuzzy (subsequence, word starts), then the score.
- `Up` and `Down` move through the list, and `Alt+1..9` picks directly.
- Confident suggestions (score ≥ 0.8 and a clear lead) are preselected and marked with a status badge. Status is never shown by colour alone.

**Date field.** Every input is interpreted as fast as possible:
- `1203` is read as 12.03. with the year inferred: the most recent date that is not in the future.
- `12.3`, `12.3.25` and `120325` are understood.
- `t` is today and `y` is yesterday.
- `Up`/`Down` step ±1 day and `Shift+Up`/`Down` step ±1 month.
  F2 overrides the Entry behaviour for this single-line field, which is safe because it has no line movement.
- Detected candidates are listed under the field with their label and can be picked with `Alt+Shift+1..3`.

**Keyboard map.** The focus usually sits in a text field. Fyne's `widget.Entry` already handles these keys:
- `Cmd+Z`, `Cmd+Shift+Z`, `Cmd+C`, `Cmd+V`, `Cmd+X`, `Cmd+A`;
- `Cmd+Left`/`Right` (line start and end), `Alt+Left`/`Right` (word), `Up`/`Down`, `Backspace`
  (`fyne.io/fyne/v2@v2.8.2/widget/entry.go` around lines 1056 to 1135).

App shortcuts must therefore avoid those keys. Every shortcut is also a shell `Command`, so it appears in the palette (`Cmd+K`).

| Key | Where | Action |
|---|---|---|
| `Cmd+1` / `Cmd+2` / `Cmd+3` | everywhere | Library / Sort / Types (shell page shortcuts) |
| `Cmd+Enter` | Sort | file the document (moves it) and open the next one |
| `Enter` | Sort form | accept the field and go to the next one; **never files** |
| `Alt+1..9` | Sort, type picker | pick a type |
| `Alt+Shift+1..3` | Sort, date | pick a date candidate |
| `Cmd+]` / `Cmd+[` | Sort | next / previous document (skip) |
| `Cmd+Shift+Backspace` | Sort | remove the document from the inbox (confirmation; the file is moved to the macOS trash) |
| `Cmd+Alt+Z` | everywhere | **undo the last filing** (`Cmd+Z` stays text undo) |
| `Cmd+Shift+C` | Sort, Library | **copy the relative path** (see below) |
| `Cmd+Alt+C` | Sort, Library | copy the absolute path |
| `Cmd+O` | everywhere | add files |
| `Cmd+Shift+V` | everywhere | import PDFs copied in Finder (`Cmd+V` stays text paste) |
| `Cmd+F` | Sort, Library | search: in Sort it focuses the type search, in Library the quick filter |
| `Cmd+E` | Library | re-sort the selected documents |
| `Cmd+=` / `Cmd+-` / `Cmd+0` | Sort | preview zoom in / out / fit width |
| `Cmd+Down` / `Cmd+Up` | Sort | next / previous preview page, if no entry claims them (to be checked in P3) |
| `Cmd+R` | Sort, Library | show in Finder (Sort: the current inbox file; Library: the selected document) |

- **Copy relative path** (`Cmd+Shift+C`):
  - In Sort, it copies the evaluated target path of the current document, relative to the project root, for example
    `receipts/2025/telecom/2025-03-12_telekom_R-88123.pdf`. It works before filing, as soon as the template can be evaluated.
    Right after filing it copies the path of the document just filed, so filing followed by pasting into the accounting software is
    possible.
  - In Library, it copies the paths of the selected rows, one per line.
  - It uses `App.Clipboard().SetContent`, which handles text, and confirms with a toast.
  - Paths use `/` and the collision suffix that was actually applied.
- **Shortcut overview (P6):** `Cmd+/` (and `?` in Vim normal mode) opens a dialog listing all shortcuts, grouped by page and
  by the Cmd and Vim layers, with a filter field. It is also a shell `Command`, so it appears in the palette. The list is built
  from the registered commands and the key parser table in `ui/presenter/keys.go`, not typed by hand, so it cannot drift from
  the real bindings. Vim entries are hidden when `sanity.vim` is off. A presenter test checks that every registered shortcut
  appears in the list.
- **P3 task:** every shortcut gets a test that checks it does not collide with a focused `Entry`, `ComboBox`/Picker or `DateEntry`.
  Collisions found go into `FINE-NOTES.md`.

### Vim layer (the user uses Vim)

The user works with Vim, so the app gets a modal layer **in addition to** the Cmd shortcuts above. It is on by default and
can be switched off with the pref `sanity.vim` on the settings page.

**Modes.**
- *Insert*: a text field has the focus, and all letters are text. This is the default when a document opens, so typing a
  type name starts at once.
- *Normal*: `Esc` leaves the field. Nothing is focused, or the grid or the queue is, so letters are commands.
  `i` or `t` goes back into the form.
- The mode is shown in the status bar (`NORMAL` / `INSERT`) as text, not only as colour.

**Keys while typing (insert mode).** These are fzf/Telescope habits:
- `Ctrl+J`/`Ctrl+K` and `Ctrl+N`/`Ctrl+P` move through the type list without leaving the search field.
- In the date field, `Ctrl+A`/`Ctrl+X` step ±1 day, as in Vim's increment and decrement. `Up`/`Down` stay as well.
  **Check in P3** that Fyne's Entry on macOS does not map `Ctrl+A`/`Ctrl+X` itself (emacs-style line start). If it
  does, F2 takes the keys over.

**Normal mode, Sort.**

| Key | Action |
|---|---|
| `j` / `k` | next / previous document in the queue |
| `gg` / `G` | first / last document |
| `h` / `l` | previous / next preview page |
| `Ctrl+D` / `Ctrl+U` | scroll the preview half a page down / up |
| `+` / `-` / `=` | zoom in / out / fit width |
| `t` / `d` / `i` | focus the type / date / first empty field (insert mode) |
| `x` | show the current file in Finder (as in Zed) |
| `u` | undo the last filing |
| `yy` / `Y` | copy the relative / absolute path |
| `dd` | remove from the inbox (with confirmation) |
| `o` | add files |
| `/` | focus the type search |
| `:` | open the command palette |

**Normal mode, Library (the grid has the focus).**

| Key | Action |
|---|---|
| `j` / `k` / `gg` / `G` / `Ctrl+D` / `Ctrl+U` | row movement |
| `/` | quick filter, then `Enter` back to the grid |
| `n` / `N` | next / previous filter hit |
| `v` | start or extend a selection (`V` selects rows) |
| `yy` | copy the relative paths of the selection |
| `Enter` | open the document |
| `x` | show in Finder (as in Zed) |
| `r` | re-sort |

- **Filing stays `Cmd+Enter` in every mode.** There is no Vim key and no plain `Enter` for it.
- **Every frequent action has a Cmd combination as well as its Vim key**, so it works from insert mode without `Esc`:

  | Action | Vim (normal) | Cmd (always) |
  |---|---|---|
  | Search (type search / quick filter) | `/` | `Cmd+F` |
  | Copy relative / absolute path | `yy` / `Y` | `Cmd+Shift+C` / `Cmd+Alt+C` |
  | Show in Finder | `x` | `Cmd+R` |
  | Undo the last filing | `u` | `Cmd+Alt+Z` |
  | Remove from the inbox | `dd` | `Cmd+Shift+Backspace` |
  | Re-sort (Library) | `r` | `Cmd+E` |
  | File | none | `Cmd+Enter` |

- Multi-key sequences (`gg`, `yy`, `dd`) time out after 800 ms.
- A small parser `ui/presenter/keys.go` is fed runes and returns command ids. It is headless and testable, and the same
  ids are shell `Command`s.

**Technical background (Fyne 2.8.2, checked).**
- `widget.Entry` does not handle `Esc`. A focused widget receives every key, and `Canvas.OnTypedRune` only fires
  when nothing is focused (`internal/driver/glfw/window.go` around line 798).
- So: the kit text fields need `Esc`, which unfocuses (F8), and normal mode then runs over `Canvas.SetOnTypedRune`/`SetOnTypedKey`.
- `Grid.TypedRune` is empty (`widgets/grid/input.go:355`), so the grid swallows letters. It needs a rune hook (F7).

- There are no confirmation dialogs while filing. A toast shows the target path and how to undo. Undo is faster than asking every time.
- A duplicate (same hash, or same invoice number and type) shows a warning banner with a link to the existing entry. It does not block filing.

### Adding documents

The draft asks for several simple ways in, in order of value:

1. **The watched inbox folder** `<root>/office/inbox/` (fsnotify). Save the PDF there and it appears in the queue without any further step.
2. **Drag and drop** onto the window. Folders are imported recursively.
3. `Cmd+O` and the toolbar button.
4. `Cmd+Shift+V` imports files copied in Finder. Fyne only supports text on the clipboard, so this needs a small
   NSPasteboard helper.

**Files are never copied, always moved.** Whatever the source (inbox, drop, dialog), filing moves the original to the
target path:
- On the same volume this is `os.Rename`.
- Across volumes it is copy, fsync, compare the hash, then delete the source. The result is still a move.
- Import does not touch the file. It stays where it is until it is filed.
- Undo moves the file back to its original path, and recreates the folder if needed.
- Folders that filing created and that are empty after an undo are removed.

### Library

- A grid with these columns: date, type, file name (relative path), invoice no., amount, filed at.
- A quick filter field above the grid (`Cmd+F`) filters in the presenter.
- `Enter` opens the document. The context menu offers: copy relative path, show in Finder, re-sort (back to the sort screen,
  which moves and renames on filing) and remove from the library.
- A status bar shows the count and the total of the selection.

### Types editor

- A list of DTs on the left and the detail on the right (HSplit).
- The detail has cards for:
  - Basics: name and path template with the live preview and the folder button.
  - Recognition: a rule grid with kind, value, mode (should/must/not), strength (weak/normal/strong) and on/off.
  - Date: preferred labels, ignored labels, a period and a maximum age.
- The sort screen has an extra action, "Learn rules from this document", which suggests distinctive identifiers
  as checkboxes. IBAN, VAT ID and domain are ticked by default.
- **P6 additions (LLM-assisted setup, see section 3a):** the list gets an "Import type…" button, a "Copy as JSON" action
  per type and a "Copy LLM prompt" button. The import opens a review dialog, described in section 3a.

## 3. Recognition (local only)

Research result: **everything runs locally through macOS frameworks via cgo**, with no Homebrew dependency.

The proof of concept (`research/_poc/`, see section 8) produced these results:

| Step | Method | Result |
|---|---|---|
| Rendering | CoreGraphics and PDFKit | about 19 ms per page at 2x |
| Text layer | PDFKit `string` | about 8 ms; umlauts and dates are correct |
| OCR | Vision `VNRecognizeTextRequest` (accurate, `de-DE`, `en-US`) | **Verified by the user outside the sandbox (P0 done).** Inside the agent sandbox it returns no text, so OCR tests must skip there. |
| Date helper | `NSDataDetector` | Recognises `12. März 2025` and `12.03.25`, but not `Dezember 2024` or `31.12.`. Use it as a second opinion only. |

- Ruled out: go-fitz (AGPL), Tesseract (Homebrew, weaker), `RecognizeDocumentsRequest` (needs macOS 26).

**Pipeline per document.** It runs in a goroutine and its results come back to the UI through `fyne.Do`.
1. SHA-256 of the file gives an exact duplicate check.
2. **Text source.** About 50 % of the documents are scans, many from Paperless-ngx with an embedded Tesseract text layer
   of poor quality. A present text layer therefore does not mean good text. Each page is classified:
   - *Born digital*: the text layer is used as is.
   - *Scan*: the page is rendered at about 250 dpi and run through Vision OCR (accurate). The Vision text replaces the embedded text for the analysis.
   - A page counts as a scan when one of these holds:
     - the text layer has fewer than about 20 characters;
     - the PDF Producer/Creator names OCRmyPDF, Tesseract or Paperless;
     - a page-sized image dominates the page;
     - the text quality is low (share of non-letter garbage, share of known words).
   - The embedded text stays as a fallback. The PDF file itself is not changed (the move-only rule).
   - OCR runs in the background worker and the preview does not wait for it. The form is filled when the analysis is done.
3. The text is normalised (NBSP, OCR repairs such as O to 0 in digit groups) and a hash of the normalised text is computed.
4. **Date**: regex families plus NSDataDetector. Documents come from DE, CH and the US:
   - DMY with dots (`12.03.2025`, `12.03.25`), ISO, and German month names.
   - US forms: `March 12, 2025`, `Mar 12 2025`, and `03/12/2025` read as MDY.
   - Slashes are ambiguous. MDY is the default; DMY applies when the day is above 12 or when the document language is German.
     The language is decided from stop words in the text.
   - Scoring: a label within 40 characters (Rechnungsdatum +, Fälligkeit −), the type's preferred labels, position on the page,
     a four-digit year, repetition, and plausibility (not in the future, not too old).
   - Fallbacks: PDF CreationDate, then a date in the file name, then mtime. A fallback is never marked as a confident suggestion.
5. **Type**: `final = noisyOR(rules, learned)`.
   - Rules: `1 - Π(1-w)`. All `must` rules have to match, and a `not` rule excludes the type.
   - Learned: the letterhead fingerprint (first 15 and last 5 lines, token set plus bigrams, Jaccard, the maximum per type), later TF-IDF kNN (k=3).
     Both are pure Go, with no library.
   - While a type has fewer than 3 filed documents, the learned score is capped at 0.7, so one example never gives a one-key suggestion without a rule.
   - The project's own identifiers (own IBAN, VAT ID, address) are filtered out. Otherwise they are the most common false positive.
6. **Fields**: invoice number and total amount (label regex; per-DT override). Amounts are read in the
   formats `1.234,56` (DE), `1'234.56` (CH) and `1,234.56` (US), with the currencies EUR, CHF and USD/$.
   Labels include the English ones: Invoice #, Total, Amount due, Balance due.
7. Every correction by the user (a suggestion overridden) is logged, so the thresholds can be tuned later.

Details and regexes: the research notes in section 8.

## 3a. LLM-assisted type setup (P6)

**Idea.** Setting up a new document type by hand (rules, weights, path template, date labels) is a one-off chore. A LLM
can draft it from one or two redacted sample receipts. The LLM is used **only at configuration time**, never while
sorting: classification stays local, deterministic and offline (section 3). The LLM produces a `DocumentType` as JSON, and
sanity validates and dry-runs it before anything is saved. The user always has the last word.

**Why it fits.**
- The output is exactly one `domain.DocumentType`, a flat JSON object that goes into `types[]` of `office/sanity.json`.
- The checks exist already: `Template.Validate`, regex compilation, `EvalRules` and `Classify` (section 3).
- It closes the cold-start gap. A new type has no filed documents, so the learned score is capped at 0.7
  (`minFiledForFull = 3`). A good rule set is what gives the first one-key suggestion.

**Flow.**
1. The user redacts a sample PDF (own IBAN, address, amounts) and gives it to an agent (Claude Code skill, or any chat with the
   copied prompt). Input is the document text; scans need OCR or a vision model first.
2. The agent returns one JSON snippet (format below).
3. The user pastes it in **Types → Import type…** (or runs the CLI check below). sanity validates, dry-runs and shows a
   review dialog. Only after "Save" is the type stored via `Store.SaveType`.

**Snippet format.** A bare `DocumentType` is accepted. The recommended form is an envelope with notes that are not stored:

```json
{
  "type": {
    "id": "hetzner-invoice",
    "name": "Hetzner Invoice",
    "pathTemplate": "receipts/{date:2006}/hosting/{date:2006-01-02}_hetzner_{invoice}.pdf",
    "rules": [
      {"kind": "domain",  "value": "hetzner.com", "mode": "should", "weight": 0.9, "where": "header", "enabled": true},
      {"kind": "vatId",   "value": "DE812871812", "mode": "must",   "weight": 0.9, "where": "any",    "enabled": true},
      {"kind": "keyword", "value": "rechnung",    "mode": "should", "weight": 0.2, "where": "header", "enabled": true}
    ],
    "threshold": 0.8,
    "useLearned": true,
    "date": {"preferredLabels": ["Rechnungsdatum"], "ignoreLabels": ["Fälligkeitsdatum"], "source": "auto", "maxAgeYears": 2}
  },
  "notes": [
    {"rule": 0, "why": "Sender domain in the letterhead"},
    {"warning": "The IBAN in the sample was redacted, so no iban rule was created."}
  ]
}
```

Import defaults (a missing JSON field decodes to Go's zero value, so the importer fills them): `enabled` → true,
`useLearned` → true, `threshold` 0 → 0.8 (`Classify` already treats 0 as default), `date.source` → `auto`.

**Skill / prompt content** (one source of truth, `prompts/type-setup.md`, embedded with `go:embed` and also used by the skill):
- Role and goal: "Return exactly one DocumentType for this receipt, as JSON, nothing else."
- The schema with the enum values from the code (`kind`, `mode`, `where`, `source`) and the weight convention
  (weak 0.2, normal 0.5, strong 0.9). The enums in the prompt are generated from the Go constants or checked by a test, so
  they cannot drift.
- The path template placeholders (`{date:<Go layout>}`, `{type}`, `{invoice}`, `{amount}`, `{original}`, `{text:<label>}`),
  with a note on the Go layout (`2006-01-02`).
- How good rules look:
  - Identifiers of the sender (VAT ID, IBAN, domain) are strong. A fixed identifier may be `must`.
  - Generic words ("Rechnung", "Betrag") are weak.
  - No amounts, dates, invoice numbers or recipient data as rules.
  - Prefer `header` and `footer`; use 3 to 6 rules.
- Matching facts the agent must know: text is compared lower-cased, regexes compile with `(?i)`, so no `[A-Z]` classes;
  IBAN and VAT ID are compared without spaces.
- Redaction contract: placeholders such as `[REDACTED]` or `XXXX` must never become a rule value. If an identifier is
  redacted, the agent says so in `notes` and does not invent one.
- Output: JSON only, validated against a JSON schema where the host supports structured output.
- A worked example (the snippet above).

**Validation and dry run** (headless, in `analysis/`, no UI): `analysis.CheckType(t, existing, samples, learned)` returns a
report. It does not change the classifier.
- *Static checks:* `ParseTemplate(...).Validate()`, every regex compiles, enums are known, weights are in 0..1, `id` is
  non-empty, at least one enabled rule. Redaction placeholders in rule values are an error. A rule that is only
  generic words (all weights ≤ 0.2) is a warning.
- *Collisions:* an `id` that already exists offers "replace" or a suggested unique id.
- *Dry run against the samples:* the type's own score and `Confident` for each sample (needs the sample text, so the
  import dialog takes the sample PDF through the normal pipeline), and which **other** types score above 0 on the sample.
- *Dry run against the library:* filed documents whose text now scores this new type above 0.5 are listed as possible false
  positives. This needs the text cache (`cache.json`); entries without cached text are skipped and counted.
- Failing static checks block "Save". A weak dry run (sample not confident, or false positives) only warns, because a
  type with a single sample often cannot be confident yet.

**CLI check for the agent loop** (second step; the first step is the import dialog only):
`sanity types check <snippet.json> <sample.pdf>... [--project <root>]` runs `CheckType` and prints the report as JSON.
An agent in Claude Code can call it and refine the snippet until it passes. It needs the `pdf` package for text, so it runs
on macOS only. In `main.go` it is a subcommand next to the GUI start, with no UI imports in the code behind it.

**Privacy.** Sample text may contain personal data.
- The prompt asks the user to redact first. The import dialog warns about unredacted own identifiers: it checks the pasted
  snippet and the sample against `Project.IgnoreTokens` and shows hits.
- Sanity itself never calls an LLM and never sends text anywhere. The LLM runs outside the app (the user's agent or chat).
- Option for later: a local model, or an in-app "Suggest rules" button with an explicit consent dialog that shows exactly
  what is sent. Not part of P6.

**UI changes** (Types page, the only page that changes):
- **List column:** a toolbar button "Import type…" and an entry in the row context menu "Copy as JSON" (the same envelope-less
  `DocumentType`, to refine a type with an LLM). The command palette gets "Import type…" and "Copy LLM prompt". No new global
  shortcut. The P6 shortcut overview lists the palette commands anyway.
- **Import dialog** (`widgets.ShowDialog` with a form; no raw Fyne widgets):
  1. A multi-line text field for the snippet, with "Paste from clipboard", and an optional "Add sample PDF…" button
     (`widgets.ShowFileOpen`) for the dry run.
  2. After "Check": a result list with one row per finding, each with icon and label (status is never colour only):
     error, warning, ok. Rules from `notes` show their `why` text next to the rule.
  3. A rule preview grid (read-only, same columns as the editor) and the evaluated path template for an example date.
  4. A dry-run table: sample file, own score, confident yes/no, other types above 0.
  5. Buttons: "Save" (disabled while errors exist), "Save and edit" (opens the type in the editor) and "Cancel".
     The `id` collision choice (replace / new id) sits above the buttons.
- **"Copy LLM prompt" button** (card header of the list, and the palette): copies `prompts/type-setup.md`, with the project's
  current type ids appended so the agent can avoid duplicates, to the clipboard. A toast confirms.
- **Rule grid:** a small note column, shown only for rules that came from an import (`why`), is *not* stored in
  `sanity.json`; it exists only in the import dialog. No new fields in `domain.Rule`.
- **Empty state:** the Types page without any type shows two actions: "Create type" and "Import type…".
- **Presenter:** `ui/presenter/types.go` gets `ParseSnippet`, `Check` (calls `analysis.CheckType`) and `ImportType`, all headless
  with fakes; the view only renders the report.
- **i18n:** new message ids `types.import.*`, `types.copyPrompt.*`, `types.check.*` in `en` and `de`.

**fine changes:** none expected. The dialog uses the multi-line entry, the file dialog (`ShowFileOpen`), a grid and status
badges that exist already. If the multi-line entry cannot paste or show long JSON comfortably, note it in `FINE-NOTES.md`
and fix it in fine (dirty workflow).

**Tests.**
- `analysis`: table tests for `CheckType` (bad enum, bad regex, redaction placeholder, template with `..`, id collision,
  false positive against a second type), using `testdata/` samples.
- `prompts`: a test checks that the enum values in the prompt match the Go constants and that the worked example passes `CheckType`.
- `ui/presenter`: import flow with fakes (valid, invalid, replace, envelope and bare form, defaults filled).
- Quality check with real LLM output is manual: the user runs the skill on 3 to 5 redacted samples and reports the hit rate.

## 4. Architecture

```
domain/          Project, DocumentType, Rule, Template (parse, evaluate, validate), LibraryEntry, InboxItem
analysis/        pure Go: dates, classify (rules, fingerprint, tfidf), fields, dedupe, typecheck (P6); table-driven tests
prompts/         type-setup.md, the LLM prompt (embedded, shared with the skill; P6)
pdf/             pdf.go (interface) + pdf_darwin.go/.m (cgo: render, text, OCR, NSDataDetector, pasteboard)
                 pdf_other.go (stub, ErrUnsupported); fake for tests
store/           Store interface; json.go (sanity.json, atomic write via temp file + rename); fake.go
inbox/           import queue, fsnotify watcher, worker pool (analysis), file operations incl. undo
ui/presenter/    sort.go, library.go, types.go, project.go, datein.go (headless, tests with fakes)
ui/              views + pages.go
locales/         en.json, de.json
```

- `domain`, `analysis`, `pdf`, `store` and `inbox` import no UI and go into the `NoUIDirs` list of `arch_test.go`.
- Module path: `github.com/72nd/sanity` (remote `git@github.com-72nd:72nd/sanity.git`). `go.work` (gitignored) contains `../..` and `.`.

### Data model (starting point)

```go
type Project struct {
    Name         string         `json:"name"`
    Root         string         `json:"-"`            // the folder that contains office/sanity.json
    Inbox        string         `json:"inbox"`        // relative, default "office/inbox"
    IgnoreTokens []string       `json:"ignoreTokens"` // own IBAN, VAT ID, address lines
    Types        []DocumentType `json:"types"`
    Library      []LibraryEntry `json:"library"`
}

type DocumentType struct {
    ID           string     `json:"id"`
    Name         string     `json:"name"`
    PathTemplate string     `json:"pathTemplate"`
    Rules        []Rule     `json:"rules"`
    Threshold    float64    `json:"threshold"`  // preselect from this score on, default 0.8
    UseLearned   bool       `json:"useLearned"` // default true
    Date         DateConfig `json:"date"`
}

type Rule struct {
    Kind    string  `json:"kind"`   // keyword | regex | iban | vatId | customerNo | domain | filename
    Value   string  `json:"value"`
    Mode    string  `json:"mode"`   // should | must | not
    Weight  float64 `json:"weight"` // the UI offers 3 steps: weak 0.2, normal 0.5, strong 0.9
    Where   string  `json:"where"`  // any | header | footer | filename
    Enabled bool    `json:"enabled"`
}

type DateConfig struct {
    PreferredLabels []string `json:"preferredLabels"`
    IgnoreLabels    []string `json:"ignoreLabels"`
    Source          string   `json:"source"` // auto | text | filename | metadata
    MaxAgeYears     int      `json:"maxAgeYears"`
}

type LibraryEntry struct {
    ID, TypeID   string
    Path         string    // relative to Root, with /
    OriginalPath string    // absolute, for undo and as a history
    Date         time.Time
    Invoice      string
    Amount       string    // as found, plus parsed cents and currency
    SHA256       string
    TextHash     string
    FiledAt      time.Time
    Overrides    []string  // which suggestions the user corrected (type, date, ...) for later tuning
}
```

`cache.json` holds, per SHA256, the text (with its source: layer or vision), the fingerprint and the TF-IDF vector.
- `pdf` is the only cgo package.
- **Persistence.** `sanity.json` holds the config and the library metadata. The extracted text and the fingerprints go into
  `<root>/office/.sanity/cache.json`, which can be rebuilt from the PDFs. This keeps the main file small and easy to diff.

## 5. fine changes (dirty workflow, each with the full Definition of Done)

| # | Change | Why | Size |
|---|---|---|---|
| F1 | `widgets.ImageView`: an image with fit width/contain, zoom, scroll and an empty or loading state | PDF preview (`canvas.Image` would be allowed, but a kit widget is cleaner) | S |
| F2 | `DateEntry` quick input: partial input with year inference, `t`/`y`, `Up`/`Down` and `Shift` stepping, an optional `Candidates` list | the most important field in the flow | M |
| F3 | `widgets.Picker`: an inline searchable list with fuzzy ranking, an external score/badge per item, MRU and `Alt+1..9` | DT selection (the ComboBox is substring-only and a popup) | M |
| F4 | Shell drop support: `Shell.SetOnDropped` and a drop overlay hint | drag and drop (Fyne only offers a window-wide callback) | S |
| F5 | `widgets.ShowFolderOpen`: wraps Fyne's in-app folder dialog, like `ShowFileOpen`, with a start folder | Folder picker in the template editor. Apps may not import `dialog`. | S |
| F6 | Default focus per page and `Shell.Focus` helper | keyboard flow after navigation | S |
| F7 | `Grid.OnTypedRune func(rune) bool` (true = handled), plus `Ctrl+D`/`Ctrl+U` half-page movement as standard keys | Vim keys in the library | S |
| F8 | `Esc` in kit text fields (Entry wrapper, ComboBox/Picker, DateEntry) unfocuses after closing an open popup; `Shell.SetOnTypedRune`/`SetOnTypedKey` per page (the shell owns the canvas hooks and resets them on page change) | normal mode | S–M |

Native macOS dialogs (NSOpenPanel) are left out for now, by the user's decision. Fyne 2.8 has none, so the folder picker
in the template editor is Fyne's own dialog for the time being.

Each item is recorded in `FINE-NOTES.md`. These limits get documented and are not worked around silently:
- Fyne has no native dialogs.
- The clipboard is text only.
- Drops arrive through one window-wide callback.
- The ComboBox filters by substring only.
- `DateEntry` parses only the full layout.

## 6. Phases

| Phase | Content | Done when |
|---|---|---|
| **P0** | Verify Vision OCR outside the sandbox | **done**: the user confirmed the OCR output |
| **P1** | Skeleton from `examples/starter`: domain (Template with tests), JSON store and fake, create/open a project, Library page | open a project and see the library with fake data |
| **P2** | `pdf/` package (including scan detection and re-OCR), `inbox/` (Cmd+O, watcher on `office/inbox/`, queue), Sort screen in a plain form (Select, DateEntry), filing as a move with collision handling and undo | file a real PDF end to end |
| **P3** | fine F1 to F4 and F6 to F8, then the keyboard flow on the sort screen and the Vim layer (`ui/presenter/keys.go`) as in section 2 | full keyboard operation, no mouse needed, including hjkl |
| **P4** | `analysis/`: dates, rules, duplicates, invoice no. and amount; suggestions and candidates in the UI | a known type is filed with one key press |
| **P5** | Learning (fingerprint, then TF-IDF), "learn rules from this document", the Types editor with live preview, F5 folder picker | a new type is learned after one document |
| **P6** | Polish: `Cmd+Shift+V` from Finder, multiple projects, a shortcut overview dialog (see section 2), a correction log, a duplicate banner, performance (pre-rendering, Vision warm-up), **LLM-assisted type setup** (section 3a: skill and prompt, `analysis.CheckType`, import dialog on the Types page, `sanity types check` CLI) | import a type from a redacted sample via a JSON snippet, validated and dry-run before saving |

**Agent split** (as the draft asks):
- **Opus** plans, owns the interfaces in `domain`, `store` and `inbox`, does the sort presenter and the integration, and reviews.
- **Sonnet** takes the fine widgets F1 to F5 (one agent each, using the fine-widget skill; CHANGELOG and reference.md are merged one after another),
  `analysis/` (test-driven, against real sample text) and `pdf/` cgo.
- **Haiku** takes the i18n catalogs, the fake data, the table tests for templates and dates, and the gallery states.

Agents that run in parallel touch disjoint packages. The fine agents use separate worktrees.

## 7. Further speed ideas (beyond the draft)

- Undo instead of confirmation, and auto-advance with pre-rendering of the next document.
- MRU ranking of types and an "as last time" type for the same sender.
- Duplicate warning (hash, text hash, invoice no.).
- Multi-select in the queue and filing several documents with the same type in one go (bank statements).
- Background analysis of the whole inbox, so that analysis is never the waiting point.
- A test corpus: real (anonymised) sample receipts in `testdata/`, used to measure the hit rate of the heuristics.
- Later: split multi-page scans that contain several receipts (`Cmd+S` at a page).

## 8. Research notes

### Fyne and fine (2.8.2)

- `Window.SetOnDropped` exists. It is window-wide and passes file URIs.
- The dialog package (`dialog.NewFolderOpen` and the file dialogs) is the Fyne-internal dialog, not native.
- The clipboard is text only.
- `canvas.Image` with `FillMode Contain` is allowed by archcheck.
- fsnotify is already an indirect dependency.
- The grid has no built-in filter and no type-to-find.
- The command palette filters by substring.

### PDF

- Proof of concept, saved in the repo: `research/_poc/` (`pdfmac.m`, `pdfmac.h`, `main.go`, `text.pdf`, `scan.pdf`; raw
  Vision and Swift tests in `standalone/`). Go skips the `_poc` directory, so it does not disturb `go test ./...`.
  - Build: `cd research/_poc && GOWORK=off go build -o /tmp/poc . && /tmp/poc scan.pdf out.png`.
  - The binary is 2.5 MB and needs only system frameworks.
  - Two compiler warnings to fix when porting it to `pdf/`: the `CGBitmapInfo` cast and the deprecated `usesCPUOnly`.
- Signatures that worked: `pdf_page_count`, `pdf_render_png(path, page, scale, &len, &err)`, `pdf_text(path, page|-1)`,
  `ocr_png(png, n, accurate, &err)` and `detect_dates(text)`. Strings are `malloc`ed and freed with `C.free`, and each body sits in an `@autoreleasepool`.
- Port the CGImage RGBA buffer straight to `image.RGBA`, without the PNG round trip.
- Warm Vision up with a tiny image at start.
- **Vision returns no text inside the agent sandbox** (the bash tool's Seatbelt). Outside it works (user, P0).
  OCR tests: `t.Skip` when the warm-up image yields no text. The user runs them by hand with `!`.
- Threading: PDFKit, CoreGraphics and Vision work from any thread, so the work runs in goroutines and the UI is updated through `fyne.Do`.
- A 4x render of A4 takes about 135 MB as RGBA. Cap OCR at about 250 dpi and process one page at a time.

### Regexes (excerpt)

```
DMY     (0?[1-9]|[12]\d|3[01])\.\s?(0?[1-9]|1[0-2])\.\s?(\d{4}|\d{2})   + check the neighbouring byte (RE2 has no lookbehind)
monthDE (0?[1-9]|[12]\d|3[01])\.?\s*(Jan(uar)?|Feb(ruar)?|M(ä|ae)rz?|Apr(il)?|Mai|Juni?|Juli?|Aug(ust)?|Sep(t(ember)?)?|Okt(ober)?|Nov(ember)?|Dez(ember)?)\.?,?\s+(\d{4})
IBAN    [A-Z]{2}\d{2}(?:\s?[A-Z0-9]{4}){2,7}(?:\s?[A-Z0-9]{1,4})?
VAT     DE\d{9} | ATU\d{8} | CHE-\d{3}\.\d{3}\.\d{3}( MWST| TVA| IVA)?
InvNo   (?i)(rechnungs?(nummer|-?nr\.?)|invoice\s?(no\.?|number|#)|beleg-?nr\.?|re-?nr\.?)\s*[:.]?\s*([A-Z0-9][A-Z0-9\-/.]{3,24})
Amount  (?i)(gesamt(betrag)?|total|summe|rechnungsbetrag|zu zahlen|endbetrag)\D{0,20}?(-?\d{1,3}(?:[.' ]\d{3})*,\d{2}|-?\d{1,3}(?:,\d{3})*\.\d{2})\s*(€|EUR|CHF)?
```

Date label weights:
- Positive: Rechnungsdatum, Belegdatum, Invoice date, Kontoauszug vom.
- Negative: Fälligkeit, zahlbar bis, Lieferdatum, Leistungsdatum, gültig bis.
- Neutral: Datum, a line with place and date.
- Periods: Leistungszeitraum and Abrechnungszeitraum become separate values.

## Decisions (user, 2026-11-22)

| Topic | Decision |
|---|---|
| P0 | Vision OCR works outside the sandbox, and the output is as wanted. |
| Storage | `<root>/office/sanity.json`, with a cache that can be rebuilt in `<root>/office/.sanity/`. |
| Inbox | One folder per project: `<root>/office/inbox/`. |
| Files | Never copied; always moved to the target path. |
| Start page | Always the Library. |
| Native dialogs | Left out for now; the Fyne dialog is used through `widgets.ShowFolderOpen`. |
| Documents | DE, CH and US English. About 50 % are scans, often with a poor Paperless/Tesseract text layer, so Vision re-OCR is wanted. |

| Copy path | `Cmd+Shift+C` copies the relative path in Sort and Library; `Cmd+Alt+C` copies the absolute one. |
| Filing | Only `Cmd+Enter` files. `Enter` never files, neither in the form nor in Vim normal mode. |
| Vim + Cmd | Search, copy path, show in Finder (Vim `x`, as in Zed), undo, remove from inbox and re-sort always have a Cmd combination as well. |
| Vim | The user uses Vim. A modal layer (normal mode after `Esc`, `hjkl`, `yy`, ...) is added on top of the Cmd shortcuts and is on by default. |
| LLM type setup | Implemented in P6 (section 3a). The LLM only drafts a type as a JSON snippet from a redacted sample, at configuration time. sanity validates and dry-runs it, and never calls an LLM itself. |

## Test corpus

The user is preparing anonymised sample receipts. Agreed place: `testdata/receipts/` (one PDF per case, both born digital
and scanned, including Paperless PDFs with a poor Tesseract layer) plus `testdata/receipts/expected.json`:

```json
[{"file": "telekom_2025-03.pdf", "type": "Telekom Invoice", "date": "2025-03-12", "invoice": "R-88123", "amount": "49.95 EUR"}]
```

- If the user only provides the PDFs, the implementing session writes `expected.json` together with the user. A test
  `analysis/corpus_test.go` reports the hit rate for type, date, invoice no. and amount. It is a report, not a hard
  threshold, at first.
- The corpus also serves the fake store and the gallery and screenshot runs.
- **Ask** whether the samples may be committed. They are anonymised, but if not, add `testdata/receipts/` to `.gitignore` and let the test skip when the folder is missing.

## Handover for the implementing session

Read in this order:
1. `AGENTS.md` (this repo), `../../AGENTS.md` (fine) and this file.
2. `.claude/skills/fine-app/SKILL.md`, `../../.claude/skills/fine-widget/SKILL.md` and `../../docs/agents/reference.md`.
3. `../../examples/starter` as the template for the skeleton.

Then:
- Start with P1. While P1 runs, F1 and F2 can go to Sonnet agents in parallel, in fine worktrees.
- Record every fine change and every surprise in `FINE-NOTES.md`.
- `go vet ./...` and `go test ./...` must be green in fine and in sanity.
- Do not commit, tag or push in either repo unless asked.
- Mention each fine change in the report (the dirty workflow in `AGENTS.md`).

Known facts that need no new research:
- The Fyne API facts in section 8.
- The keyboard conflicts listed above.
- The OCR behaviour in the sandbox.
- fsnotify is already an indirect dependency of fine.
