import { ChangeDetectionStrategy, Component, ElementRef, effect, inject, input } from '@angular/core';
import { GLYPH_ATTRS, GLYPH_TAGS, Glyph } from './decor-library';

const SVG_NS = 'http://www.w3.org/2000/svg';

/**
 * Draws one icon or shape in the current text colour. The elements are made
 * with the DOM, from a fixed list of tags and attributes, so nothing is parsed
 * as markup. Icons are outlines; shapes are filled.
 */
@Component({
  selector: 'app-decor-glyph',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [':host { display: block; line-height: 0; } svg { display: block; width: 100%; height: 100%; overflow: visible; }'],
  template: '',
})
export class DecorGlyphComponent {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  readonly glyph = input.required<Glyph | undefined>();

  constructor() {
    effect(() => {
      const g = this.glyph();
      const host = this.host.nativeElement;
      host.replaceChildren();
      if (!g) return;
      const doc = host.ownerDocument;
      const svg = doc.createElementNS(SVG_NS, 'svg');
      svg.setAttribute('viewBox', '0 0 24 24');
      svg.setAttribute('aria-hidden', 'true');
      svg.setAttribute('focusable', 'false');
      if (g.kind === 'icon') {
        svg.setAttribute('fill', 'none');
        svg.setAttribute('stroke', 'currentColor');
        svg.setAttribute('stroke-width', '2');
        svg.setAttribute('stroke-linecap', 'round');
        svg.setAttribute('stroke-linejoin', 'round');
      } else {
        svg.setAttribute('fill', 'currentColor');
      }
      for (const [tag, attrs] of g.nodes) {
        if (!GLYPH_TAGS.has(tag)) continue;
        const el = doc.createElementNS(SVG_NS, tag);
        for (const [name, value] of Object.entries(attrs)) if (GLYPH_ATTRS.has(name)) el.setAttribute(name, String(value));
        svg.appendChild(el);
      }
      host.appendChild(svg);
    });
  }
}
