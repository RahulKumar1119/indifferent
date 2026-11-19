#!/usr/bin/env python3
"""Checks, on real renders, what caption fitting (#327) promises. Used by caption_fit_check.sh.

  caption_fit_verify.py jobs   <dir>   write <dir>/jobs.txt: one "name|filter|seconds" per frame to render
  caption_fit_verify.py check  <dir>   read the frames in <dir>/out and the manifest, print a report, exit 1 on a failure

The frames are 1080x1920, a flat grey picture with the captions burned in by ffmpeg's subtitles
filter (libass) using the fonts the render images install. Nothing here needs the repository's code:
the numbers come from the pixels.

What is checked:
  clipping     every caption the layout says fits has no ink closer than 24 px to either side, and
               nothing is cut off at the frame edge. This is the promise of the fix.
  estimate     the width the layout used is never narrower than the ink it produced (plus the outline),
               so a "fits" is never wrong the dangerous way; the report says how loose the estimate is.
  sweep        a karaoke cue that wraps lights its words in order: the first line is lit before the
               second begins, and by the end both lines are fully lit.
  equivalence  a preset drawn from an ASS file (used only when one cue in a file is too wide) looks the
               same as the same preset drawn from SRT: at most 1.5% of the picture differs.
"""
import json
import pathlib
import sys

from PIL import Image, ImageChops

W, H = 1080, 1920
BG = (0x2A, 0x2A, 0x2A)
EDGE = 16  # px kept clear on each side of an ASS caption (the layout keeps 20 by its own measure)
SRT_EDGE = 14  # libass wraps SRT at a 28 px margin, and a box draws outside it


def load(d):
    return json.loads((pathlib.Path(d) / "manifest.json").read_text())


def jobs(d):
    lines = []
    for c in load(d):
        if c["kind"] == "layout":
            f = f"subtitles={c['file']}" + (f":force_style='{c['style']}'" if c.get("style") else "")
            lines.append(f"{c['id']}|{f}|{c['at']:.2f}")
        elif c["kind"] == "sweep":
            for i, t in enumerate(c["times"]):
                lines.append(f"{c['id']}-{i}|subtitles={c['file']}|{t}")
        elif c["kind"] == "equivalence":
            lines.append(f"{c['id']}-srt|subtitles={c['file']}:force_style='{c['style']}'|1.0")
            lines.append(f"{c['id']}-ass|subtitles={c['other']}|1.0")
    (pathlib.Path(d) / "jobs.txt").write_text("\n".join(lines) + "\n")
    print(len(lines), "frames")


def frame(d, name):
    return Image.open(pathlib.Path(d) / "out" / f"{name}.png").convert("RGB")


def ink_box(im):
    diff = ImageChops.difference(im, Image.new("RGB", im.size, BG)).convert("L")
    return diff.point(lambda v: 255 if v > 24 else 0).getbbox()


def check(d):
    cases = load(d)
    bad, ratios, clip_checked = [], [], 0
    sweeps = equiv = 0
    for c in cases:
        k = c["kind"]
        if k == "layout":
            box = ink_box(frame(d, c["id"]))
            label = f"{c['id']} {c['preset']}/{c['font'] or 'default'}/{c['units']}/{c['text']} ({c['file'].rsplit('.', 1)[1]})"
            if box is None:
                bad.append(f"{label}: no caption drawn")
                continue
            left, right = box[0], box[2]
            touches = left <= 1 or right >= W - 1
            if c["fits"]:
                clip_checked += 1
                if touches:
                    bad.append(f"{label}: cut off at the frame edge ({left}..{right}), though the layout said it fits")
                else:
                    edge = EDGE if c["file"].endswith(".ass") else SRT_EDGE
                    if left < edge or right > W - edge:
                        bad.append(f"{label}: ink at {left}..{right}, closer than {edge}px to an edge")
                ink = right - left
                # An ASS file breaks lines where the layout says. For SRT, libass breaks lines itself,
                # so the width to compare with is the whole cue on one line.
                est = (c["estPx"] if c["file"].endswith(".ass") else c["singlePx"]) + 2 * c["outlinePx"]
                if ink > est + 6:
                    bad.append(f"{label}: ink {ink}px is wider than the estimate {est:.0f}px")
                else:
                    ratios.append((ink / est, label))
            else:
                # Nothing could make it fit (a word wider than the frame at the smallest size). Only report.
                print(f"  note: {label} cannot fit at the smallest size (ink {left}..{right})")
        elif k == "sweep":
            sweeps += 1
            yellow = []
            for i, t in enumerate(c["times"]):
                im = frame(d, f"{c['id']}-{i}")
                box = ink_box(im)
                if box is None:
                    bad.append(f"{c['id']} sweep at {t}s: no caption drawn")
                    yellow.append((0, 0))
                    continue
                mid = (box[1] + box[3]) // 2
                counts = []
                for y0, y1 in ((box[1], mid), (mid, box[3])):
                    px = im.crop((0, y0, W, y1)).getdata()
                    counts.append(sum(1 for r, g, b in px if r > 200 and g > 170 and b < 120))
                yellow.append(tuple(counts))
            # Lit pixels only ever go up, the second line is not lit before the first has some light,
            # and at the end both lines are lit.
            for i in range(1, len(yellow)):
                if sum(yellow[i]) + 50 < sum(yellow[i - 1]):
                    bad.append(f"{c['id']} sweep: the lit text shrank between {c['times'][i-1]}s and {c['times'][i]}s {yellow}")
            first_second_lit = next((i for i, y in enumerate(yellow) if y[1] > 200), None)
            first_first_lit = next((i for i, y in enumerate(yellow) if y[0] > 200), None)
            if first_first_lit is None or yellow[-1][0] < 200 or yellow[-1][1] < 200:
                bad.append(f"{c['id']} sweep at size {c['units']}: not both lines lit at the end {yellow}")
            elif first_second_lit is not None and first_second_lit < first_first_lit:
                bad.append(f"{c['id']} sweep: the second line lit before the first {yellow}")
            else:
                print(f"  sweep size {c['units']}: lit pixels (line1, line2) by time {c['times']}: {yellow}")
        elif k == "equivalence":
            equiv += 1
            a, b = frame(d, f"{c['id']}-srt"), frame(d, f"{c['id']}-ass")
            diff = ImageChops.difference(a, b).convert("L").point(lambda v: 255 if v > 60 else 0)
            changed = sum(1 for v in diff.getdata() if v) / (W * H)
            ba, bb = ink_box(a), ink_box(b)
            label = f"{c['id']} {c['preset']}/{c['font'] or 'default'}/{c['text']}"
            if changed > 0.015:
                bad.append(f"{label}: SRT and ASS differ in {changed*100:.2f}% of the picture; boxes {ba} vs {bb}")
            elif abs(ba[1] - bb[1]) > 3 or abs(ba[3] - bb[3]) > 3 or abs(ba[0] - bb[0]) > 4 or abs(ba[2] - bb[2]) > 4:
                bad.append(f"{label}: ink boxes differ: {ba} vs {bb}")
    print(f"clipping checked on {clip_checked} captions, sweep on {sweeps}, SRT/ASS equivalence on {equiv}")
    if ratios:
        ratios.sort()
        print(f"estimate vs ink: tightest {ratios[-1][0]:.3f}, loosest {ratios[0][0]:.3f} ({ratios[0][1]}), median {ratios[len(ratios)//2][0]:.3f}")
    for b in bad:
        print("FAIL", b)
    print("OK" if not bad else f"{len(bad)} FAILED")
    sys.exit(1 if bad else 0)


if __name__ == "__main__":
    if len(sys.argv) != 3 or sys.argv[1] not in ("jobs", "check"):
        sys.exit(__doc__)
    {"jobs": jobs, "check": check}[sys.argv[1]](sys.argv[2])
