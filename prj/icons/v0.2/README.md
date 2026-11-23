# Icon-Varianten v0.2 (Auswahl, E6)

Die große Dock-Version des Stempels war zu verschnörkelt (Ticks, Textzeilen, mehrere Ringe, Tinten-Textur).
Alle drei Varianten behalten das Datumsband "23.05" und die leichte Schräglage, reduzieren den Stempel aber auf
**zwei konzentrische Ränder** (ein kräftiger Außenring plus ein innerer Rand) und lassen Textur, Ticks und
Textzeilen weg. Farben und Squircle sind unverändert.

| Variante | Datei | Innerer Rand | Charakter |
|---|---|---|---|
| A Doppelring | `icon-a-doppelring.svg` | dünner Vollring | Am ruhigsten und am saubersten, ähnelt am ehesten einem Siegel. |
| B Perlenkranz | `icon-b-perlenkranz.svg` | 30 große Perlen | Weicher und verspielter, bleibt klar als Stempel erkennbar. |
| C Perforation | `icon-c-perforation.svg` | 40 kräftige Striche | Erinnert an eine Briefmarken- bzw. Stempelperforation, am lebendigsten. |

Jede Variante gibt es als SVG und als PNG in 1024, 128 und 32 px (`icon-<name>-<größe>.png`). Es gibt keine
separate Small-Variante; alle Größen kommen aus derselben SVG. Bei 32 px bleiben A und B klar, bei C verschmelzen
die Striche zu einem Rauschen am Rand.

## Neu erzeugen

```sh
cd icon && python3 gen.py --variants     # schreibt die drei SVGs hierher
rsvg-convert -w 1024 icon-a-doppelring.svg -o icon-a-doppelring-1024.png   # usw. für 128 und 32
```

## Übernehmen

`icon/icon-large.svg` und `icon/icon.png` sind unverändert. Nach der Wahl: Motiv in `icon/gen.py` als
`large_motif()` (und ggf. `small_motif()`) übernehmen, dann `task icon`.
