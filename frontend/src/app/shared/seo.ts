/** Sets (or creates) the self-referencing canonical link for the route.
 * Takes the document explicitly: inject() must not be called from a plain
 * function outside an injection context (it throws NG0203, which would abort
 * ngOnInit and silently drop all markup injected after it). */
export function setCanonical(doc: Document, url: string): void {
  let link = doc.head.querySelector<HTMLLinkElement>('link[rel="canonical"]');
  if (!link) {
    link = doc.createElement('link');
    link.rel = 'canonical';
    document.head.appendChild(link);
  }
  link.href = url;
}
