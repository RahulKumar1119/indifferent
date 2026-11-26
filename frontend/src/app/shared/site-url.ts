import { DefaultUrlSerializer, UrlTree } from '@angular/router';

/**
 * One address per page (#361). Amplify serves each prerendered page from
 * `<route>/index.html`, so `/about` answers 301 to `/about/`, and only the
 * address with the trailing slash answers 200. Canonical tags, Open Graph URLs,
 * structured data, the sitemap and the app's own links therefore all use the
 * trailing-slash form, so nothing a crawler is told about ever redirects.
 */
export const SITE_URL = 'https://indifferent.fun';

/** True for a path whose last segment names a file (sitemap.xml, og-cover.jpg). */
function isFile(path: string): boolean {
  const last = path.split('/').pop() ?? '';
  return /\.[a-z0-9]{1,8}$/i.test(last);
}

/** `/about` and `/about/` both become `/about/`; `/`, files, a query and a fragment are kept. */
export function withTrailingSlash(url: string): string {
  const cut = url.search(/[?#]/);
  const path = cut < 0 ? url : url.slice(0, cut);
  const rest = cut < 0 ? '' : url.slice(cut);
  if (path === '' || path.endsWith('/') || isFile(path)) return url;
  return `${path}/${rest}`;
}

/** The absolute address of a page on this site, in its one served form. */
export function siteUrl(path: string): string {
  const p = path.startsWith('/') ? path : `/${path}`;
  return `${SITE_URL}${withTrailingSlash(p)}`;
}

/** A URL on this site in its served form; any other URL as given. */
export function canonicalForm(url: string): string {
  if (url !== SITE_URL && !url.startsWith(`${SITE_URL}/`)) return url;
  return siteUrl(url.slice(SITE_URL.length) || '/');
}

/**
 * The router's URLs, with a trailing slash. Every `routerLink` href, and the
 * address bar, then match the address Amplify serves, so a crawler following a
 * link never meets a redirect. A URL that arrives with the slash (a reload of
 * `/about/`) is read the same as one without it; the default parser would read
 * the slash as an extra empty segment and match no route.
 */
export class TrailingSlashUrlSerializer extends DefaultUrlSerializer {
  override parse(url: string): UrlTree {
    const cut = url.search(/[?#]/);
    const path = cut < 0 ? url : url.slice(0, cut);
    const rest = cut < 0 ? '' : url.slice(cut);
    const trimmed = path.length > 1 ? path.replace(/\/+$/, '') : path;
    return super.parse(`${trimmed || '/'}${rest}`);
  }

  override serialize(tree: UrlTree): string {
    return withTrailingSlash(super.serialize(tree));
  }
}
