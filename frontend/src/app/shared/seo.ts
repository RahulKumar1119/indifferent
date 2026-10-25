/** Sets (or creates) the self-referencing canonical link for the route.
 * Takes the document explicitly: inject() must not be called from a plain
 * function outside an injection context (it throws NG0203, which would abort
 * ngOnInit and silently drop all markup injected after it). */
export function setCanonical(doc: Document, url: string): void {
  let link = doc.head.querySelector<HTMLLinkElement>('link[rel="canonical"]');
  if (!link) {
    link = doc.createElement('link');
    link.rel = 'canonical';
    doc.head.appendChild(link);
  }
  link.setAttribute('href', url);
}

/** Sets a meta tag (creating it when absent) by name or property. */
export function setMetaTag(doc: Document, key: 'name' | 'property', keyValue: string, content: string): void {
  const selector = `meta[${key}="${keyValue}"]`;
  let tag = doc.head.querySelector<HTMLMetaElement>(selector);
  if (!tag) {
    tag = doc.createElement('meta');
    tag.setAttribute(key, keyValue);
    doc.head.appendChild(tag);
  }
  tag.setAttribute('content', content);
}

/** Robots directive for the current route. The tag lives in index.html's head
 * for the whole session, so every page must either set it or let the router
 * reset it (see App): a noindex left behind would hide the next page. */
export const ROBOTS_INDEX = 'index, follow';
export const ROBOTS_NOINDEX = 'noindex, follow';

export function setRobots(doc: Document, content: string): void {
  setMetaTag(doc, 'name', 'robots', content);
}

/** Signed-in areas. They redirect to the login page for visitors and are also
 * disallowed in robots.txt, so they must never be indexed. `/shorts-maker` is a
 * public landing page and must not match `/shorts`. */
const PRIVATE_EXACT = ['/shorts', '/new'];
const PRIVATE_PREFIXES = ['/dashboard', '/projects', '/shorts/', '/account', '/profile', '/settings', '/auth/'];

/** The robots directive a path gets before its own page code runs. */
export function robotsForUrl(url: string): string {
  const path = url.split(/[?#]/)[0] || '/';
  const isPrivate =
    PRIVATE_EXACT.includes(path) || PRIVATE_PREFIXES.some((prefix) => path === prefix.replace(/\/$/, '') || path.startsWith(prefix));
  return isPrivate ? 'noindex, nofollow' : ROBOTS_INDEX;
}

/** Upserts one JSON-LD block by id, so revisiting a page (or moving between
 * pages that share a block) never stacks duplicate structured data. */
export function setJsonLd(doc: Document, id: string, data: Record<string, unknown>): void {
  let script = doc.head.querySelector<HTMLScriptElement>(`script[type="application/ld+json"][data-seo-id="${id}"]`);
  if (!script) {
    script = doc.createElement('script');
    script.type = 'application/ld+json';
    script.setAttribute('data-seo-id', id);
    doc.head.appendChild(script);
  }
  script.text = JSON.stringify(data);
}

/** Removes every JSON-LD block this helper created. Called on navigation so a
 * page never inherits the previous page's structured data. */
export function clearJsonLd(doc: Document): void {
  doc.head.querySelectorAll('script[type="application/ld+json"][data-seo-id]').forEach((el) => el.remove());
}

/** One-call SEO for static pages: title + description (+ OG) + canonical. */
export function setPageSeo(
  doc: Document,
  opts: { title: string; description: string; canonical: string },
): void {
  doc.title = opts.title;
  setMetaTag(doc, 'name', 'description', opts.description);
  setMetaTag(doc, 'property', 'og:title', opts.title);
  setMetaTag(doc, 'property', 'og:description', opts.description);
  setMetaTag(doc, 'property', 'og:url', opts.canonical);
  setMetaTag(doc, 'name', 'twitter:title', opts.title);
  setMetaTag(doc, 'name', 'twitter:description', opts.description);
  setCanonical(doc, opts.canonical);
}
