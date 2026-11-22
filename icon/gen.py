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

BG_TOP, BG_BOTTOM, INK = "#b86bff", "#5e1ab8", "#ffeaf6"
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
    return "".join([
        ring(318, 26), ring(284, 9),            # double outer ring
        ring(262, 7), ticks(72, 232, 250, 6),   # fine ring and tick band
        ring(216, 12),                          # inner ring
        band(208, -52, 52, 190, 58, 80, 16, 74, 56),
        bars([(-132, 230), (-100, 150), (86, 150), (118, 230)]),
    ])


def small_motif():
    return "".join([ring(322, 64), band(268, -78, 78, 262, 62, 100, 20, 96, 100)])


def svg(motif, texture):
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
        f'<linearGradient id="bg" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="{BG_TOP}"/>'
        f'<stop offset="1" stop-color="{BG_BOTTOM}"/></linearGradient>'
        '<radialGradient id="glow" cx="0.5" cy="0.12" r="0.7"><stop offset="0" stop-color="#fff" stop-opacity="0.28"/>'
        '<stop offset="1" stop-color="#fff" stop-opacity="0"/></radialGradient>'
        '<filter id="sh" x="-10%" y="-10%" width="120%" height="125%">'
        '<feDropShadow dx="0" dy="14" stdDeviation="14" flood-color="#000" flood-opacity="0.32"/></filter>'
        f'{ink_filter}</defs>'
        '<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#bg)" filter="url(#sh)"/>'
        '<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#glow)"/>'
        f'<g transform="rotate({TILT} {C} {C})" fill="{INK}" stroke="{INK}" stroke-linecap="round"{filt}>{motif}</g>'
        '</svg>')


if __name__ == "__main__":
    (OUT / "icon-large.svg").write_text(svg(large_motif(), True))
    (OUT / "icon-small.svg").write_text(svg(small_motif(), False))
