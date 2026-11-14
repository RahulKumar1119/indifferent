#!/usr/bin/env python3
"""Checks the font files the #217 spike used, with fontTools (not the browser).

  1. Every character on the sample infographics is really in the file the export
     took it from (so no empty box, and the CSS ranges told the truth).
  2. Every shipped regular and bold woff2 of the three packages says it is under
     the SIL Open Font License, and each package ships its licence text.

Usage: python3 check_fonts.py [results.json]   (after: node run.mjs)
Needs: pip install fonttools brotli
"""
import glob
import json
import os
import sys

from fontTools.ttLib import TTFont

HERE = os.path.dirname(os.path.abspath(__file__))
PACKAGES = os.path.join(HERE, "node_modules", "@fontsource")
RESULTS = sys.argv[1] if len(sys.argv) > 1 else os.path.expanduser("~/shots303/spike/results.json")
# Characters that are meant to have no font (see run.mjs EXPECTED_UNCOVERED).
EXPECTED_MISSING = {0x9F18}
MARKERS = ("open font license", "openfontlicense.org", "scripts.sil.org/ofl")

failures = []
cache = {}


def font(name):
    if name not in cache:
        found = glob.glob(os.path.join(PACKAGES, "*", "files", name))
        if len(found) != 1:
            failures.append(f"{name}: expected one file, found {len(found)}")
            cache[name] = None
        else:
            cache[name] = TTFont(found[0], lazy=True)
    return cache[name]


# 1. Glyph coverage of what the export embedded.
results = json.load(open(RESULTS))
checked = 0
for case in results["cases"]:
    for cp, weight, filename in case["perChar"]:
        if filename is None:
            if cp not in EXPECTED_MISSING:
                failures.append(f"{case['lang']} {case['size']}: U+{cp:04X} has no font file")
            continue
        f = font(filename)
        if f is None:
            continue
        checked += 1
        if cp not in f.getBestCmap():
            failures.append(f"{case['lang']} {case['size']}: U+{cp:04X} is not in {filename}")
print(f"coverage: {checked} character/file pairs checked across {len(results['cases'])} renders")

# 2. Licence of everything that could ship (regular and bold).
files = []
for pkg in sorted(os.listdir(PACKAGES)):
    files += sorted(glob.glob(os.path.join(PACKAGES, pkg, "files", "*-400-normal.woff2")))
    files += sorted(glob.glob(os.path.join(PACKAGES, pkg, "files", "*-700-normal.woff2")))
    text = os.path.join(PACKAGES, pkg, "LICENSE")
    if not os.path.isfile(text) or "open font license" not in open(text, errors="ignore").read().lower():
        failures.append(f"{pkg}: no SIL Open Font License text in LICENSE")
for path in files:
    name = os.path.basename(path)
    table = TTFont(path, lazy=True)["name"]
    said = " ".join((table.getDebugName(i) or "") for i in (13, 14)).lower()
    if not any(m in said for m in MARKERS):
        failures.append(f"{name}: licence fields do not name the OFL ({said.strip()[:60]!r})")
print(f"licence: {len(files)} regular and bold woff2 files, {len(os.listdir(PACKAGES))} packages")

if failures:
    print("\nFAILED:\n  " + "\n  ".join(failures[:30]))
    sys.exit(1)
print("All font checks passed.")
