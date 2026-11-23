# Icon-Varianten v0.3 (Auswahl, E6)

Dritte Runde nach dem Feedback, v0.2 und v0.2b seien zu abstrakt. Ausgangspunkt ist wieder das bestehende
Stempel-Icon (`icon/icon.png`, `prj/icons/history/07-final`) mit seinem Reichtum: 7-Segment-Datum "23.05" in
schräger Plakette, Tintenflecken, Violett-Verlauf, Textzeilen. Reduziert ist nur der Verschnörkelungsgrad: statt
vier dünner Kreise gibt es **zwei konzentrische Motive**, einen kräftigen Außenring und einen Motivring innen,
mit größeren, weniger Elementen und mehr Luft. Die Tintenspritzer des alten Icons sind beibehalten.

| Variante | Datei | Aufbau | Gewichtung |
|---|---|---|---|
| A Strichring | `icon-a-strichring.svg` | Außenring, 48 Striche, Plakette, 4 Textzeilen | Am nächsten am alten Icon, nur luftiger. |
| B Datum groß | `icon-b-datum-gross.svg` | Außenring, 40 Striche, größere Plakette, keine Textzeilen | Das Datum dominiert, am ruhigsten und am besten lesbar. |
| C Perlen | `icon-c-perlen.svg` | Außenring, 28 Perlen, Plakette, 4 Textzeilen | Weicher und verspielter, Perlen leicht unterschiedlich groß. |

Dateien je Variante: `icon-<name>.svg` (für 128 px und größer), `icon-<name>-small.svg` (ohne Tintenspritzer und
Textzeilen, dickerer Ring, weniger und dickere Striche bzw. Perlen; für 32 px) und `icon-<name>-{1024,128,32}.png`.
Das 32er PNG kommt aus der Small-SVG, 1024 und 128 aus der großen.

## Neu erzeugen

```sh
cd icon && python3 gen.py --variants     # schreibt die SVGs nach prj/icons/v0.3/
rsvg-convert -w 1024 icon-a-strichring.svg -o icon-a-strichring-1024.png   # 128 analog
rsvg-convert -w 32 icon-a-strichring-small.svg -o icon-a-strichring-32.png
```

## Übernehmen

`icon/icon.png` und `icon/icon-*.svg` sind unverändert. Nach der Wahl: Motiv als `large_motif()` bzw.
`small_motif()` in `icon/gen.py` übernehmen (die Lambdas stehen in `VARIANTS`), dann `task icon`.
