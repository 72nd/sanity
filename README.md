# sanity

sanity files PDF receipts (mostly accounting receipts) into the folder structure of a company's
accounting. You open a project folder, the incoming PDFs appear in a queue, the app reads each one
and suggests a document type, a date, an invoice number and an amount, and one key press moves the
file to its target path. Every decision serves filing speed: a well-known receipt is filed with
`Cmd+Enter`, and an unknown one takes about five keys.

It is built for one bookkeeper who works with Vim and handles one company at a time. Everything
runs on the Mac; sanity has no network code.

The user guide with the complete workflow and keyboard shortcuts is in [docs/USAGE.md](docs/USAGE.md).
The design notes and the implementation plan are in [PLAN.md](PLAN.md).

## Requirements

- macOS. PDF rendering, text extraction and OCR use PDFKit and Vision through cgo. OCR is
  macOS-only; the other platforms get a stub.
- Go 1.27.3, the `go` line in `go.mod`.
- Xcode Command Line Tools, because cgo compiles Objective-C.
- The fine UI kit (`github.com/72nd/fine`) checked out around this repo. `go.mod` contains
  `replace github.com/72nd/fine => ../..`, so sanity expects to live in `fine/playground/sanity`.
  The local `go.work` (gitignored) uses `../..` and `.`.

## Run

```sh
go run . <project folder>   # open a project folder
go run . -demo              # sample data in a temporary folder, fake store
go run .                    # use the current directory as the project folder
```

A folder without `office/sanity.json` becomes a new project named after the folder. The app creates
the files it needs.

## Project layout

```
<root>/office/sanity.json          config: name, inbox, ignore tokens, document types, library
<root>/office/inbox/               drop PDFs here; the folder is watched
<root>/office/.sanity/cache.json   extracted text and fingerprints; a pure cache, rebuilt from the PDFs
```

`sanity.json` is written atomically (temp file, then rename) on every change. The cache can be
deleted at any time; it is rebuilt from the PDFs.

## Build and test

```sh
go build -o sanity .
go test ./...
go vet ./...
```

The repo also has a `Taskfile.yml` (`task test`, `task vet`, `task lint`, `task check`).

Some OCR tests skip themselves when Vision returns no text. This happens inside the agent sandbox,
where Vision is silent; on a normal Mac they run.

## Architecture

| Package | Role |
|---|---|
| `domain` | entities (project, document type, rule, path template, library entry); no UI, no I/O |
| `store` | `Store` interface, JSON store for `sanity.json`, fake store for `-demo` |
| `inbox` | import queue, folder watcher, analysis workers, filing and undo (files are always moved) |
| `analysis` | pure Go: dates, rules and learning, invoice number and amount, duplicates |
| `pdf` | the only cgo package: render, text, Vision OCR, date detection (stub outside macOS) |
| `clipboard` | files copied in Finder, read from the macOS pasteboard (cgo) |
| `ui/presenter` | headless view logic and the Vim key parser, tested with fakes |
| `ui` | Fyne views, pages and commands, built only from the fine kit |

Architecture rules (enforced by `arch_test.go` and `.golangci.yml`): `domain`, `store`, `inbox`, `pdf`
and `clipboard` import no UI; presenters import no Fyne package; views contain no logic and no
hard-coded text (all text comes from `locales/`, `en` and `de`).
