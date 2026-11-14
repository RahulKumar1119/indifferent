/**
 * Fails the build when the self-hosted fonts of the infographic maker (#217) are not in order:
 *
 *   node scripts/check-fonts.mjs [distDir]
 *
 * The fonts are copied from pinned @fontsource packages into <dist>/fonts/infographic/<package>/
 * (angular.json). Each package must be on the list below, which is the list of fonts whose
 * licence has been read, must ship its licence text (SIL Open Font License 1.1), and every font
 * file its stylesheets point to must exist. A new font package therefore cannot reach production
 * until someone adds it here on purpose. Plain Node, no dependencies, like the other build scripts.
 */
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const dist = process.argv[2] ?? join(here, '..', 'dist', 'frontend', 'browser');
const root = join(dist, 'fonts', 'infographic');

/** The font packages whose licence was read, and the family each one declares. Add new ones here and in angular.json together. */
const ALLOWED = { 'noto-sans': 'Noto Sans', 'noto-sans-gujarati': 'Noto Sans Gujarati' };
const WEIGHTS = [400, 700];

const problems = [];
const fail = (m) => problems.push(m);

if (!existsSync(root)) {
  console.error(`check-fonts: ${root} does not exist. Build first (ng build) so the fonts are copied.`);
  process.exit(1);
}

const found = readdirSync(root, { withFileTypes: true }).filter((e) => e.isDirectory()).map((e) => e.name);
for (const name of found) if (!(name in ALLOWED)) fail(`fonts/infographic/${name} is not on the list of fonts whose licence was checked (scripts/check-fonts.mjs).`);
for (const name of Object.keys(ALLOWED)) if (!found.includes(name)) fail(`fonts/infographic/${name} is missing.`);

let files = 0;
for (const [name, family] of Object.entries(ALLOWED)) {
  const dir = join(root, name);
  if (!existsSync(dir)) continue;
  const licence = join(dir, 'LICENSE');
  if (!existsSync(licence)) {
    fail(`${name}: no LICENSE file.`);
  } else {
    const text = readFileSync(licence, 'utf8');
    if (!/SIL Open Font License,? Version 1\.1/i.test(text)) fail(`${name}: LICENSE is not the SIL Open Font License 1.1.`);
    if (!/Copyright/i.test(text)) fail(`${name}: LICENSE has no copyright line.`);
  }
  for (const weight of WEIGHTS) {
    const sheet = join(dir, `${weight}.css`);
    if (!existsSync(sheet)) {
      fail(`${name}: ${weight}.css is missing.`);
      continue;
    }
    const css = readFileSync(sheet, 'utf8');
    const faces = css.match(/@font-face\s*{[^}]*}/g) ?? [];
    if (faces.length === 0) fail(`${name}/${weight}.css has no @font-face rule.`);
    for (const face of faces) {
      if (!face.includes(`font-family: '${family}'`)) fail(`${name}/${weight}.css declares a family other than ${family}.`);
      for (const [, url] of face.matchAll(/url\(([^)]+)\)/g)) {
        const ref = url.replace(/^['"]|['"]$/g, '');
        if (/^(https?:)?\/\//.test(ref) || ref.startsWith('data:')) {
          fail(`${name}/${weight}.css loads a font from outside the site: ${ref}`);
        } else if (!ref.endsWith('.woff2')) {
          // The stylesheets also list a .woff fallback after the .woff2. The build ships only the .woff2 (every
          // browser that can run the tool reads it), so the fallback is never requested and need not exist.
          continue;
        } else if (!existsSync(join(dir, ref))) {
          fail(`${name}/${weight}.css points to ${ref}, which is not in the build.`);
        } else {
          files++;
        }
      }
    }
  }
}

if (problems.length) {
  console.error(`check-fonts: ${problems.length} problem(s):\n  ${problems.join('\n  ')}`);
  process.exit(1);
}
console.log(`check-fonts ok: ${found.length} font packages, ${files} font files, all under the SIL Open Font License 1.1.`);
