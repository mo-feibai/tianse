#!/usr/bin/env python3
"""Subset the two bundled fonts down to what the UI can actually show.

Run from the repo root:  python scripts/subset-fonts.py

Sources live in assets/fonts/src (not committed, ~62 MB). Outputs go to
frontend/public/fonts as woff2 and are committed.

Two different strategies, because the two families do different jobs:

* KingHwa OldSong is the display face: only headings, the wordmark and digits
  ever appear in it, so it is subset to the characters found in the sources
  plus the ASCII/punctuation set. ~200 glyphs.
* LXGW WenKai Screen is the text face: it has to render every label the user
  might see, including city names and file paths, so it keeps GB2312 level 1
  (3755 hanzi) plus ASCII and CJK punctuation. ~3900 glyphs.

--layout-features='*' keeps tnum/fwid/zero, which DESIGN 3.4 relies on for
tabular digits.
"""

import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
SRC = ROOT / "assets" / "fonts" / "src"
OUT = ROOT / "frontend" / "public" / "fonts"
TMP = ROOT / ".build" / "font-chars"

KINGHWA = SRC / "KingHwaOldSong.ttf"
LXGW = SRC / "LXGWWenKaiScreen.ttf"

# Always keep these: the UI writes digits, units and separators in both faces.
ALWAYS = (
    "0123456789"
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
    " .,:;!?/\\|-_+=()[]{}<>@#$%^&*\"'"
    "\u00b0\u00b7\u2014\u2018\u2019\u201c\u201d\u2026\u3001\u3002\uff0c\uff1a\uff1b\uff01\uff1f\uff08\uff09\u300a\u300b\u00d7\u2192\u2190"
)


def gb2312_level1() -> str:
    chars = []
    for hi in range(0xB0, 0xD8):
        for lo in range(0xA1, 0xFF):
            try:
                chars.append(bytes([hi, lo]).decode("gb2312"))
            except UnicodeDecodeError:
                pass
    return "".join(chars)


def source_chars() -> str:
    """Every character that appears in a .vue/.ts/.html source file."""
    seen = set()
    for pat in ("**/*.vue", "**/*.ts", "**/*.html"):
        for p in (ROOT / "frontend" / "src").glob(pat):
            seen.update(p.read_text(encoding="utf-8"))
        for p in (ROOT / "frontend").glob(pat):
            seen.update(p.read_text(encoding="utf-8"))
    return "".join(sorted(seen))


def city_chars() -> str:
    """City names, from the one Go file that owns them.

    The display face renders the first-run sentence, which interpolates the city
    name, so every selectable city must exist in the display subset. Only
    cities.go is scanned: warning strings and log lines also reach the UI, but
    they render in the text face.
    """
    seen = set()
    p = ROOT / "internal" / "config" / "cities.go"
    for line in p.read_text(encoding="utf-8").splitlines():
        for part in line.split('"')[1::2]:
            seen.update(part)
    return "".join(sorted(seen))


def dedupe(s: str) -> str:
    out, seen = [], set()
    for ch in s:
        if ch in seen:
            continue
        seen.add(ch)
        out.append(ch)
    return "".join(out)


def subset(src: pathlib.Path, text: str, dst: pathlib.Path, name: str) -> None:
    TMP.mkdir(parents=True, exist_ok=True)
    txt = TMP / (name + ".txt")
    txt.write_text(text, encoding="utf-8")
    cmd = [
        sys.executable, "-m", "fontTools.subset", str(src),
        "--text-file=" + str(txt),
        "--output-file=" + str(dst),
        "--flavor=woff2",
        "--layout-features=*",
        "--no-subset-tables+=DSIG",
        "--name-IDs=*",
        "--drop-tables+=vhea,vmtx",
    ]
    subprocess.run(cmd, check=True)
    print(f"{dst.name}: {len(text)} chars -> {dst.stat().st_size / 1024:.0f} KB")


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    if not KINGHWA.exists() or not LXGW.exists():
        sys.exit(f"missing source fonts in {SRC}")

    display = dedupe(ALWAYS + source_chars() + city_chars())
    subset(KINGHWA, display, OUT / "kinghwa-oldsong-subset.woff2", "kinghwa")

    text = dedupe(ALWAYS + gb2312_level1() + source_chars())
    subset(LXGW, text, OUT / "lxgw-wenkai-screen-subset.woff2", "lxgw")


if __name__ == "__main__":
    main()
