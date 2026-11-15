/**
 * Removal of the cookies this site can reach (#321). Cookies written by Google's
 * tags and CloudWatch RUM sit on this site's own domain, so the page can expire
 * them. Cookies on other domains (a third party's own cookies) and HttpOnly
 * cookies cannot be seen or removed from here, and nothing in the product says
 * they were.
 */

/** The domains a cookie of this page may have been set on: none given, the host, and each parent. */
export function cookieDomains(hostname: string): (string | null)[] {
  const out: (string | null)[] = [null];
  if (!hostname || /^[\d.]+$/.test(hostname) || hostname.includes(':') || !hostname.includes('.')) return out;
  const labels = hostname.split('.');
  for (let i = 0; i <= labels.length - 2; i++) {
    const d = labels.slice(i).join('.');
    out.push(d, '.' + d);
  }
  return out;
}

/** Names of the cookies visible to the page. */
export function cookieNames(doc: Document): string[] {
  return doc.cookie
    .split(';')
    .map((part) => part.split('=')[0].trim())
    .filter((name) => name !== '');
}

/** Expires every visible cookie whose name matches, on every domain it may have been set on. Returns the names it tried. */
export function clearCookies(doc: Document, patterns: readonly RegExp[]): string[] {
  const host = doc.location?.hostname ?? '';
  const tried: string[] = [];
  for (const name of cookieNames(doc)) {
    if (!patterns.some((p) => p.test(name))) continue;
    for (const domain of cookieDomains(host)) {
      doc.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; max-age=0; path=/${domain ? `; domain=${domain}` : ''}`;
    }
    tried.push(name);
  }
  return tried;
}
