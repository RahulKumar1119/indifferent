import { ChangeDetectionStrategy, Component, ElementRef, afterRenderEffect, computed, input, viewChild } from '@angular/core';
import { ClipHook } from './clip-hook.models';

/**
 * The approximate preview of a clip's hook, drawn over the clip's player
 * (#329). It sizes itself from the player's width, so it scales with the card.
 * The page sets its opacity while the clip plays, the way the render fades it.
 */
@Component({
  selector: 'app-clip-hook-preview',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [`
    :host { position: absolute; inset: 0; z-index: 1; pointer-events: none; container-type: inline-size; }
    .hook-ov { position: absolute; left: 50%; transform: translateX(-50%); max-width: 96cqw; width: max-content; text-align: center;
      font: 700 7.4cqw/1.1 Arial, 'Noto Sans Devanagari', 'Noto Sans', sans-serif; overflow-wrap: anywhere; }
    .top { top: 8.33%; }
    .middle { top: 50%; transform: translate(-50%, -50%); }
    .box { color: #fff; background: rgb(0 0 0 / 80%); padding: 1.6cqw 1.7cqw; }
    .outline { color: #fff; -webkit-text-stroke: .25cqw #000; paint-order: stroke fill; text-shadow: .3cqw .3cqw .3cqw rgb(0 0 0 / 50%); }
    .band { color: #111; background: #ffd60a; padding: 2cqw; }
    .tag { position: absolute; top: .5rem; left: 50%; transform: translateX(-50%); z-index: 2; padding: .125rem .5rem; border-radius: 999px;
      font-size: .625rem; font-weight: 600; background: color-mix(in oklab, var(--color-paper) 80%, transparent); color: var(--color-ink); white-space: nowrap; }
  `],
  template: `
    <div #ov [class]="'hook-ov ' + hook().style + ' ' + hook().position" [attr.lang]="lang()" aria-hidden="true" data-testid="hook-preview">{{ hook().text }}</div>
    <span class="tag" aria-hidden="true">Hook preview, approximate</span>
  `,
})
export class ClipHookPreviewComponent {
  readonly hook = input.required<ClipHook>();
  readonly lang = computed(() => (/[\u0900-\u097F]/.test(this.hook().text) ? 'hi' : null));
  private readonly ov = viewChild.required<ElementRef<HTMLElement>>('ov');

  constructor() {
    // Like the render: the largest size (80 px of 1080) that fits two lines, down to 40 px.
    afterRenderEffect(() => {
      const h = this.hook();
      const el = this.ov().nativeElement;
      void h.text;
      void h.style;
      for (let size = PREVIEW_MAX_CQW; size >= PREVIEW_MIN_CQW; size -= PREVIEW_STEP_CQW) {
        el.style.fontSize = `${size}cqw`;
        const line = parseFloat(getComputedStyle(el).lineHeight) || el.clientHeight;
        if (el.scrollHeight <= line * 2 + 2 + paddingY(el)) break;
      }
    });
  }
}

/** 80 and 40 pixels of a 1080 pixel frame, and the 2 pixel step the render shrinks by. */
const PREVIEW_MAX_CQW = (80 / 1080) * 100;
const PREVIEW_MIN_CQW = (40 / 1080) * 100;
const PREVIEW_STEP_CQW = (2 / 1080) * 100;

function paddingY(el: HTMLElement): number {
  const cs = getComputedStyle(el);
  return (parseFloat(cs.paddingTop) || 0) + (parseFloat(cs.paddingBottom) || 0);
}
