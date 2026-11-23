# Icon-Varianten v0.2b (Auswahl, E6)

Überarbeitung nach dem Feedback "zu clean, kein Stempel". Die erste Runde (`../v0.2/`) bleibt zum Vergleich liegen.

Gemeinsam: zwei konzentrische Ränder um das Datumsband "23.05" (Blockschrift, Schräglage -10 Grad), keine Ticks,
keine Textzeilen. Stempelcharakter durch:

- leicht wellige, raue Kanten (Verzerrungsfilter),
- ungleichmäßige Tintendeckung: grobe helle Flecken plus feine Körnung,
- Außenring mit ungleichmäßiger Stärke (innerer Rand leicht außermittig).

| Variante | Datei | Innerer Rand | Charakter |
|---|---|---|---|
| A Doppelring | `icon-a-doppelring.svg` | dünner Vollring | Am ruhigsten, klassisches Siegel. |
| B Perlenkranz | `icon-b-perlenkranz.svg` | 30 Perlen unterschiedlicher Größe | Weicher, verspielter. |
| C Segmentring | `icon-c-segmentring.svg` | Ring aus 24 Bögen mit Lücken | Wirkt wie ein abgenutzter Stempel, am technischsten. |

Dateien je Variante: `icon-<name>.svg` (volle Textur, für 128 px und größer), `icon-<name>-small.svg` (nur leichte
Kantenunruhe, ohne Körnung und Flecken, für 32 px) sowie `icon-<name>-{1024,128,32}.png`. Das 32er PNG kommt aus
der Small-SVG, 1024 und 128 aus der großen.

## Neu erzeugen

```sh
cd icon && python3 gen.py --variants     # schreibt die SVGs nach prj/icons/v0.2b/
rsvg-convert -w 1024 icon-a-doppelring.svg -o icon-a-doppelring-1024.png   # 128 analog
rsvg-convert -w 32 icon-a-doppelring-small.svg -o icon-a-doppelring-32.png
```

## Übernehmen

`icon/icon.png` und `icon/icon-*.svg` sind unverändert. Nach der Wahl: Motiv als `large_motif()` (mit `stamp_svg`)
und als `small_motif()` in `icon/gen.py` verdrahten, dann `task icon`.
