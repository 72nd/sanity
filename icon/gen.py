#!/usr/bin/env python3
"""Generate the sanity app icon as SVG (stdlib only; `task icon` calls this).

Two variants of one motif, a date stamp ("received 23.05."):
  icon-large.svg  detailed, with ink texture; used for 128 px and up
  icon-small.svg  ring and date band only; used for 64 px and below
The design history lives in prj/icons/README.md.
"""
import math
import pathlib

OUT = pathlib.Path(__file__).parent

BG_TOP, BG_BOTTOM, INK = "#2fd0b8", "#0b5a63", "#eafff8"  # petrol (v0.2.0)
DATE = "23.05"
C = 512  # canvas is 1024; the macOS squircle is 824 wide with a 100 px margin
TILT = -10  # the stamp sits slightly crooked

# Seven-segment digits: segment -> (x1, y1, x2, y2) in a unit box.
SEGMENTS = {"a": (0, 0, 1, 0), "b": (1, 0, 1, .5), "c": (1, .5, 1, 1), "d": (0, 1, 1, 1),
            "e": (0, .5, 0, 1), "f": (0, 0, 0, .5), "g": (0, .5, 1, .5)}
DIGITS = {"0": "abcdef", "1": "bc", "2": "abged", "3": "abgcd", "4": "fgbc",
          "5": "afgcd", "6": "afgedc", "7": "abc", "8": "abcdefg", "9": "abcdfg"}


def digit(ch, x, y, w, h, sw):
    """One digit as black strokes, for the knock-out mask."""
    lines = []
    for s in DIGITS[ch]:
        x1, y1, x2, y2 = SEGMENTS[s]
        lines.append(f'<line x1="{x + x1*w:.1f}" y1="{y + y1*h:.1f}" x2="{x + x2*w:.1f}" y2="{y + y2*h:.1f}" '
                     f'stroke="#000" stroke-width="{sw}" stroke-linecap="round"/>')
    return "".join(lines)


def band(clip_r, top, bottom, half, w, h, sw, step, gap):
    """A solid band with the date knocked out. `step` is the digit pitch, `gap` the room for the dot."""
    d = DATE.replace(".", "")
    pair = step + w
    xs = [-gap / 2 - pair, -gap / 2 - pair + step, gap / 2, gap / 2 + step]
    y = C + top + (bottom - top - h) / 2
    holes = "".join(digit(c, C + x, y, w, h, sw) for c, x in zip(d, xs))
    holes += f'<circle cx="{C}" cy="{y + h}" r="{sw * .62:.1f}" fill="#000"/>'
    return (f'<clipPath id="inner"><circle cx="{C}" cy="{C}" r="{clip_r}"/></clipPath>'
            f'<mask id="date" maskUnits="userSpaceOnUse" x="0" y="0" width="1024" height="1024">'
            f'<rect width="1024" height="1024" fill="#fff"/>{holes}</mask>'
            f'<rect x="{C - half}" y="{C + top}" width="{2 * half}" height="{bottom - top}" rx="10" '
            f'clip-path="url(#inner)" mask="url(#date)"/>')


def ring(r, sw):
    return f'<circle cx="{C}" cy="{C}" r="{r}" fill="none" stroke-width="{sw}"/>'


def ticks(n, r0, r1, sw):
    out = []
    for i in range(n):
        a = math.radians(i * 360 / n)
        out.append(f'<line x1="{C + r0*math.cos(a):.1f}" y1="{C + r0*math.sin(a):.1f}" '
                   f'x2="{C + r1*math.cos(a):.1f}" y2="{C + r1*math.sin(a):.1f}" stroke-width="{sw}"/>')
    return "".join(out)


def bars(specs):
    """Rounded text-line bars; specs are (y offset, width)."""
    return "".join(f'<rect x="{C - w / 2}" y="{C + y}" width="{w}" height="18" rx="9"/>' for y, w in specs)


def large_motif():
    return MOTIF[0]()


def small_motif():
    return MOTIF[1]()


def svg(motif, texture, colors=None):
    bg_top, bg_bottom, ink = colors or (BG_TOP, BG_BOTTOM, INK)
    ink_filter = ""
    if texture:  # speckles let the background through, like a real impression
        ink_filter = ('<filter id="ink" x="-5%" y="-5%" width="110%" height="110%">'
                      '<feTurbulence type="fractalNoise" baseFrequency="0.05" numOctaves="3" seed="11" result="n"/>'
                      '<feColorMatrix in="n" type="matrix" values="0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 -26 19.6" result="m"/>'
                      '<feComposite in="SourceGraphic" in2="m" operator="in"/></filter>')
    filt = ' filter="url(#ink)"' if texture else ""
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">'
        '<defs>'
        f'<linearGradient id="bg" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="{bg_top}"/>'
        f'<stop offset="1" stop-color="{bg_bottom}"/></linearGradient>'
        '<radialGradient id="glow" cx="0.5" cy="0.12" r="0.7"><stop offset="0" stop-color="#fff" stop-opacity="0.28"/>'
        '<stop offset="1" stop-color="#fff" stop-opacity="0"/></radialGradient>'
        '<filter id="sh" x="-10%" y="-10%" width="120%" height="125%">'
        '<feDropShadow dx="0" dy="14" stdDeviation="14" flood-color="#000" flood-opacity="0.32"/></filter>'
        f'{ink_filter}</defs>'
        '<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#bg)" filter="url(#sh)"/>'
        '<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#glow)"/>'
        f'<g transform="rotate({TILT} {C} {C})" fill="{ink}" stroke="{ink}" stroke-linecap="round"{filt}>{motif}</g>'
        '</svg>')


def dots(n, r, dr):
    out = []
    for i in range(n):
        a = math.radians(i * 360 / n)
        rr = dr * (1 + .12 * math.sin(i * 2.7))  # beads differ slightly, like a worn die
        out.append(f'<circle cx="{C + r*math.cos(a):.1f}" cy="{C + r*math.sin(a):.1f}" r="{rr:.1f}" stroke="none"/>')
    return "".join(out)


def dashes(n, r0, r1, sw):
    return ticks(n, r0, r1, sw)


def uneven_ring(r, sw, dx, dy):
    """A ring whose inner edge is off-centre: thicker on one side, like uneven stamp pressure."""
    ro, ri = r + sw / 2, r - sw / 2
    return (f'<path stroke="none" fill-rule="evenodd" d="M{C - ro} {C}a{ro} {ro} 0 1 0 {2*ro} 0a{ro} {ro} 0 1 0 {-2*ro} 0z'
            f'M{C + dx - ri} {C + dy}a{ri} {ri} 0 1 0 {2*ri} 0a{ri} {ri} 0 1 0 {-2*ri} 0z"/>')


def arcs(r, sw, n, gap):
    """Ring broken into n arcs; gap is the share of each segment left empty."""
    seg = 2 * math.pi * r / n
    return (f'<circle cx="{C}" cy="{C}" r="{r}" fill="none" stroke-width="{sw}" stroke-linecap="butt" '
            f'stroke-dasharray="{seg * (1 - gap):.1f} {seg * gap:.1f}"/>')


def stamp_svg(motif, strong):
    """Stamp look: wobbly rough edges plus uneven ink coverage. strong=False for 32 px (no grain, mild wobble)."""
    if strong:
        filt = ('<filter id="ink" x="-5%" y="-5%" width="110%" height="110%">'
                '<feTurbulence type="fractalNoise" baseFrequency="0.03" numOctaves="2" seed="4" result="w"/>'
                '<feDisplacementMap in="SourceGraphic" in2="w" scale="4" xChannelSelector="R" yChannelSelector="G" result="d1"/>'
                '<feTurbulence type="fractalNoise" baseFrequency="0.6" numOctaves="2" seed="9" result="e"/>'
                '<feDisplacementMap in="d1" in2="e" scale="5" xChannelSelector="R" yChannelSelector="G" result="d2"/>'
                # coarse blotches (low frequency) decide how much ink is on a spot, fine grain punches the pits
                '<feTurbulence type="fractalNoise" baseFrequency="0.012 0.02" numOctaves="2" seed="21" result="b"/>'
                '<feTurbulence type="fractalNoise" baseFrequency="0.45" numOctaves="2" seed="5" result="g"/>'
                '<feComposite in="b" in2="g" operator="arithmetic" k1="0" k2="0.55" k3="0.45" k4="0" result="bg"/>'
                '<feColorMatrix in="bg" type="matrix" values="0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  -10 0 0 0 6.6" result="m"/>'
                '<feComposite in="d2" in2="m" operator="in"/></filter>')
    else:
        filt = ('<filter id="ink" x="-5%" y="-5%" width="110%" height="110%">'
                '<feTurbulence type="fractalNoise" baseFrequency="0.03" numOctaves="2" seed="4" result="w"/>'
                '<feDisplacementMap in="SourceGraphic" in2="w" scale="8" xChannelSelector="R" yChannelSelector="G"/></filter>')
    return svg(motif, False).replace('</defs>', filt + '</defs>', 1).replace(
        'stroke-linecap="round">', 'stroke-linecap="round" filter="url(#ink)">', 1)


def date_band():
    return band(236, -62, 62, 216, 60, 88, 18, 84, 64)


# v0.4: the chosen motif (v0.3 "c-perlen") in four colour pairs (background top, background bottom, ink).
MOTIF = (
    lambda: ring(326, 34) + dots(28, 264, 20) + band(222, -56, 56, 204, 60, 86, 17, 80, 60) + bars(
        [(-136, 220), (-102, 140), (90, 140), (124, 220)]),
    lambda: ring(322, 62) + dots(20, 258, 28) + band(220, -70, 70, 204, 62, 94, 20, 88, 70),
)

COLORS = {
    "indigo": ("#6f8dff", "#1c2f9e", "#eef2ff"),
    "petrol": ("#2fd0b8", "#0b5a63", "#eafff8"),
    "orange": ("#f27a2c", "#b02a04", "#fff1e0"),
    "anthrazit": ("#4b525c", "#14171c", "#ffc233"),
}


def write_variants(out):
    out.mkdir(parents=True, exist_ok=True)
    large, small = MOTIF
    for name, colors in COLORS.items():
        (out / f"icon-{name}.svg").write_text(svg(large(), True, colors))
        (out / f"icon-{name}-small.svg").write_text(svg(small(), False, colors))


if __name__ == "__main__" and "--variants" in __import__("sys").argv:
    write_variants(OUT.parent / "prj" / "icons" / "v0.4")
elif __name__ == "__main__":
    (OUT / "icon-large.svg").write_text(svg(large_motif(), True))
    (OUT / "icon-small.svg").write_text(svg(small_motif(), False))
