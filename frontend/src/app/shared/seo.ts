import { DOCUMENT } from '@angular/common';
import { inject } from '@angular/core';

/** Sets (or creates) the self-referencing canonical link for the route. */
export function setCanonical(url: string): void {
  const document = inject(DOCUMENT);
  let link = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]');
  if (!link) {
    link = document.createElement('link');
    link.rel = 'canonical';
    document.head.appendChild(link);
  }
  link.href = url;
}
