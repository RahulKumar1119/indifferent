#!/usr/bin/env python3
"""Checks, on real renders, what the hook layer (#329) promises. Used by hook_fit_check.sh.

  hook_fit_verify.py jobs  <dir>          write <dir>/jobs.txt: one "name|filter|seconds" per frame
  hook_fit_verify.py check <dir> [sheet]  read the frames in <dir>/out, print a report, exit 1 on a
                                          failure; with sheet, also write a contact sheet PNG there

The frames are 1080x1920, a flat grey picture with the captions and the hook burned in by ffmpeg's
subtitles filter (libass) in the production render image, in the order the renderer uses. The
numbers come from the pixels.

What is checked, for every case:
  drawn      the hook is drawn, and so are the captions when there are any
  inside     the hook's ink stays 16 px clear of each side, below the top bar (160 px) and above the
             platforms' buttons (the bottom 288 px), and is not cut off at an edge
  estimate   the hook's ink is never wider than the layout's estimate, and stays inside the block
             (box or outline included) the layout placed, so the measuring errs the safe way
  captions   the hook's ink and the captions' ink do not overlap, with at least 8 px between them,
             and the captions' ink stays inside the band the layout kept clear for them
  same       burning both layers draws the hook exactly as burning the hook alone (the captions do
             not cover it)
  shaping    (fade cases) a Hindi hook with conjuncts and the short i vowel sign differs widely
             from the same file drawn with simple shaping, so they are shaped
And for the fade cases: no hook at 0 s, part of it 0.1 s in and 0.1 s before the end, all of it at
1 s, and none after the end.
"""
import json
import pathlib
import sys

from PIL import Image, ImageChops

W, H = 1080, 1920
BG = (0x2A, 0x2A, 0x2A)
TOP_SAFE, BOTTOM_SAFE = 160, 288
EDGE = 16
GAP = 8
SLACK = 4


def load(d):
    return json.loads((pathlib.Path(d) / "manifest.json").read_text())


def caps_filter(c):
    return c.get("capsFilter", "")


def jobs(d):
    lines = []
    for c in load(d):
        hook = c["hookFilter"]
        caps = caps_filter(c)
        lines.append(f"{c['id']}-hook|{hook}|1.00")
        if caps:
            lines.append(f"{c['id']}-caps|{caps}|1.00")
            lines.append(f"{c['id']}-both|{caps},{hook}|1.00")
        if c["fade"]:
            for name, t in fade_times(c):
                lines.append(f"{c['id']}-fade-{name}|{hook}|{t:.2f}")
            # The same hook with simple shaping, to prove complex shaping is in effect.
            lines.append(f"{c['id']}-simple|ass={c['hookFile']}:shaping=simple|1.00")
    (pathlib.Path(d) / "jobs.txt").write_text("\n".join(lines) + "\n")
    print(len(lines), "frames")


def fade_times(c):
    s = c["seconds"]
    return [("start", 0.0), ("in", 0.1), ("full", 1.0), ("out", s - 0.1), ("after", s + 0.1)]


def frame(d, name):
    return Image.open(pathlib.Path(d) / "out" / f"{name}.png").convert("RGB")


def diff_of(im):
    return ImageChops.difference(im, Image.new("RGB", im.size, BG)).convert("L")


def ink_box(im):
    return diff_of(im).point(lambda v: 255 if v > 24 else 0).getbbox()


def energy(im):
    # The encoder moves the flat grey by a level or two, so differences under 5 are the background.
    return sum(v for v in diff_of(im).getdata() if v > 4)


def check(d, sheet=None):
    cases = load(d)
    bad = []
    widths = []
    shaped = []
    fades = 0
    for c in cases:
        label = f"{c['id']} {c['lang']}/{c['style']}/{c['position']} with {c['captions']}"
        hook_im = frame(d, c["id"] + "-hook")
        hb = ink_box(hook_im)
        if hb is None:
            bad.append(f"{label}: no hook drawn")
            continue
        left, top, right, bottom = hb
        if left <= 1 or right >= W - 1 or top <= 1 or bottom >= H - 1:
            bad.append(f"{label}: hook cut off at the frame edge {hb}")
        if left < EDGE or right > W - EDGE:
            bad.append(f"{label}: hook ink {left}..{right} closer than {EDGE}px to a side")
        if top < TOP_SAFE - SLACK or bottom > H - BOTTOM_SAFE + SLACK:
            bad.append(f"{label}: hook ink {top}..{bottom} outside {TOP_SAFE}..{H - BOTTOM_SAFE}")
        ink_w = right - left
        if ink_w > c["estW"] + 6:
            bad.append(f"{label}: hook ink {ink_w}px wider than the estimate {c['estW']:.0f}px")
        else:
            widths.append((ink_w / c["estW"], label))
        if top < c["blockY0"] - SLACK or bottom > c["blockY1"] + SLACK:
            bad.append(f"{label}: hook ink {top}..{bottom} outside its block {c['blockY0']:.0f}..{c['blockY1']:.0f}")
        if c.get("capsFile"):
            cb = ink_box(frame(d, c["id"] + "-caps"))
            if cb is None:
                bad.append(f"{label}: no captions drawn")
            else:
                if cb[1] < c["capsY0"] - SLACK or cb[3] > c["capsY1"] + SLACK:
                    bad.append(f"{label}: caption ink {cb[1]}..{cb[3]} outside its band {c['capsY0']:.0f}..{c['capsY1']:.0f}")
                if not (bottom + GAP <= cb[1] or cb[3] + GAP <= top):
                    bad.append(f"{label}: hook {top}..{bottom} and captions {cb[1]}..{cb[3]} overlap")
            both = frame(d, c["id"] + "-both")
            region = (left, top, right, bottom)
            changed = ImageChops.difference(both.crop(region), hook_im.crop(region)).convert("L").point(lambda v: 255 if v > 40 else 0)
            if changed.getbbox() is not None:
                bad.append(f"{label}: the captions draw over the hook at {changed.getbbox()}")
        if c["fade"]:
            simple = frame(d, c["id"] + "-simple")
            moved = ImageChops.difference(simple, hook_im).convert("L").point(lambda v: 255 if v > 40 else 0)
            changed = sum(1 for v in moved.getdata() if v)
            # Conjuncts and the short i vowel sign (drawn before its letter) move a lot of ink when
            # shaped; English moves only by kerning (about 1,300 px at most here).
            if c["lang"] in ("hi80", "hiConj") and changed < 5000:
                bad.append(f"{label}: Hindi with conjuncts drawn nearly the same with simple shaping ({changed} px differ): complex shaping is not in effect")
            shaped.append((label, changed))
            fades += 1
            e = {name: energy(frame(d, f"{c['id']}-fade-{name}")) for name, _ in fade_times(c)}
            full = e["full"] or 1
            r = {k: v / full for k, v in e.items()}
            if r["start"] > 0.05 or r["after"] > 0.02 or not (0.15 < r["in"] < 0.85) or not (0.15 < r["out"] < 0.85):
                bad.append(f"{label}: fade {r}")
    print(f"{len(cases)} hook cases checked, {fades} with fades and the shaping check")
    for label, n in shaped:
        print(f"  shaping: {label}: {n} px differ from simple shaping")
    if widths:
        widths.sort()
        print(f"hook ink vs estimate: tightest {widths[-1][0]:.3f} ({widths[-1][1]}), loosest {widths[0][0]:.3f}, median {widths[len(widths)//2][0]:.3f}")
    if sheet:
        contact_sheet(d, cases, sheet)
    for b in bad:
        print("FAIL", b)
    if bad:
        print(f"{len(bad)} failures")
        sys.exit(1)
    print("all checks passed")


def contact_sheet(d, cases, path):
    want = [c for c in cases if c["lang"] in ("en80", "hi80", "hiConj") and c["captions"] in ("bold", "karaoke-top-24", "animated-mid", "minimal-mid-72")]
    want.sort(key=lambda c: (c["captions"], c["lang"], c["style"], c["position"]))
    tw, th, cols = 216, 384, 12
    rows = (len(want) + cols - 1) // cols
    sheet = Image.new("RGB", (cols * tw, rows * th), (0, 0, 0))
    for i, c in enumerate(want):
        name = c["id"] + ("-both" if c.get("capsFile") else "-hook")
        im = frame(d, name).resize((tw, th))
        sheet.paste(im, ((i % cols) * tw, (i // cols) * th))
    sheet.save(path)
    print("contact sheet:", path, len(want), "frames")


if __name__ == "__main__":
    cmd, d = sys.argv[1], sys.argv[2]
    if cmd == "jobs":
        jobs(d)
    else:
        check(d, sys.argv[3] if len(sys.argv) > 3 else None)
