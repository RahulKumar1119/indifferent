/**
 * Build check: the inline scripts and inline event handlers in the built pages
 * must each have their hash in the Content-Security-Policy-Report-Only header of
 * customHttp.yml, so a change to index.html cannot silently put the policy out
 * of date. Fails (exit 1) on any mismatch or on an inline script that is not
 * listed.
 *
 * Usage: node scripts/check-csp.mjs [dist/frontend/browser] [--print]
 *   --print  also print the hashes found, to paste into customHttp.yml
 */
import { createHash } from 'node:crypto';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const args = process.argv.slice(2).filter((a) => !a.startsWith('--'));
const PRINT = process.argv.includes('--print');
const DIST = resolve(args[0] ?? join(here, '../dist/frontend/browser'));
const HEADERS_FILE = resolve(join(here, '../../customHttp.yml'));
const HEADER = 'Content-Security-Policy-Report-Only';

const sha256 = (text) => 'sha256-' + createHash('sha256').update(text, 'utf8').digest('base64');

/** The value of the CSP header, with the YAML folding undone. */
export function policyFromYaml(yaml) {
  const lines = yaml.split('\n');
  const at = lines.findIndex((l) => l.trim() === `- key: ${HEADER}`);
  if (at < 0) throw new Error(`${HEADER} not found in customHttp.yml`);
  const value = lines[at + 1];
  if (!/^\s+value: >-\s*$/.test(value)) throw new Error(`${HEADER} must use a folded (>-) value`);
  const parts = [];
  for (const line of lines.slice(at + 2)) {
    if (line.trim() === '' || /^\s{0,8}\S/.test(line)) break; // next key at a lower indent
    parts.push(line.trim());
  }
  return parts.join(' ');
}

/** directive name -> list of source tokens */
export function parsePolicy(policy) {
  const out = new Map();
  for (const d of policy.split(';')) {
    const [name, ...sources] = d.trim().split(/\s+/);
    if (name) out.set(name, sources);
  }
  return out;
}

function htmlFiles(dir) {
  const found = [];
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) found.push(...htmlFiles(p));
    else if (name.endsWith('.html')) found.push(p);
  }
  return found;
}

const decode = (s) =>
  s.replace(/&quot;/g, '"').replace(/&#39;/g, "'").replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&');

/**
 * Inline script bodies and inline on*= handler values found in the page.
 * Tag and attribute names are matched without regard to case, and the closing tag
 * may carry spaces or attributes (`</SCRIPT >`), because an HTML parser accepts
 * all of those. A script this check missed would be one the policy blocks.
 */
export function inlineCode(html) {
  const scripts = [];
  const handlers = [];
  for (const m of html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script\b[^>]*>/gi)) {
    const attrs = m[1];
    if (/\ssrc\s*=/i.test(attrs) || /type\s*=\s*["']?application\/(ld\+json|json)/i.test(attrs)) continue;
    scripts.push(m[2]);
  }
  for (const m of html.matchAll(/\son[a-z]+\s*=\s*(?:"([^"]*)"|'([^']*)')/gi)) handlers.push(decode(m[1] ?? m[2]));
  return { scripts, handlers };
}

function main() {
  const directives = parsePolicy(policyFromYaml(readFileSync(HEADERS_FILE, 'utf8')));
  const scriptSrc = directives.get('script-src') ?? [];
  const scriptAttr = directives.get('script-src-attr') ?? [];
  const has = (list, hash) => list.includes(`'${hash}'`);

  const missing = new Map(); // message -> pages affected
  const seen = new Map();
  for (const file of htmlFiles(DIST)) {
    const { scripts, handlers } = inlineCode(readFileSync(file, 'utf8'));
    for (const s of scripts) {
      const h = sha256(s);
      seen.set(h, `inline script: ${s.trim().slice(0, 60).replace(/\s+/g, ' ')}...`);
      if (!has(scriptSrc, h)) missing.set(`script-src is missing '${h}'`, (missing.get(`script-src is missing '${h}'`) ?? 0) + 1);
    }
    for (const s of handlers) {
      const h = sha256(s);
      seen.set(h, `handler: ${s}`);
      const msg = `script-src-attr is missing '${h}' for ${s}`;
      if (!has(scriptAttr, h)) missing.set(msg, (missing.get(msg) ?? 0) + 1);
    }
  }

  // The reverse: a hash listed in the policy that no page uses is stale.
  const used = new Set(seen.keys());
  const stale = [...scriptSrc, ...scriptAttr]
    .filter((t) => /^'sha256-/.test(t))
    .map((t) => t.slice(1, -1))
    .filter((h) => !used.has(h));

  if (PRINT) for (const [h, what] of seen) console.log(`'${h}'  ${what}`);
  if (missing.size > 0 || stale.length > 0) {
    console.error('CSP check failed. Update customHttp.yml (run: node scripts/check-csp.mjs --print).');
    for (const [m, pages] of missing) console.error(`  ${m} (${pages} page${pages === 1 ? '' : 's'})`);
    for (const s of stale) console.error(`  stale hash in the policy, no page uses it: '${s}'`);
    process.exit(1);
  }
  console.log(`CSP check ok: ${seen.size} inline script/handler hashes match across ${htmlFiles(DIST).length} pages.`);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) main();
