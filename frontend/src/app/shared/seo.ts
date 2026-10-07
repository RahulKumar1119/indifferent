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
  setCanonical(doc, opts.canonical);
}
