#!/usr/bin/env python3
"""Generate the bold stamp icons (no paper): one large detailed and one small simplified SVG per palette."""
import math, pathlib

# name: (bg top, bg bottom, ink)
PALETTES = {
    "tomato":      ("#ff7a45", "#d12f1c", "#fff3df"),
    "ultramarine": ("#5b7bff", "#2326b8", "#fff3df"),
    "violet":      ("#b86bff", "#5e1ab8", "#ffeaf6"),
    "emerald":     ("#1fd3a6", "#07705f", "#f4fff0"),
    "sunny":       ("#ffdc5a", "#f29a00", "#b3240f"),
}
C = 512
OUT = pathlib.Path(__file__).parent / "_out"


def ticks(n, r0, r1, w):
    out = []
    for i in range(n):
        a = math.radians(i * 360 / n)
        out.append(f'<line x1="{C + r0*math.cos(a):.1f}" y1="{C + r0*math.sin(a):.1f}" '
                   f'x2="{C + r1*math.cos(a):.1f}" y2="{C + r1*math.sin(a):.1f}" stroke-width="{w}"/>')
    return "".join(out)


def star(cx, cy, r):
    pts = []
    for i in range(10):
        a = math.radians(-90 + i * 36)
        rr = r if i % 2 == 0 else r * 0.42
        pts.append(f"{cx + rr*math.cos(a):.1f},{cy + rr*math.sin(a):.1f}")
    return f'<polygon points="{" ".join(pts)}" stroke="none"/>'


def defs(top, bottom, texture):
    ink = ""
    if texture:
        ink = ('<filter id="ink" x="-5%" y="-5%" width="110%" height="110%">'
               '<feTurbulence type="fractalNoise" baseFrequency="0.05" numOctaves="3" seed="11" result="n"/>'
               '<feColorMatrix in="n" type="matrix" values="0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 -26 19.6" result="m"/>'
               '<feComposite in="SourceGraphic" in2="m" operator="in"/></filter>')
    return (f'<defs><linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">'
            f'<stop offset="0" stop-color="{top}"/><stop offset="1" stop-color="{bottom}"/></linearGradient>'
            f'<radialGradient id="glow" cx="0.5" cy="0.12" r="0.7"><stop offset="0" stop-color="#fff" stop-opacity="0.28"/>'
            f'<stop offset="1" stop-color="#fff" stop-opacity="0"/></radialGradient>'
            f'<filter id="sh" x="-10%" y="-10%" width="120%" height="125%">'
            f'<feDropShadow dx="0" dy="14" stdDeviation="14" flood-color="#000" flood-opacity="0.32"/></filter>{ink}</defs>')


def wrap(top, bottom, texture, body):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">'
            f'{defs(top, bottom, texture)}'
            f'<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#bg)" filter="url(#sh)"/>'
            f'<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#glow)"/>{body}</svg>')


def large(top, bottom, ink):
    body = (f'<g transform="rotate(-10 {C} {C})" fill="none" stroke="{ink}" filter="url(#ink)" stroke-linecap="round">'
            f'<circle cx="{C}" cy="{C}" r="332" stroke-width="40"/>'
            f'<circle cx="{C}" cy="{C}" r="298" stroke-width="7"/>'
            f'{ticks(72, 262, 284, 6)}'
            f'<circle cx="{C}" cy="{C}" r="240" stroke-width="14"/>'
            f'<g fill="{ink}">{star(C-108, C+152, 24)}{star(C, C+172, 30)}{star(C+108, C+152, 24)}</g>'
            f'<path d="M{C-120} {C+0} L{C-34} {C+88} L{C+128} {C-120}" stroke-width="84" stroke-linejoin="round"/>'
            f'</g>')
    return wrap(top, bottom, True, body)


def small(top, bottom, ink):
    body = (f'<g transform="rotate(-10 {C} {C})" fill="none" stroke="{ink}" stroke-linecap="round" stroke-linejoin="round">'
            f'<circle cx="{C}" cy="{C}" r="318" stroke-width="76"/>'
            f'<path d="M{C-125} {C+10} L{C-30} {C+105} L{C+135} {C-125}" stroke-width="104"/></g>')
    return wrap(top, bottom, False, body)


if __name__ == "__main__":
    OUT.mkdir(parents=True, exist_ok=True)
    for name, (top, bottom, ink) in PALETTES.items():
        (OUT / f"stamp-large-{name}.svg").write_text(large(top, bottom, ink))
        (OUT / f"stamp-small-{name}.svg").write_text(small(top, bottom, ink))
