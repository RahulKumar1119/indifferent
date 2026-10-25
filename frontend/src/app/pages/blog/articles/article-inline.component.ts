import { Component, computed, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { parseInline } from './article-inline';

/** Renders article text with the inline syntax from article-inline.ts. Plain
 * Angular bindings only, so nothing in a content module is ever treated as HTML. */
@Component({
  selector: 'app-inline',
  standalone: true,
  imports: [RouterLink],
  styles: [':host { display: contents; }'],
  template: `@for (s of segments(); track $index) {
    @if (s.kind === 'bold') {<strong>{{ s.text }}</strong>}
    @else if (s.kind === 'code') {<code>{{ s.text }}</code>}
    @else if (s.kind === 'link') {
      @if (s.external) {<a [href]="s.href" target="_blank" rel="noopener noreferrer">{{ s.text }}</a>}
      @else {<a [routerLink]="s.href">{{ s.text }}</a>}
    }
    @else {<ng-container>{{ s.text }}</ng-container>}
  }`,
})
export class InlineTextComponent {
  readonly text = input.required<string>();
  protected readonly segments = computed(() => parseInline(this.text()));
}
