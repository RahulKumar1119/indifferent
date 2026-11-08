import { Component, input } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';

/** The short hero above the watermark editor. */
@Component({
  selector: 'app-watermark-hero',
  standalone: true,
  imports: [LucideDynamicIcon],
  styles: [`
    :host { display: block; }
    .card { background: var(--color-paper); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); box-shadow: 0 1px 2px color-mix(in oklab, var(--color-ink) 5%, transparent), 0 12px 32px -18px color-mix(in oklab, var(--color-ink) 28%, transparent); }
    .wrap { width: min(80rem, 100%); margin-inline: auto; padding-inline: var(--space-md); }
    @media (min-width: 48rem) { .wrap { padding-inline: var(--space-xl); } }
    .hero { background: var(--color-paper); color: var(--color-ink); padding: calc(56px + var(--space-2xl)) 0 calc(var(--space-2xl) + var(--space-xl)); }
    h1 { margin: 0; font-family: var(--font-display); font-weight: 500; font-size: var(--type-display-s); line-height: 1; letter-spacing: var(--tracking-display); }
    h1 em { color: var(--color-accent-text); }
    .lead { margin: var(--space-sm) 0 0; max-width: 54ch; font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); }
    .facts { margin: var(--space-md) 0 0; padding: 0; list-style: none; display: flex; flex-wrap: wrap; gap: var(--space-xs) var(--space-lg); font-size: var(--type-sm); color: var(--color-ink-2); }
    .facts li { display: inline-flex; align-items: center; gap: var(--space-xs); }
    @media (max-width: 40rem), (max-height: 30rem) { .hero { padding: calc(56px + var(--space-xl)) 0 calc(var(--space-2xl) + var(--space-md)); } .lead { font-size: var(--type-md); } .facts { gap: var(--space-2xs) var(--space-md); font-size: var(--type-xs); } }
    .facts svg { color: var(--color-accent-text); }
  `],
  template: `
    <header class="hero">
      <div class="wrap">
        <h1>Watermark images <em>and video.</em></h1>
        <p class="lead">Add a text or logo watermark, see it before and after, and download the result. Free, with no sign-in.</p>
        <ul class="facts">
          <li><svg lucideIcon="lock" [size]="16" aria-hidden="true"></svg>Processed in your browser</li>
          <li><svg lucideIcon="user-x" [size]="16" aria-hidden="true"></svg>No account needed</li>
          <li><svg lucideIcon="image" [size]="16" aria-hidden="true"></svg>JPG, PNG, WebP, MP4, WebM, MOV</li>
        </ul>
      </div>
    </header>
  `,
})
export class WatermarkHeroComponent {}

/** How it works and the FAQ, below the editor. */
@Component({
  selector: 'app-watermark-about',
  standalone: true,
  styles: [`
    :host { display: block; }
    .card { background: var(--color-paper); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); box-shadow: 0 1px 2px color-mix(in oklab, var(--color-ink) 5%, transparent), 0 12px 32px -18px color-mix(in oklab, var(--color-ink) 28%, transparent); }
    .wrap { width: min(80rem, 100%); margin-inline: auto; padding-inline: var(--space-md); }
    @media (min-width: 48rem) { .wrap { padding-inline: var(--space-xl); } }
    .steps { padding: var(--space-2xl) 0; }
    .steps h2, .faq h2 { margin: 0; font-family: var(--font-display); font-weight: 500; font-size: var(--type-section); line-height: 1.05; letter-spacing: var(--tracking-display); }
    .step-grid { margin: var(--space-lg) 0 0; padding: 0; list-style: none; display: grid; gap: var(--space-md); }
    @media (min-width: 48rem) { .step-grid { grid-template-columns: repeat(3, 1fr); } }
    .step { padding: var(--space-lg); }
    .step-n { width: 2.25rem; height: 2.25rem; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; background: var(--color-accent); color: var(--color-accent-ink); font-weight: 600; font-variant-numeric: lining-nums; }
    .step h3 { margin: var(--space-sm) 0 0; font-size: var(--type-lg); font-weight: 600; }
    .step p { margin: var(--space-2xs) 0 0; color: var(--color-ink-2); line-height: 1.55; }
    .note { margin: var(--space-md) 0 0; max-width: 70ch; color: var(--color-muted); font-size: var(--type-sm); }
    .faq { padding: 0 0 var(--space-3xl); }
    .faq-list { margin-top: var(--space-lg); display: flex; flex-direction: column; gap: var(--space-xs); max-width: 52rem; }
    .faq .ui-disclosure { background: var(--card); }
    .faq .ui-disclosure p { margin: 0; line-height: 1.6; }
  `],
  template: `
    <section class="steps" aria-labelledby="steps-heading">
      <div class="wrap">
        <h2 id="steps-heading">Three steps. Nothing leaves your device.</h2>
        <ol class="step-grid">
          <li class="card step">
            <span class="step-n" aria-hidden="true">1</span>
            <h3>Open a file</h3>
            <p>Drop in an image or a video. It is read by your browser, not sent to a server.</p>
          </li>
          <li class="card step">
            <span class="step-n" aria-hidden="true">2</span>
            <h3>Place the mark</h3>
            <p>Drag the watermark anywhere on the preview. Set the text, font, size, opacity, colour and rotation, or use a logo image.</p>
          </li>
          <li class="card step">
            <span class="step-n" aria-hidden="true">3</span>
            <h3>Download it</h3>
            <p>Images export at full resolution as PNG or JPEG. Video is rendered locally with FFmpeg and exported as MP4.</p>
          </li>
        </ol>
        <p class="note">No account, no upload, no queue. Images work with the network off once the page has loaded, and the first video export downloads a processing engine of about 30 MB.</p>
      </div>
    </section>
    <section id="faq" class="faq" aria-labelledby="faq-heading">
      <div class="wrap">
        <h2 id="faq-heading">Before you watermark</h2>
        <div class="faq-list">
          @for (f of faqs(); track f.q) {
            <details class="ui-disclosure">
              <summary>{{ f.q }}</summary>
              <p>{{ f.a }}</p>
            </details>
          }
        </div>
      </div>
    </section>
  `,
})
export class WatermarkAboutComponent {
  readonly faqs = input.required<{ q: string; a: string }[]>();
}
