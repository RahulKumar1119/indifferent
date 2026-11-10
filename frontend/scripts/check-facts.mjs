/**
 * Fails when a usage limit in product-facts.ts (shown on /pricing) differs
 * from the constant the API enforces in the Go source (#255).
 *
 *   node scripts/check-facts.mjs
 *
 * Needs Node 22.18 or newer: it imports product-facts.ts directly.
 */
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { FRONTEND_DIR } from './site-routes.mjs';

const API = join(FRONTEND_DIR, '..', 'backend', 'internal', 'api');
const { PRODUCT_FACTS } = await import(pathToFileURL(join(FRONTEND_DIR, 'src/app/core/config/product-facts.ts')).href);

const read = (file) => readFileSync(join(API, file), 'utf8');
/** `quota{name: "<name>", limit: N` in quota.go. */
const quota = (name) => {
  const m = new RegExp(`name:\\s*"${name}",\\s*limit:\\s*(\\d+)`).exec(read('quota.go'));
  return m ? Number(m[1]) : NaN;
};
/** `<constant> = N` in a Go file. */
const constant = (file, name) => {
  const m = new RegExp(`\\b${name}\\s*=\\s*(\\d+)\\b`).exec(read(file));
  return m ? Number(m[1]) : NaN;
};

const L = PRODUCT_FACTS.limits;
const pairs = [
  ['limits.quizProjectsPerDay', L.quizProjectsPerDay, 'quota.go project-create', quota('project-create')],
  ['limits.fileUploadsPerDay', L.fileUploadsPerDay, 'quota.go upload-url', quota('upload-url')],
  ['limits.shortsUploadsPerDay', L.shortsUploadsPerDay, 'quota.go shorts-create', quota('shorts-create')],
  ['limits.shortsRetriesPerDay', L.shortsRetriesPerDay, 'quota.go shorts-retry', quota('shorts-retry')],
  ['limits.thumbnailSetsPerDay', L.thumbnailSetsPerDay, 'quota.go thumbnails', quota('thumbnails')],
  ['limits.storyPlansPerHour', L.storyPlansPerHour, 'storyplan.go storyPlanDefaultPerHour', constant('storyplan.go', 'storyPlanDefaultPerHour')],
  ['limits.stockSearchesPerHour', L.stockSearchesPerHour, 'stockmedia.go stockSearchPerHour', constant('stockmedia.go', 'stockSearchPerHour')],
  ['story.perDay', PRODUCT_FACTS.story.perDay, 'storyvideo.go storyVideoDefaultPerDay', constant('storyvideo.go', 'storyVideoDefaultPerDay')],
  ['story.maxActive', PRODUCT_FACTS.story.maxActive, 'storyvideo.go storyVideoDefaultMaxActive', constant('storyvideo.go', 'storyVideoDefaultMaxActive')],
];

let failed = 0;
for (const [fact, shown, source, enforced] of pairs) {
  if (Number.isNaN(enforced)) {
    console.error(`✗ ${source}: not found in the Go source. Update scripts/check-facts.mjs if it moved.`);
    failed++;
  } else if (shown !== enforced) {
    console.error(`✗ ${fact} is ${shown} on the site, but ${source} enforces ${enforced}.`);
    failed++;
  }
}
if (failed) {
  console.error(`check-facts: ${failed} problem(s).`);
  process.exit(1);
}
console.log(`check-facts ok: ${pairs.length} limits match the API source.`);
