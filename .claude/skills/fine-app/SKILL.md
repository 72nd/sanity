---
name: fine-app
description: Use when building or changing an application on the fine UI kit (github.com/72nd/fine, Fyne-based): adding a page or view, a presenter, a store method, an i18n message id, a grid, or wiring pages in ui/pages.go. Triggers on imports of github.com/72nd/fine/shell, widgets or theme, and on layout domain/ store/ ui/presenter/ locales/.
---

# Working on a fine app

The pattern is `examples/starter/` in the fine repository (a complete minimal app). Read its
`ui/presenter/todos.go`, `ui/todos_view.go` and `ui/pages.go` before writing new code. The
repo's `AGENTS.md` rules apply; `arch_test.go` enforces the dependency rules.

Look up APIs with `go doc github.com/72nd/fine/<pkg>[.Symbol]`. A cheat sheet is in
`$(go list -m -f '{{.Dir}}' github.com/72nd/fine)/docs/agents/reference.md`.

## Add a page

1. **Domain** (if new data): add the entity and its rules to `domain/`, plus a test. No UI imports.
2. **Presenter** in `ui/presenter/<name>.go`: a struct that takes a `store.Store` and a
   `Translator` (`i18n.T` in production). Methods return plain values (strings, slices,
   errors). No `fyne.io` imports. Write `<name>_test.go` with a stub translator and `store.NewFake`.
3. **i18n ids**: add every new id to every catalog in `locales/*.json` (same keys, translated
   values). Ids look like `area.thing.detail` (`nav.todos`, `col.title`, `error.title.required`).
   Never use the `shell.` or `command.` prefix. `locales_test.go` fails when catalogs diverge.
4. **View** in `ui/<name>_view.go`: `func XPage(sh *shell.Shell, p *presenter.X) fyne.CanvasObject`.
   Compose `widgets.*` and `grid.Grid[T]`. Text comes from `i18n.T`. Run store calls in a
   goroutine and update the UI inside `fyne.Do`. Forms and settings go in a `widgets.NewCard`
   inside `widgets.PagePadded` (`Scroll(PagePadded(MaxWidth(600, NewCard("", "", form))))`).
5. **Wire it** in `ui/pages.go`: add a `shell.Page{ID, TitleKey, Icon, Build}` and pass the
   presenter in from `main.go`.

## Add a store method

1. Add the method to the interface in `store/store.go` (context first, domain types only).
2. Implement it in `store/fake.go` with sample data. Keep the fake deterministic.
3. Use it from a presenter, never from a view. Add a presenter test against the fake.

## Finish

Run `task check` (vet, test, lint). Fix `arch_test.go` failures by moving code, not by loosening
the rules. If a new package directory was added, add it to the lists in `arch_test.go`.

## Do not

- Import `fyne.io/fyne/v2/widget`, `dialog` or `container`.
- Write color literals or hard-coded UI strings.
- Show status by color alone: use `widgets.NewStatusBadge` or `theme.StatusStyleOf`.
- Update widgets from a goroutine without `fyne.Do`.
