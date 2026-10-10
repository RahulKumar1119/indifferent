# DESIGN.md

Status: DRAFT written by the agent for owner review. Nothing here is approved until the owner edits and accepts it.

Warning: agent-written direction tends toward default AI taste, which is the slop antislop filters. This draft is built from what the codebase already uses (colors, type, README, blog topics) instead of new taste. Sections marked Owner are left for the owner to decide or correct.

## Identity

Indifferent (indifferent.fun) is a studio with two pipelines and one free tool, per the README:

- Quiz films: a TXT file of multiple-choice questions becomes a narrated MP4.
- AI Shorts Generator: a long video or audio file (up to 10 minutes) becomes ranked 9:16 clips with burned-in captions.
- Watermark tool: free in-browser image watermarking. Nothing uploads.

Audience (Owner to confirm): inferred only from the blog topics, which cover quiz videos for YouTube and educational video. Likely people who make quiz or educational videos, and creators who repurpose long videos. No visitor data was used.

## Personality

Draft, taken from the existing type and palette: editorial and warm, closer to a printed page than a dashboard. Calm, plain-spoken, specific. Owner to edit the traits.

## Palette

Two or three core colors plus one accent (R-29).

| Role | Hex | Uses in frontend code |
|---|---|---|
| Paper (base) | #FAF7F2 | 42 |
| Paper, second surface | #F4EFE6 | 18 |
| Ink (text) | #1A1714 | 117 |
| Muted text | #6B6560 | 163 |
| Deep green (second core) | #1E3A2A | 29 |
| Accent: terracotta | #BC5227 | 66 |
| Accent, lighter step for dark backgrounds | #D96C3D | 50 |

The accent belongs at the key moment only: the primary action and the active step. Not on every icon, badge, link and border.

Contrast, computed with contrast-check.py (WCAG 2.x). Text color on background:

| Pair | Ratio | Normal text (4.5) | Large text (3.0) |
|---|---|---|---|
| #1A1714 on #FAF7F2 | 16.70 | Pass | Pass |
| #6B6560 on #FAF7F2 | 5.38 | Pass | Pass |
| #6B6560 on #F4EFE6 | 5.02 | Pass | Pass |
| #BC5227 on #FAF7F2 | 4.49 | Fail | Pass |
| White on #BC5227 | 4.80 | Pass | Pass |
| #D96C3D on #1A1714 | 5.24 | Pass | Pass |
| #FAF7F2 on #1E3A2A | 11.60 | Pass | Pass |
| #FAF7F2 on #1A1714 | 16.70 | Pass | Pass |
| #6366F1 on #FAF7F2 | 4.18 | Fail | Pass |

Rule from the results: terracotta #BC5227 on paper misses 4.5:1 by 0.01 as small text. On paper, use it for large text and button fills, or use a darker step for small text. White text on a #BC5227 fill passes.

Status colors exist as tokens (success #10b981, warning #f59e0b, error #f43f5e). Every status also needs text or an icon, never color alone.

Conflicts to resolve (Owner decision):

- The token --color-primary #6366f1 (indigo) and the theme-color #7c3aed (purple) are not part of the palette above. Indigo on paper is 4.18:1 and fails as small text. Both read as the default AI palette (R-01).
- The "Aurora gradient background" in styles.css, and 40 gradient uses across the frontend, each need a written reason or removal (R-01).

## Typography

- Display: Cormorant Garamond. Reason: a high-contrast serif gives the editorial, printed-page feel, and it already appears in 13 font-family declarations.
- UI and body: Outfit. Reason: a clean geometric sans that pairs with the serif, and it already appears in 19 font-family declarations plus the body rule.
- Inter stays only as a fallback in the font stack.

## Theme

Owner decision (R-21). The codebase has a light paper theme and dark values (#0F0E0B, #141310). Either ship a working light and dark toggle, or commit to one theme. Every theme shipped must pass the contrast and focus checks (R-34).

## Mood

Owner to fill in: sites you admire, and sites you do not want to look like. Left blank on purpose. No references were invented.

## Dials

Dial: ENERGY 2 / RHYTHM 2 / MOTION 1

- ENERGY 2: the site has to explain two tools and earn trust, so it is confident but not experimental.
- RHYTHM 2: sections stay consistent with a few deliberate breaks, so pages are not one repeated template (R-05).
- MOTION 1: hover states and real progress feedback only. Both tools are task-driven, and motion should show job progress, not decorate.

These are drafts for the owner to change.

## Measured starting point (counts, not audit findings)

Counted in frontend/src on the day this draft was written:

- backdrop-blur: 25 occurrences (R-10 caps glass at 1 or 2 elements)
- gradient occurrences: 40 (R-01)
- animate-pulse: 2 (R-19)
- lucide mentions in .ts files: 301 (R-04)
- .ts and .html files containing an em dash: 30 (R-02)

Per-page work decides what to change. A count is not a verdict.

## Open decisions

1. Audience and mood (Owner).
2. Keep, change or drop the indigo and purple tokens.
3. Light only, or light and dark.
4. Radius, shadow and icon set are not decided yet. Each choice needs a written reason (R-31) before it is used.
