# Icon-Farbvarianten v0.4

Gewähltes Motiv: `../v0.3/icon-c-perlen` (Außenring, Perlenkranz, Datumsplakette 23.05, Textzeilen, Tintenspritzer).
Hier nur andere Farbpaare: Hintergrundverlauf des Squircles (oben hell, unten dunkel) und passende Stempelfarbe.
Übersicht mit Dock-Größen (128 und 32 px): [`overview.png`](overview.png).

| Variante | Datei | Hintergrund | Stempel |
|---|---|---|---|
| Indigo | `icon-indigo.svg` | Blau nach Dunkelindigo (`#6f8dff` nach `#1c2f9e`) | Kühles Weiß (`#eef2ff`) |
| Petrol | `icon-petrol.svg` | Türkis nach Petrol (`#2fd0b8` nach `#0b5a63`) | Minzweiß (`#eafff8`) |
| Orange | `icon-orange.svg` | Orange nach Rotorange (`#f27a2c` nach `#b02a04`) | Warmes Creme (`#fff1e0`) |
| Anthrazit | `icon-anthrazit.svg` | Grau nach fast Schwarz (`#4b525c` nach `#14171c`) | Gelb-Gold (`#ffc233`) als Akzent |

Dateien je Variante: `icon-<name>.svg` (ab 128 px), `icon-<name>-small.svg` (32 px, ohne Spritzer und Textzeilen) und
`icon-<name>-{1024,128,32}.png`. Das 32er PNG kommt aus der Small-SVG.

Anthrazit ist die einzige Variante mit farbigem statt hellem Stempel und hebt sich im Dock am stärksten ab. Bei Orange
ist der Kontrast der hellen Stempelfarbe am oberen Rand am geringsten.

## Neu erzeugen

```sh
cd icon && python3 gen.py --variants     # schreibt die SVGs nach prj/icons/v0.4/ (Farben: COLORS in gen.py)
rsvg-convert -w 1024 icon-indigo.svg -o icon-indigo-1024.png   # 128 analog; 32 aus icon-indigo-small.svg
```

`icon/icon.png` und die violette Produktion bleiben unverändert. Nach der Wahl: Farben und Motiv in `icon/gen.py`
übernehmen (`BG_TOP`, `BG_BOTTOM`, `INK`, `large_motif()`), dann `task icon`.
