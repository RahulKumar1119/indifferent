/**
 * After a deploy: checks the live site the way a crawler meets it (#361).
 *
 *   node scripts/verify-live-urls.mjs [https://indifferent.fun]
 *
 * It reads the live sitemap.xml and, for every address in it, asks for the page
 * without following redirects. Each must answer 200 itself (not 301), carry a
 * canonical tag and an og:url equal to its own address, not be marked noindex,
 * and have real content in the prerendered page. It also confirms that the
 * slash-less form of a few pages redirects to the slash form (which Amplify does,
 * and which is fine as long as nothing points at the slash-less form), and that
 * robots.txt names the sitemap.
 *
 * It says nothing about indexing. A page that passes here can still be missing
 * from Google: Search Console is where that shows.
 *
 * Exits 1 and lists every failure. No dependencies; Node 18 or newer.
 */
const BASE = (process.argv[2] ?? 'https://indifferent.fun').replace(/\/+$/, '');
const MIN_WORDS = 150;
const failures = [];
const notes = [];
const fail = (m) => failures.push(m);

async function get(url) {
  for (let attempt = 1; attempt <= 3; attempt++) {
    try {
      const res = await fetch(url, { redirect: 'manual', headers: { 'user-agent': 'indifferent-url-check/1 (+https://indifferent.fun)' } });
      const body = res.status === 200 ? await res.text() : '';
      return { status: res.status, location: res.headers.get('location'), body };
    } catch (err) {
      if (attempt === 3) return { status: 0, location: null, body: '', error: String(err) };
      await new Promise((r) => setTimeout(r, 1000 * attempt));
    }
  }
}

const attr = (html, re) => re.exec(html)?.[1]?.replace(/&amp;/g, '&');
const canonicalOf = (h) => attr(h, /<link[^>]*rel="canonical"[^>]*href="([^"]*)"/i) ?? attr(h, /<link[^>]*href="([^"]*)"[^>]*rel="canonical"/i);
const ogUrlOf = (h) => attr(h, /<meta[^>]*property="og:url"[^>]*content="([^"]*)"/i);
const robotsOf = (h) => attr(h, /<meta[^>]*name="robots"[^>]*content="([^"]*)"/i) ?? '';
function words(html) {
  const root = /<app-root[^>]*>([\s\S]*)<\/app-root>/i.exec(html)?.[1] ?? '';
  const text = root.replace(/<script[\s\S]*?<\/script>|<style[\s\S]*?<\/style>/gi, ' ').replace(/<[^>]+>/g, ' ');
  return text.split(/\s+/).filter(Boolean).length;
}

const sitemap = await get(`${BASE}/sitemap.xml`);
if (sitemap.status !== 200) {
  console.error(`sitemap.xml answered ${sitemap.status}${sitemap.error ? ` (${sitemap.error})` : ''}`);
  process.exit(1);
}
const locs = [...sitemap.body.matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1]);
if (locs.length === 0) fail('sitemap.xml lists no addresses.');

const robots = await get(`${BASE}/robots.txt`);
if (robots.status !== 200 || !robots.body.includes(`Sitemap: ${BASE}/sitemap.xml`)) fail('robots.txt does not answer 200 with the sitemap line.');

const rows = [];
for (const loc of locs) {
  const r = await get(loc);
  const row = { loc, status: r.status, ok: true };
  const problem = (m) => {
    row.ok = false;
    fail(`${loc}: ${m}`);
  };
  if (r.status !== 200) {
    problem(`answered ${r.status}${r.location ? ` to ${r.location}` : ''}${r.error ? ` (${r.error})` : ''}`);
  } else {
    const canonical = canonicalOf(r.body);
    if (canonical !== loc) problem(`canonical is ${canonical ?? 'missing'}`);
    const og = ogUrlOf(r.body);
    if (og !== loc) problem(`og:url is ${og ?? 'missing'}`);
    if (/noindex/i.test(robotsOf(r.body))) problem(`is marked ${robotsOf(r.body)}`);
    row.words = words(r.body);
    if (row.words < MIN_WORDS) problem(`only ${row.words} words in the served page`);
  }
  rows.push(row);
}

// The slash-less form of a few pages should redirect to the slash form, so an old link still lands.
for (const loc of locs.filter((l) => l !== `${BASE}/`).slice(0, 5)) {
  const bare = loc.replace(/\/$/, '');
  const r = await get(bare);
  const target = r.location && new URL(r.location, bare).href;
  if (r.status === 301 && target === loc) notes.push(`${bare} redirects to ${loc}, as expected`);
  else fail(`${bare}: expected a 301 to ${loc}, got ${r.status}${r.location ? ` to ${r.location}` : ''}`);
}

for (const row of rows) console.log(`${row.ok ? '  ok  ' : '  FAIL'} ${row.status} ${String(row.words ?? '-').padStart(5)} words  ${row.loc}`);
for (const n of notes) console.log(`  ok   ${n}`);
if (failures.length > 0) {
  console.error(`\n${failures.length} problem${failures.length === 1 ? '' : 's'}:`);
  for (const f of failures) console.error(`  - ${f}`);
  process.exit(1);
}
console.log(`\nAll ${locs.length} sitemap addresses answer 200 without a redirect, with a self-referencing canonical and og:url.`);
console.log('This does not show that Google has indexed them. Check Search Console.');
