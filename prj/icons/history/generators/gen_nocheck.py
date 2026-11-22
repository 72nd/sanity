#!/usr/bin/env python3
"""Stamp studies without the check mark: D date band + laurel, E big date band + bars, F receipt glyph + laurel."""
import pathlib
from gen import PALETTES, C, ticks, defs
from gen_motifs import scallop, digit, laurel, FILLED

OUT = pathlib.Path(__file__).parent / "_out" / "nocheck"
OUT.mkdir(parents=True, exist_ok=True)


def band(rin, top, bot, half, xs, w, h, sw, dot_x=0):
    y = C + top + (bot - top - h) / 2
    digs = "".join(digit(c, C + dx, y, w, h, sw) for c, dx in zip("2211", xs))
    digs += f'<circle cx="{C+dot_x}" cy="{y+h}" r="{sw*0.62:.1f}" fill="#000" stroke="none"/>'
    return (f'<clipPath id="inner"><circle cx="{C}" cy="{C}" r="{rin}"/></clipPath>'
            f'<mask id="digits" maskUnits="userSpaceOnUse" x="0" y="0" width="1024" height="1024">'
            f'<rect width="1024" height="1024" fill="#fff"/>{digs}</mask>'
            f'<rect x="{C-half}" y="{C+top}" width="{2*half}" height="{bot-top}" rx="10" fill="currentColor" stroke="none" '
            f'clip-path="url(#inner)" mask="url(#digits)"/>')


def bars(specs):
    return "".join(f'<rect x="{C-w/2}" y="{C+y}" width="{w}" height="18" rx="9" fill="currentColor" stroke="none"/>'
                   for y, w in specs)


def receipt():
    x0, x1, y0, y1, f = -62, 62, -118, 70, 38
    p = (f'M{C+x0} {C+y0} H{C+x1-f} L{C+x1} {C+y0+f} V{C+y1} H{C+x0} Z')
    corner = f'M{C+x1-f} {C+y0} V{C+y0+f} H{C+x1}'
    lines = "".join(f'<line x1="{C-34}" y1="{C+y}" x2="{C+34 - (18 if i==2 else 0)}" y2="{C+y}" stroke-width="14"/>'
                    for i, y in enumerate((-40, -4, 32)))
    return f'<path d="{p}" stroke-width="20" stroke-linejoin="round"/><path d="{corner}" stroke-width="16" stroke-linejoin="round"/>{lines}'


def build(variant, top, bottom, ink):
    g = [f'<path d="{scallop(296, 14, 40)}" stroke-width="34" stroke-linejoin="round"/>',
         f'<circle cx="{C}" cy="{C}" r="262" stroke-width="7"/>', ticks(72, 232, 250, 6),
         f'<circle cx="{C}" cy="{C}" r="216" stroke-width="12"/>']
    rin = 208
    if variant == "D":
        g.append(laurel(160))
        g.append(band(rin, -62, 38, 128, (-104, -50, 30, 84), 40, 66, 13, dot_x=-3))
    elif variant == "E":
        g.append(band(rin, -52, 52, 190, (-152, -76, 18, 94), 58, 80, 16))
        g.append(bars([(-132, 230), (-100, 150), (86, 150), (118, 230)]))
    else:
        g.append(laurel(160)); g.append(receipt())
    strokes = [x for x in g if not FILLED(x)]
    fills = [x for x in g if FILLED(x)]
    body = (f'<g transform="rotate(-10 {C} {C})" fill="{ink}" stroke="{ink}" color="{ink}" filter="url(#ink)" stroke-linecap="round">'
            f'<g fill="none">{"".join(strokes)}</g>{"".join(fills)}</g>')
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">{defs(top, bottom, True)}'
            f'<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#bg)" filter="url(#sh)"/>'
            f'<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#glow)"/>{body}</svg>')


import math


def outer_ring(kind):
    if kind == "plain":
        return f'<circle cx="{C}" cy="{C}" r="312" stroke-width="40"/>'
    if kind == "double":
        return (f'<circle cx="{C}" cy="{C}" r="318" stroke-width="26"/>'
                f'<circle cx="{C}" cy="{C}" r="284" stroke-width="9"/>')
    if kind == "stitch":
        return f'<circle cx="{C}" cy="{C}" r="314" stroke-width="34" stroke-dasharray="34 17.8" stroke-linecap="butt"/>'
    if kind == "zigzag":
        pts = []
        for k in range(96):
            a = 2 * math.pi * k / 96
            r = 326 if k % 2 == 0 else 296
            pts.append(f"{C + r*math.cos(a):.1f},{C + r*math.sin(a):.1f}")
        return f'<path d="M{" L".join(pts)} Z" stroke-width="12" stroke-linejoin="miter"/>'
    if kind == "pearls":
        n = 56
        return "".join(f'<circle cx="{C + 312*math.cos(2*math.pi*k/n):.1f}" cy="{C + 312*math.sin(2*math.pi*k/n):.1f}" r="15" fill="currentColor" stroke="none"/>'
                       for k in range(n))


def build_e(kind, top, bottom, ink):
    g = [outer_ring(kind), f'<circle cx="{C}" cy="{C}" r="262" stroke-width="7"/>', ticks(72, 232, 250, 6),
         f'<circle cx="{C}" cy="{C}" r="216" stroke-width="12"/>',
         band(208, -52, 52, 190, (-152, -76, 18, 94), 58, 80, 16),
         bars([(-132, 230), (-100, 150), (86, 150), (118, 230)])]
    strokes = [x for x in g if not FILLED(x) and "fill=\"currentColor\"" not in x]
    fills = [x for x in g if FILLED(x) or "fill=\"currentColor\"" in x]
    body = (f'<g transform="rotate(-10 {C} {C})" fill="{ink}" stroke="{ink}" color="{ink}" filter="url(#ink)" stroke-linecap="round">'
            f'<g fill="none">{"".join(strokes)}</g>{"".join(fills)}</g>')
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">{defs(top, bottom, True)}'
            f'<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#bg)" filter="url(#sh)"/>'
            f'<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#glow)"/>{body}</svg>')


if __name__ == "__main__":
    for name in ("tomato", "violet"):
        top, bottom, ink = PALETTES[name]
        for v in "DEF":
            (OUT / f"{v}-{name}.svg").write_text(build(v, top, bottom, ink))
    outer = OUT / "outer"
    outer.mkdir(parents=True, exist_ok=True)
    for kind in ("plain", "double", "stitch", "zigzag", "pearls"):
        for name in ("tomato", "violet"):
            top, bottom, ink = PALETTES[name]
            (outer / f"E-{kind}-{name}.svg").write_text(build_e(kind, top, bottom, ink))
