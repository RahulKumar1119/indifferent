# Infographic maker spike (#217)

A throwaway proof, run before anything is built, that the export design works for Latin, Hindi, Arabic (right to left) and Simplified Chinese. It has its own `package.json` and `node_modules`, so the app's install and bundle are untouched. Nothing here is routed or shipped.

## Run it

```
cd frontend/e2e/infographic-spike
npm ci
node run.mjs                 # builds the harness, drives headless Chrome, exits 1 if a check fails
python3 check_fonts.py       # needs: pip install fonttools brotli
```
`run.mjs` uses the app's `puppeteer` and `esbuild` (`frontend/node_modules`; run `npm ci` in `frontend/` first) and writes screenshots and `results.json` to `~/shots303/spike` (or the folder you pass), never into the repo.

## What it does
- `harness.ts` draws one sample infographic (title, three steps, a Chart.js bar chart, a footer line) in one language at one of the three export sizes, then exports it with html-to-image using a font stylesheet built by `src/app/pages/tools/infographic/font-slices.ts` (the part that will be kept).
- `run.mjs` does that for 4 languages x 3 sizes (1080x1920, 1080x1080, 1920x1080), with the preview scaled to 0.3 as in the real editor, with 8 times the text to check overflow is noticed, and once with html-to-image's own font embedding for comparison.
- `check_fonts.py` checks, with fontTools, that every character is in the file the export took it from, and that every regular and bold woff2 is under the SIL Open Font License.

## Result (13-14 Nov 2026, Chrome 154 headless, Linux)
All checks pass. Fonts: `@fontsource/noto-sans` 5.3.0, `noto-sans-arabic` 5.3.0, `noto-sans-sc` 5.3.1, `chart.js` 4.6.1, `html-to-image` 1.11.13, all pinned.

| | English | Hindi + English | Arabic | Chinese + English |
|---|---|---|---|---|
| Font files embedded | 7 | 4 | 4 | 20 |
| Font data (woff2) | 106 kB | 112 kB | 123 kB | 570 kB |
| Time to build the font CSS | 31-47 ms | 26-32 ms | 21-26 ms | 123-134 ms |
| PNG export time | 130-250 ms | 187-330 ms | 168-186 ms | 221-239 ms |
| PNG size | 140-208 kB | 142-207 kB | 148-223 kB | 140-201 kB |
| JPG size (quality 0.92) | 114-176 kB | 114-171 kB | 107-156 kB | 108-165 kB |
| Preview vs export, differing pixels | 0.30-0.64% | 0.41-0.80% | 0.19-0.48% | 0.37-1.15% |

- **Sizes:** every PNG and JPG has exactly the requested pixel size (12 of 12). The JPG matches its PNG at 43.1-44.6 dB PSNR.
- **Preview scaled to fit the screen:** exporting while the preview is scaled to 0.3 gives a full-size image with the same number of pixels differing from the preview as the unscaled export (8 of 8).
- **Preview vs export:** the layout, line breaks, shaping and glyphs are the same. The remaining difference is on glyph edges only (sub-pixel anti-aliasing). Shifting the export by one pixel in any direction makes it worse, not better, so it is not an offset. The limit in `run.mjs` is 2%.
- **Hindi:** conjuncts (क्षत्रिय, ज्ञान, श्रद्धा, उज्ज्वल, द्वारा) and mixed Hindi and English lines render correctly, in the page and in the chart labels. This was checked by eye on screenshots by the author of this spike, who is not a Hindi reader, so it needs a Hindi reader's review before it is claimed.
- **Arabic:** the whole template mirrors (number badges, accent bar, chart axis and order), joined letters, diacritics and mixed Latin and digits render, and the full stop after a Latin word at the end of a line sits correctly. Same caveat on review.
- **Chinese:** only the 20 slices the text needs are embedded, not the 204 (about 6.9 MB) in the package.
- **html-to-image on its own** requests all 228 font files on every export (about 7.3 MB of regular and bold files for these three packages, 2.5-2.9 s on this machine with files served locally). The slicing above brings that to 4-20 files.
- **Too much text is noticed:** with 8 times the text, content taller than the canvas is detected in 12 of 12 cases (the real templates will show a warning).
- **Licences:** all 230 regular and bold files in the three packages name the SIL Open Font License, and each package ships its licence text.
- **Code size:** html-to-image, the Chart.js bar chart and `font-slices.ts` bundle to 157 kB minified, 56 kB gzip (kB here is 1024 bytes, as in the tables).

## Findings that change the plan
1. **Noto Sans already contains Devanagari** (Noto Sans 2.015 with the full Devanagari block and the conjunct features). A separate Devanagari package is not needed. Gujarati and the other Indic scripts still need their own packages.
2. **Pasted ligatures** such as U+FB01 are not in the Noto Sans slices, so the browser (and the export) takes them from the Chinese font, which looks different. The tool should turn those presentation forms into plain letters when text is pasted or exported.
3. **Rare characters** have no slice at all (U+9F18 here). The export reports them, so the editor can say which characters will use the visitor's own font.
4. **Square is the tight size.** A layout that fits 1080x1920 can overflow at 1080x1080 (it did, until the harness layout was tightened), so every template has to be tested at all three sizes with long text.
5. **Build output:** regular and bold of these three packages add 7.3 MB to the deployed files (6.9 MB of it is Simplified Chinese), none of it to the page load. `npm ci` for the spike (95 MB of `node_modules`) took 28 s. Japanese and Korean are larger packages and were not measured.

## Not verified
- Safari, Firefox and real phones. Only headless Chrome was run.
- Whether a Hindi, Arabic or Chinese reader finds the text correct. I can only check characters and layout.
- Large exports on low-memory phones.
- The Angular build step that copies the fonts out of `node_modules` (done in the foundation PR).
