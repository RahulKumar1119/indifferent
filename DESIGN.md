# DESIGN.md

Status: DRAFT v2 for owner approval (issue #244, part of #243). The direction below comes from the owner's answers on #243. Everything marked "Proposed" is a decision made in this draft and needs the owner's yes. Nothing else in the redesign sub-issues (#245 to #256) starts until this is approved.

The machine-readable half of this document is `frontend/src/tokens.css`. If the two disagree, fix the one that is wrong and say so in the PR.

## Identity

Indifferent (indifferent.fun) is a creator studio that turns written or recorded material into short narrated video:

- Story to video: paste a story, get a plan of 2 to 10 scenes with stock media and narration, render up to 90 seconds.
- AI Shorts: a long video or audio file (up to 1 hour) becomes ranked 9:16 clips with burned-in captions and optional AI thumbnails.
- Quiz videos: a TXT file of multiple-choice questions becomes a narrated MP4.
- Watermark tool: free, in the browser, nothing uploads.

## Audience (owner, #243)

Beginners and experienced creators who want engaging short-form video without advanced editing skills: YouTube Shorts, Reels and TikTok creators; storytellers and writers; educators and quiz makers; motivational, history and mythology channels; people repurposing long content; social media managers.

## Direction (owner, #243)

Technical Editorial. Premium, minimal, strong typography, generous whitespace, confident and creator-focused. No generic AI landing-page look, no excess gradients, no oversized rounded cards, no decoration that does not help.

Two related treatments:

- Public pages: expressive and editorial, conversion-led, one clear action per page.
- App screens (dashboard, editors, workspace): quieter and utilitarian, compact, one accent used only where it means something. A professional creative tool, not a landing page.

Primary action per public page: Home, try the free tool. Story page, paste a story and generate a first video. AI Shorts page, start creating. Features, understand then create. Login, get in with minimal friction. Blog, find something useful and convert.

## Palette

Kept from the existing brand (owner, #243). Defined once in `frontend/src/tokens.css`, light on `:root`, dark on `.dark` (set by ThemeService).

| Role | Light | Dark |
|---|---|---|
| Paper | #FAF7F2 | #0F0E0B |
| Paper 2 | #F4EFE6 | #141310 |
| Paper 3 | #EDE8DB | #2A2620 |
| Ink | #1A1714 | #FAF7F2 |
| Ink 2 | #3E3A33 | #EDE8DB |
| Muted | #6B6560 | #A8A196 |
| Accent (fill) | #BC5227 | #D96C3D |
| Text on accent fill | #FFFBF6 | #0F0E0B |
| Accent as text | #A8481F | #D96C3D |
| Focus ring | #A8481F | #D96C3D |
| Positive / Caution / Negative | #1E3A2A / #8A5C00 / #B3372F | #6FBF8F / #E0A24A / #F08A74 |

Every value except the dark Muted, the dark status colours, #FFFBF6 and #8A5C00 is already used in the app. Those four are new and exist only to pass contrast (Proposed). Contrast, computed (WCAG 2.x):

| Pair | Ratio | Small text (4.5) |
|---|---|---|
| Ink on paper | 16.7 | Pass |
| Muted on paper / paper 3 | 5.4 / 4.7 | Pass |
| Terracotta #BC5227 on paper | 4.49 | Fail |
| Terracotta #BC5227 on paper 2 | 4.19 | Fail |
| Accent text #A8481F on paper / paper 2 / paper 3 | 5.44 / 5.08 / 4.75 | Pass |
| #FFFBF6 on terracotta fill | 4.66 | Pass |
| #FFFBF6 on #A8481F (hover) | 5.64 | Pass |
| Dark ink on dark paper | 18.1 | Pass |
| Dark accent #D96C3D on dark paper / paper 3 | 5.66 / 4.41 | Pass / Fail (large text only on paper 3) |
| Light status colours on all three papers | 4.75 or more | Pass |

Rules that follow:

- Terracotta #BC5227 is for fills and large type. For words, links and small icons use `--color-accent-text`.
- The accent marks the primary action, the current step and the current selection. Not every icon, badge and border.
- Status always carries text or an icon, never colour alone.

## What stops being the palette (Proposed)

The audit found a second, unrelated palette growing in the code. It goes away as pages are redesigned:

- Indigo/violet: `--color-primary #6366f1`, `--primary`/`--accent`/`--ring` (239 84% 67%) in `styles.css`, the purple accent `#7C3AED` in the page `theme-color`, and the violet radial gradients in `.aurora-bg` and `.spotlight-card`. Indigo on paper is 4.18, and the whole set is the stock AI palette.
- Slate and teal stragglers: #0F172A, #E2E8F0, #64748B, #0F766E, #115E59, #22D3EE, #FFD60A and similar.
- Angular Material and `.glass-card` overrides in `styles.css` that hard-code white-alpha surfaces. They are rewritten against the tokens when the shared chrome lands (#245).

Until each page adopts the tokens, the old values keep working, so this PR changes no page.

## Typography

Kept from the existing brand.

- Display: Cormorant Garamond. Headings, large numerals, editorial moments. Public pages only.
- UI and body: Outfit. All app screens use Outfit alone; no display face on labels, buttons or data.
- Labels, table heads, captions and nav links are the body face, a step smaller, sentence case, untracked. No tracked capitals, no eyebrows (the audit found 86 `uppercase` and 88 `tracking-[..em]` uses to retire).
- Scale and spacing live in tokens.css (`--type-*`, `--space-*`). Body is 16px or larger.

## Theme

Light and dark both ship (decided: ThemeService already toggles, default dark on first visit). Every screen is checked in both before it merges.

## Motion

Hover and focus states, and real progress feedback. `transform` and `opacity` only, never `transition: all`, every animation has a `prefers-reduced-motion` fallback, nothing hidden until a script runs.

Landing page exception, kept from the earlier draft (owner request): the page-flip book and the hero slide pair may move. Nothing loops, the 3D scene draws only on change, and with reduced motion, data saver, no WebGL or a failed library load, visitors get static images and the same content as a plain list.

## Layout

One container (1280px) centred, gutters grow on wide screens, the measure does not. One primary button style. One radius per nesting level: control 8px, surface 12px, pill for tags only. No card inside a card, no glass panels, no purple-to-blue gradient.

## Icons (R-31 / R-04)

Lucide throughout. A single icon family keeps the studio from reading as a collage, and Lucide's line weight is neutral enough not to fight the type. Icons are used where they have a concrete referent (clapperboard, scissors, settings); otherwise text carries the label. The generic sparkles and zap glyphs stay removed. No new icon family without updating this reason.

## Honest content

No invented numbers, testimonials, customer logos or case-study counts. Real product screenshots where possible; where none exist, a clearly labelled product mockup. Facts printed on pages are checked against the code (for example 2 to 10 scenes and 90 seconds come from `storyplan` and `shorts` limits).

## Open decisions

1. Approve this document and the Proposed items above (owner).
2. Pricing and Examples pages do not exist yet (#255).
3. The renderer's slide templates (Classic and the five palettes in `backend/internal/slidegen/theme.go`) are a separate system from the website. Classic still uses a purple accent and plain white and is frozen by golden images; moving it to the brand changes every existing Classic project on its next render. Not part of this redesign unless the owner asks.
