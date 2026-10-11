import {
  Component,
  ElementRef,
  OnDestroy,
  PLATFORM_ID,
  AfterViewInit,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { isPlatformBrowser } from '@angular/common';
import type { PageFlip } from 'page-flip';

type GsapModule = typeof import('./gsap-landing');

/**
 * "From a TXT file to a narrated video" page-flip book.
 *
 * Without JavaScript, or if a library fails to load, the eight pages stay as an
 * ordinary stacked list. page-flip and GSAP are loaded only when the section is
 * close to the viewport, and nothing here runs on the server.
 */
@Component({
  selector: 'app-flipbook',
  standalone: true,
  styles: [
    `
      :host {
        display: block;
      }
      .fb-stage {
        perspective: 1100px;
      }
      .fb-book {
        width: 100%;
        max-width: 500px;
        margin-inline: auto;
      }
      /* Pending reserves the live book's exact box, so loading the library moves nothing. */
      .fb-book.fb-live,
      .fb-book.fb-pending {
        aspect-ratio: 3 / 4;
        container-type: inline-size;
      }
      .fb-book.fb-pending {
        overflow: hidden;
      }
      .fb-book.fb-pending .fb-page {
        height: 100%;
      }
      .fb-book.fb-pending .fb-page:not(:first-child) {
        display: none;
      }
      .fb-page {
        box-sizing: border-box;
        overflow: hidden;
        background: #faf7f2;
        color: #1a1714;
        border: 1px solid rgba(26, 23, 20, 0.1);
        border-radius: 16px;
        line-height: 1.5;
      }
      .fb-book:not(.fb-live):not(.fb-pending) .fb-page {
        margin-bottom: 14px;
        font-size: 15px;
        padding: 22px;
      }
      .fb-book.fb-live .fb-page,
      .fb-book.fb-pending .fb-page {
        padding: 7cqw;
        font-size: 4.2cqw;
        border-radius: 0;
        border: 0;
        box-shadow: inset 0 0 0 1px rgba(26, 23, 20, 0.1);
      }
      .fb-hard {
        background: #1a1714;
        color: #faf7f2;
        border-color: rgba(255, 255, 255, 0.12);
      }
      .fb-kicker {
        font-size: 0.78em;
        color: #6b6560;
        margin: 0;
      }
      .fb-hard .fb-kicker {
        color: rgba(250, 247, 242, 0.7);
      }
      .fb-title {
        font-family: 'Cormorant Garamond', Georgia, serif;
        font-weight: 500;
        font-size: 1.9em;
        line-height: 1.05;
        margin: 0.35em 0 0.5em;
        letter-spacing: -0.01em;
      }
      .fb-cover-title {
        font-size: 2.5em;
        margin-top: 2.2em;
      }
      .fb-hard p {
        margin: 0;
      }
      .fb-pre {
        font-family: ui-monospace, 'SFMono-Regular', Menlo, Consolas, monospace;
        font-size: 0.86em;
        line-height: 1.55;
        background: #f3eee3;
        border-radius: 10px;
        padding: 0.9em 1em;
        margin: 0 0 0.9em;
        white-space: pre-wrap;
        color: #1a1714;
      }
      .fb-note {
        margin: 0;
        color: #6b6560;
        font-size: 0.9em;
      }
      .fb-list {
        list-style: none;
        margin: 0 0 0.9em;
        padding: 0;
      }
      .fb-list li {
        padding: 0.55em 0;
        border-top: 1px solid rgba(26, 23, 20, 0.12);
      }
      .fb-list li:last-child {
        border-bottom: 1px solid rgba(26, 23, 20, 0.12);
      }
      .fb-list b {
        font-weight: 600;
        margin-right: 0.4em;
      }
      .fb-img {
        display: block;
        width: 100%;
        height: auto;
        border-radius: 8px;
        border: 1px solid rgba(26, 23, 20, 0.15);
        margin: 0.3em 0 0.8em;
      }
      .fb-controls {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 14px;
        margin-top: 18px;
      }
      .fb-btn {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-width: 44px;
        min-height: 44px;
        padding: 0 20px;
        border-radius: 999px;
        border: 0;
        background: #1a1714;
        color: #faf7f2;
        font: inherit;
        font-size: 14px;
        font-weight: 600;
        cursor: pointer;
        transition: background-color 0.2s;
      }
      .fb-btn:hover:not(:disabled) {
        background: #2a2620;
      }
      .fb-btn:disabled {
        opacity: 0.4;
        cursor: default;
      }
      .fb-btn:focus-visible,
      .fb-stage:focus-visible {
        outline: 2px solid #a8481f;
        outline-offset: 3px;
      }
      .fb-count {
        min-width: 7.5em;
        text-align: center;
        font-size: 14px;
        color: #6b6560;
      }
      @media (prefers-reduced-motion: reduce) {
        .fb-btn {
          transition: none;
        }
      }
    `,
  ],
  template: `
    <section aria-label="An example, page by page: from a TXT file to a narrated video">
      <div
        #stage
        class="fb-stage"
        [attr.tabindex]="live() ? 0 : null"
        [attr.role]="live() ? 'group' : null"
        [attr.aria-roledescription]="live() ? 'book' : null"
        [attr.aria-label]="live() ? 'Example book. Use the left and right arrow keys to turn pages.' : null"
        (keydown)="onKey($event)"
      >
        <div #book class="fb-book" [class.fb-live]="live()" [class.fb-pending]="pending()">
          <div class="fb-page fb-hard" data-density="hard">
            <p class="fb-kicker">A worked example</p>
            <h3 class="fb-title fb-cover-title">From a text file to a narrated video</h3>
            <p class="fb-kicker">One real file, turned into the slides your video is made of.</p>
          </div>

          <div class="fb-page">
            <p class="fb-kicker">1 · Your file</p>
            <h3 class="fb-title">Plain text</h3>
            <pre class="fb-pre">1. What is the capital of France?
a) London
b) Paris
c) Berlin
d) Madrid
Answer: b</pre>
            <p class="fb-note">A numbered question, lettered options and an Answer line.</p>
          </div>

          <div class="fb-page">
            <p class="fb-kicker">2 · What the parser reads</p>
            <h3 class="fb-title">Question, options, answer</h3>
            <ul class="fb-list">
              <li><b>Question</b>1. What is the capital of France?</li>
              <li><b>Options</b>a to d, four in total</li>
              <li><b>Answer</b>b, Paris</li>
            </ul>
            <p class="fb-note">Numbering, options and answer keys are detected when you upload.</p>
          </div>

          <div class="fb-page">
            <p class="fb-kicker">3 · The question slide</p>
            <h3 class="fb-title">Question first</h3>
            <img
              class="fb-img"
              src="landing/slide-question.webp"
              width="1280"
              height="720"
              loading="lazy"
              decoding="async"
              alt="Question slide made by Indifferent from the sample file: Question 1 of 3, What is the capital of France? Options A London, B Paris, C Berlin, D Madrid."
            />
            <p class="fb-note">Shown on screen while the narrator reads the question aloud.</p>
          </div>

          <div class="fb-page">
            <p class="fb-kicker">4 · The answer slide</p>
            <h3 class="fb-title">Then the answer</h3>
            <img
              class="fb-img"
              src="landing/slide-answer.webp"
              width="1280"
              height="720"
              loading="lazy"
              decoding="async"
              alt="Answer slide made by Indifferent from the sample file: option B, Paris, is shown in green as the correct answer."
            />
            <p class="fb-note">The correct option turns green and the narrator reads the answer.</p>
          </div>

          <div class="fb-page">
            <p class="fb-kicker">5 · The video</p>
            <h3 class="fb-title">One MP4</h3>
            <ul class="fb-list">
              <li>Each question: question slide, then answer slide</li>
              <li>Both narrated in the voice you pick</li>
              <li>Joined into a single 1080p MP4</li>
            </ul>
            <p class="fb-note">Download it, watermark it if you like, and post it.</p>
          </div>

          <div class="fb-page">
            <p class="fb-kicker">6 · The voices</p>
            <h3 class="fb-title">Five narrators</h3>
            <ul class="fb-list">
              <li><b>Joanna</b>US English</li>
              <li><b>Matthew</b>US English</li>
              <li><b>Amy</b>British English</li>
              <li><b>Brian</b>British English</li>
              <li><b>Aditi</b>Indian English</li>
            </ul>
          </div>

          <div class="fb-page fb-hard" data-density="hard">
            <p class="fb-kicker">Your turn</p>
            <h3 class="fb-title fb-cover-title">Try it with your own file</h3>
            <p class="fb-kicker">Sign in, upload a TXT file, and pick a voice.</p>
          </div>
        </div>
      </div>

      @if (live() || pending()) {
        <div class="fb-controls">
          <button type="button" class="fb-btn" (click)="prev()" [disabled]="pending() || pageNo() === 1" aria-label="Previous page">
            Previous
          </button>
          <span class="fb-count" aria-live="polite">{{ pending() ? 'Loading the example' : 'Page ' + pageNo() + ' of ' + count() }}</span>
          <button type="button" class="fb-btn" (click)="next()" [disabled]="pending() || pageNo() === count()" aria-label="Next page">
            Next
          </button>
        </div>
      }
    </section>
  `,
})
export class FlipbookComponent implements AfterViewInit, OnDestroy {
  protected readonly live = signal(false);
  protected readonly pending = signal(false);
  protected readonly pageNo = signal(1);
  protected readonly count = signal(8);

  private readonly isBrowser = isPlatformBrowser(inject(PLATFORM_ID));
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly stage = viewChild.required<ElementRef<HTMLElement>>('stage');
  private readonly book = viewChild.required<ElementRef<HTMLElement>>('book');

  private flip?: PageFlip;
  private io?: IntersectionObserver;
  private ctx?: { revert(): void };
  private destroyed = false;
  private touched = false;
  private reduce = false;

  ngAfterViewInit(): void {
    if (!this.isBrowser) return;
    this.reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (!('IntersectionObserver' in window)) return;
    // From here the page has JavaScript, so hold the book's final shape instead of the long stacked list.
    this.pending.set(true);
    this.io = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          this.io?.disconnect();
          void this.init();
        }
      },
      { rootMargin: '600px 0px' },
    );
    this.io.observe(this.host.nativeElement);
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.io?.disconnect();
    this.ctx?.revert();
    try {
      this.flip?.destroy();
    } catch {
      /* the library may already have removed its own nodes */
    }
  }

  protected prev(): void {
    this.touched = true;
    if (!this.flip) return;
    if (this.reduce) this.turn(-1);
    else this.flip.flipPrev();
  }

  protected next(): void {
    this.touched = true;
    if (!this.flip) return;
    if (this.reduce) this.turn(1);
    else this.flip.flipNext();
  }

  protected onKey(e: KeyboardEvent): void {
    if (!this.live()) return;
    if (e.key === 'ArrowRight') {
      e.preventDefault();
      if (this.pageNo() < this.count()) this.next();
    } else if (e.key === 'ArrowLeft') {
      e.preventDefault();
      if (this.pageNo() > 1) this.prev();
    }
  }

  private turn(delta: number): void {
    const target = Math.min(Math.max(this.flip!.getCurrentPageIndex() + delta, 0), this.flip!.getPageCount() - 1);
    this.flip!.turnToPage(target);
    this.pageNo.set(target + 1);
  }

  private async init(): Promise<void> {
    try {
      const [flipMod, g] = await Promise.all([
        import('page-flip') as Promise<Record<string, unknown>>,
        import('./gsap-landing'),
      ]);
      if (this.destroyed) return;
      const Ctor = (flipMod['PageFlip'] ?? (flipMod['default'] as Record<string, unknown> | undefined)?.['PageFlip']) as
        | (new (el: HTMLElement, s: object) => PageFlip)
        | undefined;
      if (!Ctor) throw new Error('page-flip export not found');

      const bookEl = this.book().nativeElement;
      const pages = Array.from(bookEl.querySelectorAll<HTMLElement>(':scope > .fb-page'));
      // Already the right shape (pending), so page-flip can measure it right away.
      this.live.set(true);
      this.pending.set(false);
      await new Promise<void>((r) => requestAnimationFrame(() => r()));
      if (this.destroyed) return;

      this.flip = new Ctor(bookEl, {
        width: 360,
        height: 480,
        size: 'stretch',
        minWidth: 300,
        maxWidth: 480,
        minHeight: 320,
        maxHeight: 640,
        showCover: true,
        usePortrait: true,
        drawShadow: !this.reduce,
        maxShadowOpacity: 0.35,
        flippingTime: this.reduce ? 1 : 850,
        showPageCorners: !this.reduce,
        mobileScrollSupport: true,
        clickEventForward: true,
        autoSize: true,
      });
      this.flip.loadFromHTML(pages);
      this.count.set(this.flip.getPageCount());
      this.flip.on('flip', (e) => this.pageNo.set(Number(e.data) + 1));

      if (!this.reduce) this.motion(g);
    } catch {
      // Fall back to the stacked list.
      this.pending.set(false);
      this.live.set(false);
    }
  }

  private motion(g: GsapModule): void {
    const { gsap, ScrollTrigger } = g;
    const stageEl = this.stage().nativeElement;
    this.ctx = gsap.context(() => {
      // The book settles flat as it scrolls into place.
      gsap.fromTo(
        stageEl,
        { rotateX: 7, y: 26, transformOrigin: '50% 100%' },
        {
          rotateX: 0,
          y: 0,
          ease: 'none',
          scrollTrigger: { trigger: stageEl, start: 'top 95%', end: 'top 55%', scrub: true },
        },
      );
      // Open the cover once, unless the visitor has already started turning pages.
      ScrollTrigger.create({
        trigger: stageEl,
        start: 'top 60%',
        once: true,
        onEnter: () => {
          gsap.delayedCall(0.7, () => {
            if (!this.touched && !this.destroyed && this.flip?.getCurrentPageIndex() === 0) this.flip.flipNext();
          });
        },
      });
    }, this.host.nativeElement);
  }
}
