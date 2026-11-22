# AGENTS.md

**sanity** helps sort PDF files (mainly accounting receipts) into folder structures. It is also the first
real test app for the fine UI kit (`github.com/72nd/fine`, a Fyne-based kit).

## Project context

- **Purpose:** Take incoming PDFs (receipts), show them, and file them into the right folder of a
  target folder structure with as few steps as possible. Detailed features are not fixed yet; ask the user
  before inventing scope.
- **Location:** This repo lives at `fine/playground/sanity` inside the fine working tree. fine ignores it
  (`/playground/`); both are separate git repos with separate histories. `go.work` here is gitignored and
  uses `../..` (fine) plus `.`, so local fine changes are visible immediately.
- **Dirty workflow (temporary, while sanity is young):** If sanity needs something fine does not have or
  gets wrong (widget, token, shell feature, bug), change fine directly in `../..` instead of working around
  it here. Keep these changes small and focused, and mention each one in your report.
  Every such change carries fine's Definition of Done with it (`../../AGENTS.md`: per-widget checklist and
  rule 10: gallery entry, unit test, golden images, contrast test, CHANGELOG, docs/guide,
  docs/agents/reference.md, Example test; skill `../../.claude/skills/fine-widget`). Do it in the same
  task, not later. Where it is a self-contained chunk, delegate it to a subagent (fine-widget skill) while
  you continue in sanity; otherwise do it yourself. `go vet ./...` and `go test ./...` must be green in
  fine and in sanity. Do not commit, tag or push in either repo unless asked.
- **Findings for fine:** Collect gaps and surprises about the kit in `FINE-NOTES.md` so they can be
  cleaned up properly later.

This app is built on the fine UI kit (`github.com/72nd/fine`, a Fyne-based kit). The rules below
are enforced by `arch_test.go` (`archcheck`) and `.golangci.yml`. Run `task check` before every commit.

Layout: `domain/` (entities, rules), `store/` (interface + `fake.go`), `ui/presenter/` (view logic),
`ui/` (views and `pages.go`), `locales/` (catalogs), `main.go` (wiring).

## Rules

1. **Tokens only.** No color literals (`color.NRGBA{...}`, hex) anywhere. Use `theme.Color(theme.Text)` and
   `theme.Size(theme.SizeRow)`. Custom accent: `shell.Config.AccentHue`, verified with `theme.Check`.
2. **No raw Fyne widgets.** Never import `fyne.io/fyne/v2/widget`, `.../dialog` or `.../container`. Use
   `github.com/72nd/fine/widgets`, `widgets/grid` and `shell`. Plain `fyne` and `fyne/v2/app` imports are fine.
3. **Dependency direction.** `domain/` and `store/` import no UI (neither `fyne.io` nor fine).
   Views hold no logic: they call presenters. Presenters import no `fyne.io`, so they are tested headless.
4. **Threading.** Store calls run in goroutines; UI updates from goroutines go through `fyne.Do` / `fyne.DoAndWait`.
5. **No hard-coded UI text.** Views use `i18n.T("message.id")`. Every id exists in every catalog in
   `locales/` (the locales test checks this). Do not use the reserved prefixes `shell.` and `command.`.
6. **Status is never color only.** Use `theme.StatusStyleOf` or `widgets.NewStatusBadge` (icon + label).
7. **Every app has a fake store** with sample data (`store/fake.go`) behind the `store.Store` interface.
8. **Small files, clear interfaces:** props in, callbacks out.
9. **Architecture test stays in the suite.** `arch_test.go` calls `archcheck.Run`; add every new package
   directory to its lists.
10. **Forms and settings pages** go in a `widgets.NewCard` inside `widgets.PagePadded` (see `docs/guide/widgets.md`).
    Destructive actions (delete) ask first: `widgets.ShowConfirm(..., danger=true, ...)`; use `ShowError` for failures, `ShowContextMenu` for row actions.
11. Report results that contradict these assumptions (for example "Fyne cannot do X"). Do not gloss over them.

## Finding documentation

- API: `go doc github.com/72nd/fine/<pkg>` (`theme`, `widgets`, `widgets/grid`, `shell`, `shell/i18n`, `archcheck`).
- Guides and the agent cheat sheet ship in the module cache:
  `$(go list -m -f '{{.Dir}}' github.com/72nd/fine)/docs/` (`guide/*.md`, `agents/reference.md`).
- Widget catalogue: `go run github.com/72nd/fine/gallery/cmd/gallery@latest`.

## Commands

`task test` · `task vet` · `task lint` · `task check` · `task run`

## Skill

`.claude/skills/fine-app/SKILL.md` describes how to add a page, a store method and i18n ids.
