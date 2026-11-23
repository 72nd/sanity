# App icon

A date stamp ("received 23.05.") in petrol, with a ring, a bead wreath and a date plaque. Two variants of one motif:

| File | Used for |
|---|---|
| `icon-large.svg` | 128 px and up: ring, bead wreath, date plaque, text lines, ink texture |
| `icon-small.svg` | 64 px and below: ring, thicker beads and date plaque only |
| `icon.png` | 1024 px, the large variant. Embedded in the binary (`icon.go`) and the `Icon` of `FyneApp.toml` |
| `icon.icns` | All macOS sizes, large and small variants mixed |

All of them are generated and checked in, so a plain `go build` needs no tools.

## Build

```sh
task icon       # gen.py -> SVGs -> PNGs -> icon.png + icon.icns
task package    # Sanity.app; swaps in icon.icns (fyne only knows icon.png)
```

- Needs `python3` and `rsvg-convert` (`brew install librsvg`). No macOS-only tools; `mkicns.py` writes the icns itself.
- Change the look in `gen.py` (colors, date, geometry), then run `task icon` and commit the generated files.
- `task package` needs the fyne CLI: `go install fyne.io/tools/cmd/fyne@latest`.
- Why the swap in `package`: `fyne package` builds the icns from `icon.png` alone, which would use the detailed
  variant at 16 and 32 px. The hand-made file uses the small one there.

Design history: [`prj/icons/README.md`](../prj/icons/README.md).
