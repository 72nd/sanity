#!/usr/bin/env python3
"""Motif studies for the large stamp: A scalloped seal + date band, B scalloped seal + laurel, C guilloche + laurel."""
import math, pathlib
from gen import PALETTES, C, ticks, defs

OUT = pathlib.Path(__file__).parent / "_out" / "motifs"
OUT.mkdir(parents=True, exist_ok=True)


def polar(r_fn, n=720):
    pts = []
    for i in range(n):
        a = 2 * math.pi * i / n
        r = r_fn(a)
        pts.append(f"{C + r*math.cos(a):.1f},{C + r*math.sin(a):.1f}")
    return "M" + " L".join(pts) + " Z"


def scallop(R, A, n):
    return polar(lambda a: R + A * abs(math.sin(n * a / 2)))


def wave(R, A, n, phase):
    return polar(lambda a: R + A * math.sin(n * a + phase))


SEG = {"a": (0, 0, 1, 0), "b": (1, 0, 1, .5), "c": (1, .5, 1, 1), "d": (0, 1, 1, 1),
       "e": (0, .5, 0, 1), "f": (0, 0, 0, .5), "g": (0, .5, 1, .5)}
DIGIT = {"1": "bc", "2": "abged"}


def digit(ch, x, y, w, h, sw):
    out = []
    for s in DIGIT[ch]:
        x1, y1, x2, y2 = SEG[s]
        out.append(f'<line x1="{x + x1*w:.1f}" y1="{y + y1*h:.1f}" x2="{x + x2*w:.1f}" y2="{y + y2*h:.1f}" '
                   f'stroke="#000" stroke-width="{sw}" stroke-linecap="round"/>')
    return "".join(out)


def date_band(rin):
    w, h, sw = 46, 66, 13
    y = C + 42
    digs = "".join(digit(c, C + dx, y, w, h, sw) for c, dx in (("2", -132), ("2", -68), ("1", 26), ("1", 90)))
    digs += f'<circle cx="{C-1}" cy="{y+h}" r="8" fill="#000" stroke="none"/>'
    top, bot = C + 18, C + 132
    return (f'<clipPath id="inner"><circle cx="{C}" cy="{C}" r="{rin}"/></clipPath>'
            f'<mask id="digits" maskUnits="userSpaceOnUse" x="0" y="0" width="1024" height="1024">'
            f'<rect width="1024" height="1024" fill="#fff"/>{digs}</mask>'
            f'<rect x="{C-rin}" y="{top}" width="{2*rin}" height="{bot-top}" fill="currentColor" stroke="none" '
            f'clip-path="url(#inner)" mask="url(#digits)"/>')


def laurel(r=172):
    out = []
    for side in (1, -1):
        out.append("<g>")
        steps = 7
        for i in range(steps):
            t = i / (steps - 1)
            ang = math.radians(100 + t * 82)
            ang = math.pi - ang if side == -1 else ang  # mirror
            px, py = C + r * math.cos(ang), C + r * math.sin(ang)
            tang = ang + math.pi / 2 * (1 if side == 1 else -1)
            rot = math.degrees(tang)
            size = 1.0 - 0.25 * t
            for off, sgn in ((-3, -1), (3, 1)):
                lx, ly = px + sgn * 16 * math.cos(ang), py + sgn * 16 * math.sin(ang)
                out.append(f'<ellipse cx="{lx:.1f}" cy="{ly:.1f}" rx="{27*size:.1f}" ry="{12*size:.1f}" '
                           f'transform="rotate({rot + sgn*32:.1f} {lx:.1f} {ly:.1f})" fill="currentColor" stroke="none"/>')
        a0, a1 = (100, 182)
        pts = []
        for k in range(30):
            ang = math.radians(a0 + (a1 - a0) * k / 29)
            ang = math.pi - ang if side == -1 else ang
            pts.append(f"{C + r*math.cos(ang):.1f},{C + r*math.sin(ang):.1f}")
        out.append(f'<path d="M{" L".join(pts)}" fill="none" stroke-width="7"/></g>')
    return "".join(out)


def check(dx=0, dy=0, s=1.0, sw=84):
    return (f'<path d="M{C+(-120+dx)*s:.0f} {C+(0+dy)*s:.0f} L{C+(-34+dx)*s:.0f} {C+(88+dy)*s:.0f} '
            f'L{C+(128+dx)*s:.0f} {C+(-120+dy)*s:.0f}" stroke-width="{sw}" stroke-linejoin="round"/>')


def FILLED(x):
    return x.startswith(('<clipPath', '<rect', '<g>'))


def build(variant, top, bottom, ink):
    scal = variant in "AB"
    g = []
    if scal:
        g.append(f'<path d="{scallop(296, 14, 40)}" stroke-width="34" stroke-linejoin="round"/>')
        g.append(f'<circle cx="{C}" cy="{C}" r="262" stroke-width="7"/>')
        g.append(ticks(72, 232, 250, 6))
        g.append(f'<circle cx="{C}" cy="{C}" r="216" stroke-width="12"/>')
        rin = 208
    else:
        g.append(f'<circle cx="{C}" cy="{C}" r="332" stroke-width="40"/>')
        g.append(f'<circle cx="{C}" cy="{C}" r="298" stroke-width="7"/>')
        g.append(f'<path d="{wave(268, 13, 14, 0)}" stroke-width="6"/>')
        g.append(f'<path d="{wave(268, 13, 14, math.pi/14*1)}" stroke-width="6"/>')
        g.append(f'<path d="{wave(268, 13, 14, math.pi/7*1)}" stroke-width="6"/>')
        g.append(f'<circle cx="{C}" cy="{C}" r="240" stroke-width="14"/>')
        rin = 232
    if variant == "A":
        g.append(date_band(rin))
        g.append(check(0, -88, 0.78, 66))
    elif variant == "B":
        g.append(laurel(160)); g.append(check(0, -12, 0.74, 64))
    else:
        g.append(laurel(176)); g.append(check(0, -12, 0.82, 70))
    strokes = [x for x in g if not FILLED(x)]
    fills = [x for x in g if FILLED(x)]
    body = (f'<g transform="rotate(-10 {C} {C})" fill="{ink}" stroke="{ink}" color="{ink}" filter="url(#ink)" stroke-linecap="round">'
            f'<g fill="none">{"".join(strokes)}</g>{"".join(fills)}</g>')
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">{defs(top, bottom, True)}'
            f'<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#bg)" filter="url(#sh)"/>'
            f'<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#glow)"/>{body}</svg>')


if __name__ == "__main__":
    for name in ("tomato", "violet"):
        top, bottom, ink = PALETTES[name]
        for v in "ABC":
            (OUT / f"{v}-{name}.svg").write_text(build(v, top, bottom, ink))
