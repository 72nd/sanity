#!/usr/bin/env python3
"""Pack PNG files into icon.icns (stdlib only; `task icon:icns` calls this).

An icns file is the magic `icns`, a total length and a list of chunks (type, length, PNG data), all
big-endian. This does what `iconutil -c icns` does for a PNG-only iconset, without needing macOS.

usage: mkicns.py OUT.icns DIR   (DIR holds one PNG per pixel size, named <size>.png)
"""
import pathlib
import struct
import sys

# pixel size -> chunk types (a 2x image of a smaller point size has its own type)
TYPES = {16: ["icp4"], 32: ["icp5", "ic11"], 64: ["icp6", "ic12"], 128: ["ic07"],
         256: ["ic08", "ic13"], 512: ["ic09", "ic14"], 1024: ["ic10"]}


def main(out, src):
    chunks = []
    for size, types in sorted(TYPES.items()):
        data = (pathlib.Path(src) / f"{size}.png").read_bytes()
        for t in types:
            chunks.append(t.encode() + struct.pack(">I", len(data) + 8) + data)
    body = b"".join(chunks)
    pathlib.Path(out).write_bytes(b"icns" + struct.pack(">I", len(body) + 8) + body)


if __name__ == "__main__":
    main(*sys.argv[1:3])
