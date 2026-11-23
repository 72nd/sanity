# Umsetzungsplan v0.2.0

Status: freigegeben (2026-11-23). Quelle: `prj/todo.md`. Stand von sanity: v0.1.0 noch nicht getaggt (siehe Phase 0).

Ein Orchestrator (Opus, die Hauptsession) verteilt die Arbeit auf parallele **Sonnet**- und **Haiku**-Agenten,
führt die geteilten Dateien selbst und integriert nach jeder Welle. Die Agenten committen nicht.

## Überblick

| Phase | Inhalt | parallel |
|---|---|---|
| 0 | CHANGELOG anlegen, v0.1.0 committen und taggen, auf v0.2.0 bumpen | nein (Orchestrator) |
| Welle 1 | fine-Bausteine (F1–F6) und unabhängige sanity-Pakete (S1, S2, S3, S7, S9) | 11 Agenten |
| Integration 1 | Doku und CHANGELOG von fine, Goldens, `task check` in beiden Repos | nein |
| Welle 2 | Bildschirme, die auf Welle 1 aufbauen (S4, S5, S6, S8, S10) | 5 Agenten |
| Integration 2 | CHANGELOG sanity, FINE-NOTES, Testliste für die manuelle Abnahme | nein |

## Entscheidungen

Am 2026-11-23 so freigegeben, wie empfohlen.

| # | Frage | Entscheidung |
|---|---|---|
| E0 | Gehören die offenen Änderungen `FyneApp.toml` (Build 3) und `pdf/classify.go` (`minLayerChars` 20 → 40) in v0.1.0? | Ja, beide in den Release-Commit. |
| E1 | Import-Modell: Wird jede importierte Datei in die Inbox **verschoben**? | Ja. Jede Datei, die per Drop, Dateidialog oder „Neu sortieren“ hereinkommt, wird nach `office/inbox/` verschoben. Der Ursprungsort wird in `office/inbox/.origins.json` festgehalten. Damit überlebt die Inbox den Neustart (der Watcher liest den Ordner beim Start ein), und „Zurücklegen“ hat ein Ziel. |
| E2 | „Zurücklegen“ vs. „Aus Sort entfernen“ | Ein Befehl **Zurücklegen**: verschiebt an den Ursprungsort und entfernt aus der Queue. Er deckt ab: Dokument aus dem Projektordner wieder loswerden, Neu-Sortieren abbrechen (Datei geht an ihren Bibliothekspfad zurück, der Eintrag bleibt gültig). Dateien ohne Ursprung (lagen schon in der Inbox) können nicht zurückgelegt werden, nur gelöscht. |
| E3 | Bestehende, korrekt abgelegte Belege in die Bibliothek aufnehmen | v0.2.0: Befehl **„Am Ort aufnehmen“** für Dokumente mit Ursprung im Projektordner: legt einen Bibliothekseintrag mit Typ, Datum und Feldern aus dem Formular an, die Datei geht an den Ursprungsort zurück (kein Umbenennen). |
| E4 | Welcome-Ansicht: wann? | Beim Start ohne Argument immer. Mit Pfad-Argument oder `-demo` direkt ins Projekt. |
| E5 | Projekt-Dropdown in der Toolbar | Listet die zuletzt geöffneten Projekte, darunter „Projekt öffnen…“ und „Übersicht“ (Welcome). |
| E6 | Icon | Der Icon-Agent liefert 2–3 Varianten nach `prj/icons/v0.2/`. Du wählst, erst dann wird `icon/` ersetzt. |
| E7 | Changelog-Regel sagt „erst bumpen, nachdem ausgeliefert“, `todo.md` sagt „direkt danach bumpen“. | Wie in `todo.md`: taggen, dann sofort bumpen. Kein Push. |

## Phase 0: Release v0.1.0 (Orchestrator)

1. `.claude/rules/changelog*.mdc` an sanity anpassen: Sie stammen aus einem Wails-Projekt. Versionsquellen sind
   `FyneApp.toml` (`Version`) und die neue `version.go` (siehe S7); `whatsNew.ts` gibt es nicht, der Schritt entfällt.
2. `.gitignore`: `/sanity` (Binary) und `.task/` ergänzen.
3. `CHANGELOG.md` anlegen: `## v0.1.0 – Erste nutzbare Version` mit Bullets aus `git log` (FEAT/FIX/… nach Regel).
4. Commit (inkl. E0, `prj/todo.md` und dieser Plan), Tag `v0.1.0`. Nicht pushen.
5. Bump: `FyneApp.toml` → `0.2.0`, Build zurück auf 1, leere Überschrift `## v0.2.0` am Ende von `CHANGELOG.md`.
   Commit `chore: bump to v0.2.0`.

## Arbeitspakete

Spalte „Dateien“ ist **Besitz**: nur dieser Agent ändert sie in seiner Welle. Neue Dateien sind erlaubt.
Modell: **S** = Sonnet, **H** = Haiku.

### Welle 1: fine (Repo `../..`)

Jede Änderung erfüllt fines Definition of Done (`../../AGENTS.md`, Skill `fine-widget`): Galerie-Eintrag,
Unit-Test, Goldens hell/dunkel, Kontrasttest, Example-Test, Abschnitt in `docs/guide/`.
`CHANGELOG.md` und `docs/agents/reference.md` von fine schreibt **nur der Orchestrator** (Text liefert der Agent im Bericht).

| ID | Todo | Modell | Dateien | Ergebnis |
|---|---|---|---|---|
| F1 | Version und Shortcuts im Footer, Projekt rechts in der Toolbar | S | `shell/shell.go`, `widgets/statusbar.go`, `widgets/toolbar.go` | `StatusBar.SetTrailing(objs ...fyne.CanvasObject)`: feste Objekte rechts, unabhängig vom Seitenwechsel. `shell.Config.Version` (als gedämpftes Label im Footer). `Shell.SetToolbarApp(objs ...)`: App-Objekte links vor Palette/Theme. |
| F2 | Command Palette: Shortcuts rechtsbündig und anders formatiert; keine unnötige horizontale Scrollbar | S | `shell/palette.go` | Eigene Zeile mit Titel links, Shortcut rechts (Mono, gedämpfte Farbe, Token). Horizontale Scrollbar nur bei Bedarf. |
| F3 | Dialoge mit Buttons im Footer; Label mit Umbruch; Breite steuerbar | S | `widgets/dialog.go`, `widgets/label.go` | `ShowDialog(win, title, content, opts DialogOptions)` mit Buttons im Footer neben Abbrechen und `MinWidth`. `Label.Wrap` (oder `LabelWrap`), damit lange Texte im Dialog umbrechen. |
| F4 | Textnavigation (macOS: Cmd/Alt+Pfeile, Linux/Windows: Ctrl+Pfeile, Home/End) in Entry/TextArea; Monospace-Option | S | `widgets/inputs.go`, `widgets/textinputs.go` | Erst prüfen, was Fyne 2.8 schon kann, und das Ergebnis berichten (Regel 9). Fehlendes ergänzen (Wort- und Zeilensprung, mit Shift für Auswahl). `Entry.SetMono(bool)`. |
| F5 | DateEntry: Kandidaten mit Ctrl+j/Ctrl+k wählen, Auswahl in der Liste hervorheben | S | `widgets/date_field.go`, `widgets/date_candidates.go`, `widgets/date_quick.go` | Ctrl+j/k wie Pfeil runter/hoch; der gewählte Kandidat ist in der Liste markiert (nicht nur über Farbe, z.B. Hintergrund + Marker). |
| F6 | Kontextmenü: rechts fast kein Padding | H | `widgets/menu.go` | Rechtes Padding = linkes. Golden aktualisieren. |

### Welle 1: sanity

| ID | Todo | Modell | Dateien | Ergebnis |
|---|---|---|---|---|
| S1 | Import verschiebt in die Inbox; Inbox überlebt Neustart; Zurücklegen; Neu-Sortieren abbrechen; Am Ort aufnehmen (E1–E3) | S | `inbox/queue.go`, `inbox/ops.go`, neu `inbox/origins.go`, `ui/presenter/sort.go` (`Import`, `Remove`, neue Methoden), `ui/presenter/library_ops.go` (`Resort`) | `Queue`/Presenter verschieben beim Import nach `office/inbox/` (Namenskonflikt: Suffix), Ursprung persistent. `Sort.PutBack()`, `Sort.CanPutBack()`, `Sort.AdoptInPlace(ctx)`. Nur Presenter und Backend, keine Views. Tests headless, inkl. Neustart-Szenario (neue Queue auf gleichem Ordner). |
| S2 | **Kopierbare relative Pfade relativ zum `office`-Ordner** | S | neu `domain/paths.go` (+Test), `ui/presenter/library_ops.go` (`CopyPaths`), `ui/presenter/sort.go` (nur `CopyPath`) | `domain.OfficeRel(projectRel string) string`: `receipts/2026/x.pdf` → `../receipts/2026/x.pdf`; `office/a.pdf` → `a.pdf`. Path-Template und gespeicherte Pfade bleiben relativ zum Projekt. Tests für beide Fälle und Windows-Trenner. |
| S3 | Betrag mit führendem Punkt (`.39000.00 EUR`) | S | Betragserkennung (`inbox/analyze.go` bzw. wo der Betrag geparst wird) + Tests | Ursache finden, allgemein lösen (Tausendertrenner, führende Satzzeichen). Regressionstest mit synthetischem Text, keine privaten PDFs einchecken. |
| S7 | Versionsnummer im Programm | H | neu `version.go`, neu `version_test.go` | `const Version = "0.2.0"`; Test prüft Gleichheit mit `FyneApp.toml` und mit der letzten Überschrift in `CHANGELOG.md`. |
| S9 | Icon: Dock-Variante weniger verschnörkelt, zwei konzentrische Motive/Ränder | S | `icon/gen.py`, Ausgabe `prj/icons/v0.2/` | 2–3 Varianten als SVG und PNG (1024, 128, 32) zur Auswahl (E6). `icon/` erst nach Wahl ersetzen (`task icon`). |

S1 und S2 ändern beide `presenter/sort.go` und `presenter/library_ops.go`, aber in getrennten Funktionen (S2 nur `CopyPath`/`CopyPaths`). Kein Umformatieren ganzer Dateien außer `gofmt`.

### Integration 1 (Orchestrator)

1. Berichte lesen; fine `CHANGELOG.md` und `docs/agents/reference.md` aus den gelieferten Texten ergänzen.
2. In fine: `task gallery:screenshots`, nur zugehörige Golden-Diffs behalten; `go vet ./...` und `go test ./...` in jedem Modul.
3. In sanity: `task check`.
4. Befunde nach `FINE-NOTES.md`.

### Welle 2: sanity-Bildschirme

| ID | Todo | Modell | Abhängig | Dateien | Ergebnis |
|---|---|---|---|---|---|
| S10 | Sort-Bildschirm: Löschen und Zurücklegen in der GUI und per Tastatur; Zoom mit Cmd++ / Cmd+−; Enter immer nächstes Feld (auch Typ → Datum), Shift+Enter voriges; neue Typen ohne Neustart | S | S1, F5 | `ui/sort_view.go`, `ui/sort_actions.go`, `ui/sort_vim.go`, `ui/pages.go`, `ui/presenter/keys.go` | Buttons „Zurücklegen“ / „Löschen“ (Löschen mit `ShowConfirm(danger)`), Befehle mit Shortcuts in der Palette, Vim-Tasten nach bestehendem Muster. Typen beim Anzeigen der Seite neu laden (`OnShow` + `Sort.LoadTypes`, wenn `Types` gespeichert hat). Fokuskette Typ → Datum → Rechnung → Betrag → Freitexte. |
| S4 | Types-Einstellungen: Datumsformat-Legende je Zeile (Jahr/Monat/Tag); „Choose folder“ rechts unter dem Template-Feld neben Valid/Beispiel; Source und Maximum age nebeneinander bei breitem Fenster; Mono in Pfadfeldern | S | F4 | `ui/types_view.go`, `ui/layout.go` | Legende als drei Zeilen (`2006` Jahr, `01` Monat, `02` Tag). Responsives Layout (Umbruch unter einer Breite). Alle Pfadfelder (Template, Inbox, Root) `SetMono(true)`, auch auf der Projektseite (`ui/project_view.go`, nur die Feld-Initialisierung). |
| S5 | Typ-Import-Dialog: Textarea sprengt den Dialog; Tabellen in eigener horizontaler Scroll-Area; viel Leerraum zwischen Dry-Run-Tabelle und Warnungen; Speichern-Buttons in den Footer | S | F3 | `ui/types_import_view.go` | `ShowDialog` mit „Speichern und bearbeiten“, „Speichern“, „Abbrechen“ im Footer; Textarea umbricht; Tabellen in `HScroll`; Warnungen mit `Label.Wrap`. |
| S6 | Shortcuts-Popup zu schmal; Button im Footer | H | F1, F3 | `ui/shortcuts_view.go` | Dialog mit `MinWidth` (z.B. 720) und zwei Spalten; Footer-Button (Icon + Label) öffnet es. Das Einhängen in den Footer macht S8 (besitzt `main.go`); S6 liefert `ShortcutsButton(sh) fyne.CanvasObject`. |
| S8 | Welcome-Ansicht beim Start; aktuelles Projekt als Dropdown rechts in der Toolbar; Version im Footer | S | F1, S7 (S9 optional) | `main.go`, `app.go`, neu `ui/welcome_view.go`, `ui/project_view.go` (außer Feld-Init, siehe S4), `ui/presenter/recents.go`, `ui/presenter/project.go` | Start ohne Argument zeigt nur die Welcome-Seite (großes Icon, zuletzt geöffnete Projekte, „Neues Projekt“, „Öffnen…“). Toolbar: `widgets.Select`/Menü mit Projektname, Wechsel über `switchTo`. Footer: `Config.Version = Version`, `SetTrailing(version, ShortcutsButton)`. |

### Integration 2 (Orchestrator)

1. `task check` in sanity, `go vet ./...`/`go test ./...` in fine (alle Module).
2. `CHANGELOG.md` von sanity unter `## v0.2.0` ergänzen (je Paket ein Bullet), Commit-Vorschläge machen, nicht committen.
3. `FINE-NOTES.md` ergänzen, offene Punkte aus den Berichten sammeln.
4. Testliste für dich (Dinge, die nur am echten Mac prüfbar sind): Cmd+Pfeile in Feldern, Cmd++ Zoom, Drop aus Finder
   landet in `office/inbox/`, Neustart mit vollem Eingang, Zurücklegen an den Ursprungsort, Dock-Icon.

## Regeln für parallele Agenten

- **Kein Worktree für sanity.** `go.work` ist gitignored; ein Worktree sähe die lokalen fine-Änderungen nicht. Alle
  Agenten arbeiten im selben Baum, getrennt über den Dateibesitz oben. fine-Agenten arbeiten direkt in `../..`.
- **Geteilte Dateien nur durch den Orchestrator:** `CHANGELOG.md` (beide Repos), `docs/agents/reference.md`,
  `FINE-NOTES.md`, `PLAN.md`.
- **Locales** (`locales/en.json`, `de.json`): Agenten fügen neue ids neben die bestehenden ids ihres Bereichs ein
  (nicht ans Dateiende), mit Präfix des Bereichs (`sort.`, `types.`, `welcome.`, `footer.`). Nie `shell.`/`command.`.
  Schlägt ein Edit wegen paralleler Änderung fehl: neu lesen, erneut einfügen.
- **Tests eng fassen** während der Arbeit (`go test ./inbox/ -run X`), weil andere Agenten gleichzeitig ändern.
  Der volle Lauf kommt in der Integration. Fremde Kompilierfehler nicht reparieren, im Bericht nennen.
- **Goldens:** nur mit `-run` auf den eigenen Test regenerieren; fremde Golden-Diffs zurücksetzen.
- Keine Commits, keine Tags, kein Push.

## Auftragsvorlage für einen Agenten

```
Du arbeitest an <sanity | fine> im Paket <ID> aus prj/plan-v0.2.0.md (lies dort deine Zeile und
"Regeln für parallele Agenten"). Lies zuerst AGENTS.md des Repos und den Skill <fine-app | fine-widget>.
Dateibesitz: <Dateien>. Andere Dateien nur lesen.
Ziel: <Ergebnis-Spalte>. Entscheidungen: <E-Nummern mit Wortlaut>.
Fertig, wenn: Unit-Tests für das Verhalten, eng gefasste Tests grün, go vet für dein Paket grün.
Bericht (kurz): geänderte Dateien; neue öffentliche API; CHANGELOG-Bullet; (fine) Text für
docs/agents/reference.md; Befunde für FINE-NOTES; was nur manuell prüfbar ist; Abweichungen vom Plan.
```

Aufruf: `Agent(model: "sonnet" | "haiku", prompt: <Vorlage>)`, alle Pakete einer Welle in einer Nachricht.
