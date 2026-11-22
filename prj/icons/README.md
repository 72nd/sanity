# Icon design history

How the sanity app icon came about. The production icon lives in [`icon/`](../../icon/README.md); this folder only
documents the way there. Every stage has an `overview.png` (and dock-size comparisons where they mattered), the
hand-written SVGs of the early stages, and the generator scripts of the later ones.

Requirements throughout: works in the macOS Dock (squircle on a 1024 px canvas, 824 px body), one dominant motif, a
simplified variant for small sizes.

| Stage | What was tried | Outcome |
|---|---|---|
| [01 first concepts](history/01-first-concepts) | Five concepts proposed: check receipt, **stamp**, stack with arrow, key press, **folder tab**. Stamp and tab were drawn. | Stamp and tab were the favorites. |
| [02 combination](history/02-combination) | Stamped paper inside a folder (vivid cyan and orange), then a friendlier stamp tool with muted green. | Too "Windows" in color. The simplified variant without the stamp tool was better. |
| [03 paper and colors](history/03-paper-and-colors) | Stamp on a tilted sheet; large detailed variant (ticks, dots, ink texture) and a simplified one. Five background colors compared in the Dock. | Direction fixed: stamp, two variants. Wanted: bolder colors, no paper. |
| [04 bold, no paper](history/04-bold-no-paper) | The stamp fills the whole icon. Palettes tomato, ultramarine, violet, emerald, sunny. | Tomato and violet liked. Small stars looked like footballs. |
| [05 motifs](history/05-motifs) | A scalloped seal with a date band, laurel, or guilloche rosette. | Date band (an "Eingangsstempel") is the most distinctive idea. |
| [06 no check](history/06-no-check) | Without the check mark: date band with laurel (D), large date band with text lines (E), receipt glyph (F). Then five outer rings for E: plain, double, stitched (stamp perforation), zigzag, pearls. | **E with the double ring**, in violet. |
| [07 final](history/07-final) | Date set to 23.05, hand-made icns. | Shipped, see `icon/`. |

## Decisions worth remembering

- **Two variants, one motif.** The large one (128 px and up) has the double ring, tick band, text lines and ink
  texture. The small one (64 px and below) is only a thick ring and the date band: at 32 px the details turn into noise.
- **No paper, no folder.** The stamp is the icon. Paper and folder cost space and made the stamp small.
- **Color.** Violet is rare in the Dock, the cream ink keeps strong contrast, and it avoids the cyan/orange
  "Windows" look of stage 02. Tomato was the runner-up.
- **No check mark, no stars.** The check was a generic "done" sign; the stars read as footballs. The date carries the
  idea (the date is a central step of the filing workflow).
- **The date is fixed to 23.05.** It is only a motif, not a live date.
- **Own icns writer.** `iconutil` is macOS only (and failed inside the agent sandbox). The icns format is a header
  plus PNG chunks, so `icon/mkicns.py` writes it with the standard library.

## Reproducing stages 04 to 06

```sh
cd prj/icons/history/generators
python3 gen.py           # stage 04: palettes, written to _out/ (git-ignored)
python3 gen_motifs.py    # stage 05: motif studies A, B, C
python3 gen_nocheck.py   # stage 06: variants D, E, F and the outer-ring studies
```

`gen_motifs.py` and `gen_nocheck.py` import `gen.py`, so keep the three together. The final generator is
standalone: `icon/gen.py`.
