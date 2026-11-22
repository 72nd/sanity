# sanity user guide

This guide describes how to file documents with sanity day to day. For the project itself see
[README.md](../README.md). Items marked **planned** are designed but not built yet; they are listed
in the last section so that nothing in the rest of the guide is a promise the app does not keep.

In the shortcut tables, `Cmd` is the Command key. Fine's shortcuts use the Command key on macOS.
`Ctrl` is the Control key.

## Contents

1. [Concepts](#concepts)
2. [Adding documents](#adding-documents)
3. [Filing a document](#filing-a-document)
4. [The type picker](#the-type-picker)
5. [The date field](#the-date-field)
6. [Keyboard shortcuts](#keyboard-shortcuts)
7. [The Vim layer](#the-vim-layer)
8. [Undo, removing and duplicates](#undo-removing-and-duplicates)
9. [The Library](#the-library)
10. [The Types editor](#the-types-editor)
11. [Privacy](#privacy)
12. [The correction log](#the-correction-log)
13. [Planned, not built yet](#planned-not-built-yet)

## Concepts

- **Project**: one company's accounting. It is a folder that contains `office/sanity.json`. The app
  opens one project folder at a time.
- **Inbox**: `office/inbox/` inside the project. Documents waiting to be filed. They stay where they
  are until you file them.
- **Document type**: a kind of document (for example "Telekom Invoice"). A type has a path template
  that says where its documents go, rules that recognise them, and settings for the date.
- **Library**: all documents that have been filed. It is the list on the Library page.

The pages are in the sidebar and have these shortcuts:

| Shortcut | Page |
|---|---|
| `Cmd+1` | Library (the start page) |
| `Cmd+2` | Sort (the work screen) |
| `Cmd+3` | Types (the type editor) |
| `Cmd+4` | Settings (fine's settings page, with the Vim switch) |

## Adding documents

There are four ways to get a PDF into the queue. All of them only add the PDF to the queue; the
file itself is not touched.

1. **The inbox folder.** Save or move a PDF into `<project>/office/inbox/`. The app watches the
   folder. A new PDF appears in the queue after it has been quiet for about 0.4 seconds, so a file
   that is still being written is not picked up early. PDFs already in the folder are imported when
   the project opens.
2. **Drag and drop.** Drag PDFs or whole folders onto the window. Folders are read recursively. Hidden
   entries (names that start with a dot) are skipped, and only `.pdf` files are taken. The app shows
   the Sort page with the first dropped document open.
3. **`Cmd+O`** (or `o` in Vim normal mode, or the "Add files…" button in the queue). A file dialog
   opens, filtered to PDF files.
4. **`Cmd+Shift+V`** imports the PDFs that you copied in Finder (select the files in Finder and press
   `Cmd+C`, then `Cmd+Shift+V` in sanity). Text paste stays on `Cmd+V`. If the clipboard has no
   files, a message says so.

Documents are analysed in the background, two at a time. Until the analysis is done the form shows
no suggestions, and it fills in as soon as the analysis of the open document finishes.

## Filing a document

The Sort page has three parts: the queue on the left, the PDF preview in the middle and the form on
the right. The form has the fields Type, Date, Invoice no. and Amount, then the target path, then
the buttons.

**For a known receipt** (the analysis suggested a type with confidence, and the date and invoice
number were found):

1. Check the form. The path line shows where the file will go.
2. Press `Cmd+Enter`.

**For an unknown receipt**, five steps:

1. Type a few letters of the document type in the Type field, for example `tel`. The list shows the
   matching types.
2. Press `Enter`. This takes the highlighted type and moves to the Date field.
3. Type the date, for example `1203` for 12 March. Press `Enter`. The date rules are in
   [The date field](#the-date-field).
4. Fill any field that the hint line names as still needed (for example `Still needed: Invoice no.`).
   `Enter` moves from one empty field to the next: Date, then Invoice no., then Amount.
5. Check the path line and press `Cmd+Enter`.

`Enter` never files a document. Only `Cmd+Enter` (or the File button) does. When a required value
is missing, `Cmd+Enter` shows a warning and nothing is filed.

After filing, the toast shows the new path and "Cmd+Alt+Z undoes". The next document in the queue
opens at once, with the Type field focused.

`Cmd+Shift+C` copies the target path of the open document when that path can already be worked out
(type and all path values are set). Otherwise it copies the path of the document filed last. Right
after a filing the open document is usually the next one, which normally has no complete path yet,
so the copy then gives the path of the document just filed. If the next document already has a
complete path, that path is copied instead.

**Duplicates.** If the document looks like one already in the library, a line above the form says
"Looks like a duplicate of ..." with the path of the existing document and the reason (same file,
same text, or same invoice number within the same type). The line does not block filing. Its button
**Show in library** opens the Library page with the filter set to that path and the entry selected.

**Free text fields.** When the template of the selected type has `{text:<label>}` placeholders (for
example `projects/{text:Project}/...`), the form shows one more entry per label below Amount. A label
that appears twice in the template has one entry. `Enter` moves through the empty ones like through
the other fields, the hint line names a missing one (`Still needed: Project`), and the path line
shows the result. What you typed stays per label when you switch to another type and back, and is
cleared when the next document opens. A batch has no form, so a document of a batch whose type needs
a text is shown as blocked; file it alone.

**Skipping.** `Cmd+]` and `Cmd+[` open the next and previous document without filing the current one.
The Skip button does the same. The document stays in the queue.

**Learn rules.** `Cmd+L` (or the Learn button) looks at the open document for distinctive identifiers
(IBAN, VAT ID, domain, customer number) that are not yet rules of the selected type. A dialog lists
them as checkboxes; the ticked ones are added to the type and saved. Your own identifiers, set as
ignore tokens in the project, are never suggested.

## The type picker

The Type field is a search field with the list of types directly below it. It is not a popup.

- **Ranking.** With an empty field the list is sorted by the score of the analysis, then by the
  types you used most recently. While you type, the list is filtered fuzzily: the letters must
  appear in order, and matches at word starts rank higher. Then the score decides.
- **Suggested.** A type is suggested when its score reaches the threshold of that type (default 0.8)
  and clearly leads the next type. It is preselected and marked with the badge "Suggested". The
  badge is text, not only colour.
- **Picking.** `Enter` takes the highlighted type. `Alt+1` to `Alt+9` pick the type shown with that
  digit next to the row. A click on a row picks it too.
- **Moving through the list** without leaving the field: `Up` and `Down`, `Ctrl+J` and `Ctrl+K`,
  `Ctrl+N` and `Ctrl+P`, and `PageUp` and `PageDown`.
- **Leaving the field.** `Esc` takes the focus out of the field. In Vim normal mode the letters are
  commands again (see [The Vim layer](#the-vim-layer)).

## The date field

The date is typed fast. The field reads the input as soon as you press `Enter` or leave the field,
and shows the full date then. The day comes before the month. The year is the most recent one for
which the date is not after today. Today is 2026-11-22 in this guide's examples.

| You type | Meaning (example on 2026-11-22) |
|---|---|
| `1203`, `12.3`, `12/3`, `12-3`, `12 3` | 12 March 2026 (the most recent 12 March that is not in the future) |
| `120325`, `12.3.25`, `12.03.2025` | 12 March 2025 (two-digit years are 2000 + year) |
| `12032025` | 12 March 2025 (four-digit year) |
| `2025-03-12` | 12 March 2025 (ISO order when the first part has four digits) |
| `t` | today |
| `y` | yesterday |

An impossible date, such as `3002` or `31.02.`, is not accepted; the text stays as you typed it.

Keys in the date field:

| Key | Action |
|---|---|
| `Up` / `Down` | one day later / earlier |
| `Shift+Up` / `Shift+Down` | one month later / earlier |
| `Ctrl+A` / `Ctrl+X` | one day later / earlier (the Vim increment and decrement keys) |
| `Alt+Shift+1` to `Alt+Shift+3` | pick the date candidate with that number |
| `Enter` | accept the date and move to the next empty field |
| `Esc` | close the candidate list if it is open; otherwise leave the field |

**Candidates.** The analysis finds dates in the document, for example "Invoice date" or "due date".
They are listed below the field with their label, numbered 1 to 3. When the analysis could only
guess (for example from the file date), the label says "Fallback (file date)", and the date is never
marked as confident.

## Keyboard shortcuts

These tables were checked against the code. The Command layer is always active. The Vim layer
is described in [The Vim layer](#the-vim-layer).

### Everywhere

| Shortcut | Action |
|---|---|
| `Cmd+1` … `Cmd+4` | Library, Sort, Types, Settings |
| `Cmd+K` | command palette (all commands, filtered by typing; fine's shell) |
| `Cmd+D` | switch light and dark theme (fine's shell) |
| `Cmd+Alt+Z` | undo the last filing (`Cmd+Z` stays text undo). Works once the Sort page has been opened in this session. |

### Sort page

| Shortcut | Action |
|---|---|
| `Cmd+Enter` | file the open document and open the next one |
| `Enter` (in a field) | accept the field and move to the next empty one; never files |
| `Cmd+]` / `Cmd+[` | next / previous document (skip, nothing is filed) |
| `Cmd+O` | add PDF files with the file dialog |
| `Cmd+Shift+V` | import the PDFs copied in Finder |
| `Cmd+F` | search types (focuses the Type field) |
| `Cmd+Shift+C` | copy the relative target path, for example `receipts/2025/telecom/2025-03-12_telekom_R-88123.pdf` |
| `Cmd+Alt+C` | copy the absolute target path |
| `Cmd+R` | show the current inbox file in Finder |
| `Cmd+Shift+Backspace` | remove the document from the inbox (after confirmation; the file goes to the macOS Trash) |
| `Cmd+L` | learn rules from this document |
| `Cmd+=` / `Cmd+-` / `Cmd+0` | zoom the preview in / out / fit to width |
| `Alt+Down` / `Alt+Up` | next / previous page of the preview |
| `Ctrl+D` / `Ctrl+U` | scroll the preview half a page down / up |
| `Alt+1` … `Alt+9` | pick a type from the type list |
| `Alt+Shift+1` … `Alt+Shift+3` | pick a date candidate |

The Sort commands also appear in the command palette (`Cmd+K`). Their shortcuts are the same.

## The Vim layer

The Vim layer is on by default. It adds a normal mode to the Sort page, on top of the Command
shortcuts. Switch it off in **Settings, Vim keys**. The change applies the next time the Sort page
is shown.

**Modes.**

- **Insert** is the default when a document opens. A field has the focus and letters are text. The
  status bar shows `INSERT`.
- **Normal** starts with `Esc` in a field. Nothing is focused, so letters are commands. The status
  bar shows `NORMAL`. `i` (first empty field), `t` (Type) or `d` (Date) bring you back into a field.

Filing stays `Cmd+Enter` in both modes. No Vim key files a document.

**Normal mode on the Sort page.**

| Key | Action |
|---|---|
| `j` / `k` | next / previous document |
| `gg` / `G` | first / last document |
| `h` / `l` | previous / next page of the preview |
| `+` / `-` / `=` | zoom in / out / fit to width |
| `t` / `d` / `i` | focus Type / Date / first empty field |
| `x` | show the current inbox file in Finder |
| `u` | undo the last filing |
| `yy` / `Y` | copy the relative / absolute target path |
| `dd` | remove the document from the inbox (after confirmation) |
| `o` | add files |
| `/` | focus the type search |
| `:` | open the command palette |

Multi-key sequences (`gg`, `yy`, `dd`) must be completed within 0.8 seconds. Because `dd` exists, a
single `d` waits about 0.8 seconds before it focuses the Date field.

**Insert mode extras** (typing in the fields):

| Key | Action |
|---|---|
| `Ctrl+J` / `Ctrl+K`, `Ctrl+N` / `Ctrl+P` | move through the type list without leaving the field |
| `Ctrl+A` / `Ctrl+X` | in the date field: one day later / earlier |

## Undo, removing and duplicates

**Filing is a move.** Filing moves the PDF from the inbox (or wherever it was imported from) to its
target path. On the same disk this is a rename. Across disks the file is copied, verified by hash,
and the original is then deleted. The original is never kept as a copy. The file is never
overwritten: if the target name is taken, `_2`, `_3` and so on are added before the extension, for
example `2025-03-12_telekom_R-88123_2.pdf`.

**Undo** (`Cmd+Alt+Z`, or `u` in normal mode) reverses the last filing:

- the PDF moves back to its original location, and the folder is recreated if needed;
- the document goes back to the top of the queue, and its library entry is removed;
- folders that the filing created and that are now empty are removed again.

Each press undoes one filing, newest first. The undo history lives in memory, so it covers the
filings made since the app was started.

**Removing** (`Cmd+Shift+Backspace`, or `dd` in normal mode) takes a document out of the inbox
without filing it. After a confirmation the file is moved to the macOS Trash. You can restore it from
the Trash in Finder. A removed document is not in the library and cannot be undone in sanity.

## The Library

The Library is the start page. It lists every filed document with these columns: Date, Type, File
(the relative path), Invoice no., Amount and Filed at. The newest date is first.

- The field above the grid filters the rows as you type. It matches the path, the type name, the
  invoice number and the amount.
- `Cmd+F` focuses the filter field; `Enter` in it returns to the grid. The filter text and the
  selection are kept when you visit another page.
- When documents wait in the inbox, a line above the filter says how many, with a button that opens
  the Sort page.
- The status bar shows the number of rows shown and in total. With a selection it adds "N selected,
  total X EUR": the amounts of the selected rows are added up per currency (rows whose amount cannot
  be read are left out).
- `Enter` (or double-click) opens the PDF in the default viewer.
- The context menu (right click on a row) offers copy relative path, show in Finder, sort again and
  remove from the library. Removing asks first and deletes only the library entry; the file stays.
  Sort again puts the file back into the sort queue and shows the Sort page; filing it again moves and
  renames it. The old library entry stays until then.

| Shortcut | Action |
|---|---|
| `Cmd+F` | focus the filter |
| `Cmd+Shift+C` / `Cmd+Alt+C` | copy the relative / absolute paths of the selected rows, one per line |
| `Cmd+R` | show the selected document in Finder |
| `Cmd+E` | sort the selected documents again |

The same keys work on the Sort page for the open document; which one runs depends on the page on screen.

With the Vim layer on (the grid has the focus), these keys work:

| Key | Action |
|---|---|
| `j` / `k` / `gg` / `G` / `Ctrl+D` / `Ctrl+U` | move the cursor |
| `/` | focus the filter (`Enter` goes back to the grid) |
| `n` / `N` | next / previous row that matches the filter, wrapping around |
| `v` | start a range selection that movement extends; `v` again ends it |
| `V` | select all rows shown |
| `yy` | copy the relative paths of the selection |
| `x` | show in Finder |
| `r` | sort again |
| `Enter` | open the document |

## The Types editor

Open it with `Cmd+3`. The list of types is on the left, with the buttons **New type**, **Duplicate**
and **Delete** (delete asks first; filed documents are kept). The detail is on the right, in four
cards. **Revert** and **Save** become active when there are unsaved changes. If you select another
type while there are unsaved changes, you are asked whether to discard them.

**Basics**

- **Name.**
- **Path template.** The path is relative to the project root. It is shown with a live preview for an
  example date and example values. The preview turns red when the template is invalid or escapes the
  project folder with `..`. Below the field is a short legend of the Go date layout tokens (`2006`,
  `01`, `02`, `Jan`, and so on).
- **Choose folder…** opens a folder picker inside the project. It inserts the relative folder and
  keeps the file name part. A folder outside the project is refused.

Placeholders in the path template:

| Placeholder | Value |
|---|---|
| `{date:<Go layout>}` | the document date, for example `{date:2006-01-02}` gives `2025-03-12` |
| `{type}` | the type name, made safe for a file name |
| `{invoice}` | the invoice number from the form |
| `{amount}` | the amount from the form, for example `49.95` |
| `{original}` | the original file name without `.pdf` |
| `{text:<label>}` | a free text value; the sort form shows one extra entry named `<label>` while the type is selected (see [Free text fields](#free-text-fields)) |

Example:

```
receipts/{date:2006}/telecom/{date:2006-01-02}_telekom_{invoice}.pdf
```

A placeholder without a value (for example no invoice number was found) makes the form ask for that
value before filing. Unsafe characters in values, such as `/`, become `_`. Paths must be relative and
must not contain `..`.

**Recognition**

- **Preselect from score** (default 0.8): the score from which the type is suggested and preselected.
- **Use learned documents**: lets the documents already filed with this type help recognise new ones.
- **Rules**: a grid with one row per rule.

| Column | Values |
|---|---|
| On | the rule is used or ignored |
| Kind | Keyword, Regex, IBAN, VAT ID, Customer no., Domain, File name |
| Value | the text to look for (regular expressions are case-insensitive) |
| Mode | Should (adds to the score), Must (the type needs this rule), Not (excludes the type) |
| Strength | Weak (0.2), Normal (0.5), Strong (0.9) |
| Where | Anywhere, Header, Footer, File name |

Use **Add rule** and **Remove rule** for the rows. Identifiers of the sender (VAT ID, IBAN, domain)
are the strong rules; generic words are weak.

**Date**

- **Preferred labels** (comma separated), for example `Invoice date`. A date next to a preferred
  label scores higher.
- **Ignored labels** (comma separated), for example `Due date`.
- **Source**: Automatic, Text, File name or PDF metadata.
- **Maximum age** (years). Dates older than this are not suggested. The value 0 means 15 years.

**Learning.** A type learns from the documents you file with it (the letterhead fingerprint and the
text of each document). Until a type has three filed documents, its learned score is capped at 0.7,
so one example alone does not produce a confident suggestion without a rule. The learned data is kept
in the cache file of the project (`office/.sanity/cache.json`), which can be rebuilt from the PDFs.

## Privacy

- sanity runs entirely on your Mac. It has no network code. PDF rendering, text extraction, OCR
  (Vision) and date detection (PDFKit and Vision) happen on the device.
- Your PDFs stay where they are until you file them, and filing only moves them inside the project.
- `office/.sanity/cache.json` contains the extracted text of your documents. Treat the project folder
  with the same care as the PDFs themselves.
- Your own identifiers (IBAN, VAT ID, address lines) can be listed as `ignoreTokens` in
  `office/sanity.json`. They are left out of the learned fingerprints and the suggestion index, and
  the Learn dialog never offers them.

## The correction log

Every filing is written as one line of JSON to `office/.sanity/corrections.jsonl` of the project (the
demo project writes none). The line records what the analysis suggested (top type with its score and
whether it was confident, top date, invoice number and amount) next to what you filed, so thresholds
can be tuned later. Documents of a batch get a line each. An undo adds a line with `"undone": true`
and the same SHA-256; no line is ever deleted.

`sanity corrections [--project <root>]` reads the log and prints the number of filings, the type hit
rate (suggested type equals the filed type), the precision of confident suggestions, the date hit rate
and the hits per type. Filings that were undone are not counted. The project is the current directory
unless `--project` is given.

## Planned, not built yet

These items are in [PLAN.md](../PLAN.md). They are not in the app today.

- **Inbox count in the Sort sidebar title.** Today the count is in the Sort status bar ("N open"); the Library hint is built.
- **Shortcut overview** (`Cmd+/`, and `?` in normal mode).
- **Project page and project switcher** (`Cmd+Shift+P`). Today a project is opened with the command
  line argument. To change its ignore tokens, edit `office/sanity.json` while sanity is closed, since
  the app rewrites the file on every change.
- **Multi-select and batch filing** in the queue.
- **Importing a document type** from a JSON snippet, **Copy as JSON** per type, and a **Copy LLM
  prompt** action in the Types editor, with a `sanity types check` command. The app itself never calls
  a language model.
- **`Cmd+Down` / `Cmd+Up`** for the preview pages. The app uses `Alt+Down` and `Alt+Up`, as listed
  above.
