import { isPlatformBrowser } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  OnDestroy,
  PLATFORM_ID,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { CANVAS_STYLES } from './canvas.styles';
import { DecorGlyphComponent } from './decor-glyph.component';
import { glyphById } from './decor-library';
import { InfographicChartComponent } from './infographic-chart.component';
import { InfographicDiagramComponent, LIST_LAYOUTS, ListLayout, MAP_LAYOUTS, MapLayout } from './infographic-diagram.component';
import { InfographicMapComponent } from './infographic-map.component';
import { docText } from './doc-ops';
import { DecorColor, Decoration, FORMATS, InfographicDoc, MARK_TEXT } from './infographic.model';
import { TEMPLATES, Template, templateById } from './templates';
import { themeById } from './themes';
import { detectDirection, detectLanguage, scriptsIn } from './text-utils';

export interface FitResult {
  /** The scale the text was brought to, 1 being the design size. Above 1 the text was enlarged to fill the image. */
  k: number;
  /** How far the content still runs past the artboard at the smallest scale, in design pixels. 0 when it fits. */
  overflowPx: number;
}

/**
 * The text scale tried, largest first: sparse content grows to fill the image
 * (up to 1.4 times the design size) and crowded content shrinks until it fits
 * (down to 0.58 times). The first that fits is used.
 */
export const FIT_STEPS = [1.4, 1.32, 1.24, 1.16, 1.08, 1, 0.94, 0.88, 0.82, 0.76, 0.7, 0.64, 0.58];

/** The artboard of the infographic maker: draws the document at its design size, scaled to fit the screen. */
@Component({
  selector: 'app-infographic-canvas',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [InfographicChartComponent, InfographicDiagramComponent, InfographicMapComponent, DecorGlyphComponent],
  styles: [CANVAS_STYLES],
  template: `
    <div class="holder" [style.margin-inline]="'auto'" [style.width.px]="size().width * scale()" [style.height.px]="size().height * scale()">
      <div
        #stage
        [class]="'stage f-' + doc().format + ' l-' + template().layout"
        [attr.lang]="lang()"
        [attr.dir]="dir()"
        [attr.data-testid]="'artboard'"
        [style]="vars()"
        [style.width.px]="size().width"
        [style.height.px]="size().height"
        [style.transform]="scale() < 1 ? 'scale(' + scale() + ')' : null"
      >
        <header [class.head]="template().layout === 'concept'">
          @if (!titleInside()) {
            <p class="title" dir="auto">{{ doc().title }}</p>
          }
          @if (doc().subtitle) {
            <p class="sub" dir="auto">{{ doc().subtitle }}</p>
          }
        </header>

        @if (listLayout(); as d) {
          <app-infographic-diagram [layout]="d" [doc]="doc()" [rtl]="dir() === 'rtl'" />
        } @else if (mapLayout(); as m) {
          <app-infographic-map [layout]="m" [doc]="doc()" [rtl]="dir() === 'rtl'" />
        } @else {
        @switch (template().layout) {
          @case ('compare') {
            <div class="list">
              @for (s of doc().sections; track s.id) {
                <section class="card side">
                  <div class="band"><h2 dir="auto">{{ s.heading }}</h2></div>
                  <ul class="bul">
                    @for (line of lines(s.body); track $index) {
                      <li dir="auto">{{ line }}</li>
                    }
                  </ul>
                </section>
              }
            </div>
          }
          @case ('concept') {
            <div class="list">
              @for (s of doc().sections; track s.id) {
                <section class="card">
                  <h2 dir="auto">{{ s.heading }}</h2>
                  <p class="body" dir="auto">{{ s.body }}</p>
                </section>
              }
            </div>
          }
          @case ('timeline') {
            <ol class="list timeline">
              @for (s of doc().sections; track s.id) {
                <li class="ev">
                  @if (s.label) {
                    <span class="when" dir="auto">{{ s.label }}</span>
                  }
                  <h2 dir="auto">{{ s.heading }}</h2>
                  <p class="body" dir="auto">{{ s.body }}</p>
                </li>
              }
            </ol>
          }
          @case ('bars') {
            <app-infographic-chart layout="bars" [doc]="doc()" [rtl]="dir() === 'rtl'" />
          }
          @case ('line') {
            <app-infographic-chart layout="line" [doc]="doc()" [rtl]="dir() === 'rtl'" />
          }
          @case ('donut') {
            <app-infographic-chart layout="donut" [doc]="doc()" [rtl]="dir() === 'rtl'" />
          }
          @case ('numbers') {
            <app-infographic-chart layout="numbers" [doc]="doc()" [rtl]="dir() === 'rtl'" />
          }
          @default {
            <ol class="list">
              @for (s of doc().sections; track s.id; let i = $index) {
                <li class="item">
                  <span class="num" aria-hidden="true">{{ i + 1 }}</span>
                  <div class="txt">
                    @if (s.label) {
                      <p class="cap" dir="auto">{{ s.label }}</p>
                    }
                    <h2 dir="auto">{{ s.heading }}</h2>
                    <p class="body" dir="auto">{{ s.body }}</p>
                  </div>
                </li>
              }
            </ol>
          }
        }
        }
        <footer>
          @if (doc().footer) {
            <span dir="auto">{{ doc().footer }}</span>
          }
          @if (doc().mark) {
            <span class="mark">{{ mark }}</span>
          }
        </footer>
        <div class="decos">
          @for (d of doc().items; track d.id) {
            <div
              class="deco"
              role="button"
              tabindex="0"
              data-testid="deco"
              [attr.data-id]="d.id"
              [class.sel]="selected() === d.id"
              [style.left.%]="d.x"
              [style.top.%]="d.y"
              [style.width.%]="d.size"
              [style.color]="color(d.color)"
              [attr.aria-label]="describe(d)"
              (pointerdown)="down($event, d)"
              (pointermove)="move($event, d)"
              (pointerup)="up($event)"
              (pointercancel)="up($event)"
              (keydown)="key($event, d)"
              (focus)="decoSelected.emit(d.id)"
            >
              <app-decor-glyph [glyph]="glyphOf(d)" />
              <span class="ring" data-skip-export aria-hidden="true"></span>
            </div>
          }
        </div>
      </div>
    </div>
  `,
})
export class InfographicCanvasComponent implements OnDestroy {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);
  private readonly browser = isPlatformBrowser(inject(PLATFORM_ID));
  private readonly stageRef = viewChild.required<ElementRef<HTMLElement>>('stage');

  readonly doc = input.required<InfographicDoc>();
  /** Told after every fit, so the page can warn when the text does not fit. */
  readonly fitted = output<FitResult>();
  /** The icon or shape that has the ring round it. */
  readonly selected = input<string | null>(null);
  /** An icon or shape was dragged or moved with the keys: where its centre is now, in percent. */
  readonly decoMoved = output<{ id: string; x: number; y: number }>();
  /** An icon or shape was moved by the keys, by this many percent. The page applies it to the latest document, so quick key repeats add up. */
  readonly decoNudged = output<{ id: string; dx: number; dy: number }>();
  readonly decoSelected = output<string>();
  readonly decoRemoved = output<string>();
  /** The pointer was let go after dragging, which ends one undo step. */
  readonly decoDropped = output<void>();

  protected readonly mark = MARK_TEXT;
  protected readonly template = computed<Template>(() => templateById(this.doc().templateId) ?? TEMPLATES[0]);
  protected readonly size = computed(() => FORMATS[this.doc().format]);
  /** The layout when it is one of the diagrams, which the diagram components draw. */
  protected readonly listLayout = computed<ListLayout | null>(() => (LIST_LAYOUTS.includes(this.template().layout) ? (this.template().layout as ListLayout) : null));
  protected readonly mapLayout = computed<MapLayout | null>(() => (MAP_LAYOUTS.includes(this.template().layout) ? (this.template().layout as MapLayout) : null));
  /** The mind map and the tree draw the title as their centre or top, so the header leaves it out. */
  protected readonly titleInside = computed(() => this.mapLayout() === 'mindmap' || this.mapLayout() === 'tree');
  protected readonly scale = signal(1);
  protected readonly k = signal(FIT_STEPS[0]);

  /**
   * What the layout depends on. Moving an icon changes the document but not the layout, so the text is not measured again for it:
   * this string stays the same while an icon is dragged, and the effect that fits the text does not run.
   */
  private readonly fitKey = computed(() => {
    const d = this.doc();
    return JSON.stringify([d.templateId, d.format, d.themeId, d.language, d.direction, d.title, d.subtitle, d.footer, d.mark, d.sections]);
  });
  /** Everything written on the artboard, to tell the language, direction and line height. */
  readonly allText = computed(() => docText(this.doc()));
  protected readonly lang = computed(() => (this.doc().language !== 'auto' ? this.doc().language : detectLanguage(this.allText())));
  protected readonly dir = computed(() => {
    const d = this.doc();
    return d.direction !== 'auto' ? d.direction : detectDirection(`${d.title} ${d.sections[0]?.heading ?? ''} ${d.subtitle}`);
  });
  /** Indic, Arabic and Thai letters carry marks above and below, so their lines are taller. */
  private readonly lineHeight = computed(() => {
    const scripts = scriptsIn(this.allText());
    const tall = ['devanagari', 'gujarati', 'bengali', 'gurmukhi', 'odia', 'tamil', 'telugu', 'kannada', 'malayalam', 'thai', 'arabic'];
    if (tall.some((s) => scripts.has(s as never))) return 1.75;
    return scripts.has('han') || scripts.has('kana') || scripts.has('hangul') ? 1.65 : 1.45;
  });
  protected readonly vars = computed(() => {
    const t = themeById(this.doc().themeId);
    return {
      '--bg': t.bg,
      '--surface': t.surface,
      '--ink': t.ink,
      '--ink2': t.ink2,
      '--accent': t.accent,
      '--accent-ink': t.accentInk,
      '--rule': t.rule,
      '--lh': String(this.lineHeight()),
      '--k': String(this.k()),
    };
  });

  private resizeObserver?: ResizeObserver;
  private pending = 0;
  private readonly onFonts = () => this.scheduleFit();
  private readonly onResize = () => this.updateScale();

  constructor() {
    // Fit again whenever what is drawn changes.
    effect(() => {
      this.fitKey();
      untracked(() => {
        this.updateScale();
        afterNextRender(() => this.fit(), { injector: this.injector });
      });
    });
    afterNextRender(() => {
      if (!this.browser) return;
      this.updateScale();
      if (typeof ResizeObserver !== 'undefined') {
        this.resizeObserver = new ResizeObserver(() => this.updateScale());
        this.resizeObserver.observe(this.host.nativeElement);
      }
      this.host.nativeElement.ownerDocument.defaultView?.addEventListener('resize', this.onResize);
      const fonts = this.host.nativeElement.ownerDocument.fonts;
      fonts?.addEventListener?.('loadingdone', this.onFonts);
      fonts?.ready.then(() => this.scheduleFit());
    });
  }

  ngOnDestroy(): void {
    this.resizeObserver?.disconnect();
    this.host.nativeElement.ownerDocument.defaultView?.removeEventListener('resize', this.onResize);
    this.host.nativeElement.ownerDocument.fonts?.removeEventListener?.('loadingdone', this.onFonts);
    if (this.pending) cancelAnimationFrame(this.pending);
  }

  /** The element to export. */
  stageElement(): HTMLElement {
    return this.stageRef().nativeElement;
  }

  /** Fits the artboard to the width of its box and to most of the window's height, so a tall size never needs a long scroll. */
  private updateScale(): void {
    const available = this.host.nativeElement.clientWidth;
    if (available <= 0) return;
    const win = this.host.nativeElement.ownerDocument.defaultView;
    const byHeight = win ? (win.innerHeight * 0.8) / this.size().height : 1;
    this.scale.set(Math.min(1, available / this.size().width, byHeight));
  }

  private scheduleFit(): void {
    if (this.pending) return;
    this.pending = requestAnimationFrame(() => {
      this.pending = 0;
      this.fit();
    });
  }

  /** How far content runs past the artboard, or a heading's word runs past its box, in design pixels. */
  private static overflowOf(stage: HTMLElement): number {
    let over = Math.max(stage.scrollHeight - stage.clientHeight, stage.scrollWidth - stage.clientWidth, 0);
    stage.querySelectorAll<HTMLElement>('.title, h2, .big, .val').forEach((h) => {
      over = Math.max(over, h.scrollWidth - h.clientWidth);
    });
    return Math.max(over, InfographicCanvasComponent.overlapOf(stage));
  }

  /** How far two marked boxes (`data-node`: the nodes of a cycle) run into each other, in design pixels. 0 when none do. */
  private static overlapOf(stage: HTMLElement): number {
    const nodes = Array.from(stage.querySelectorAll<HTMLElement>('[data-node]'));
    if (nodes.length < 2) return 0;
    // The preview is scaled down; measure in design pixels.
    const scale = stage.offsetWidth > 0 ? stage.getBoundingClientRect().width / stage.offsetWidth : 1;
    const rects = nodes.map((n) => n.getBoundingClientRect());
    let worst = 0;
    for (let i = 0; i < rects.length; i++) {
      for (let j = i + 1; j < rects.length; j++) {
        const w = Math.min(rects[i].right, rects[j].right) - Math.max(rects[i].left, rects[j].left);
        const h = Math.min(rects[i].bottom, rects[j].bottom) - Math.max(rects[i].top, rects[j].top);
        if (w > 0 && h > 0) worst = Math.max(worst, Math.min(w, h) / scale);
      }
    }
    return worst > 1 ? worst : 0;
  }

  /**
   * Lowers the text scale until the content fits the artboard, as far as the
   * smallest step. It works on the real element, so it sees what the browser
   * lays out with the fonts that are loaded now.
   */
  fit(): FitResult {
    const stage = this.stageRef().nativeElement;
    stage.removeAttribute('data-break');
    let result: FitResult = { k: FIT_STEPS[0], overflowPx: 0 };
    for (const k of FIT_STEPS) {
      stage.style.setProperty('--k', String(k));
      const over = InfographicCanvasComponent.overflowOf(stage);
      result = { k, overflowPx: over > 1 ? over : 0 };
      if (result.overflowPx === 0) break;
    }
    if (result.overflowPx > 0) {
      // Smallest size and still too wide: let long words break, so nothing is cut off sideways.
      stage.setAttribute('data-break', '');
      const over = InfographicCanvasComponent.overflowOf(stage);
      result = { k: result.k, overflowPx: over > 1 ? over : 0 };
    }
    this.k.set(result.k);
    this.fitted.emit(result);
    return result;
  }

  // --- icons and shapes

  protected glyphOf(d: Decoration) {
    return glyphById(d.glyph);
  }

  protected color(c: DecorColor): string {
    return c === 'accent' ? 'var(--accent)' : c === 'ink' ? 'var(--ink)' : 'var(--ink2)';
  }

  protected describe(d: Decoration): string {
    const name = glyphById(d.glyph)?.label ?? 'Icon';
    return `${name}, ${Math.round(d.x)} percent across and ${Math.round(d.y)} percent down. Move it with the arrow keys, or press Delete to remove it.`;
  }

  private drag: { id: string; startX: number; startY: number; x: number; y: number; pointer: number } | null = null;

  protected down(e: PointerEvent, d: Decoration): void {
    if (e.button !== 0) return;
    this.decoSelected.emit(d.id);
    this.drag = { id: d.id, startX: e.clientX, startY: e.clientY, x: d.x, y: d.y, pointer: e.pointerId };
    this.capture(e, true);
    e.preventDefault();
  }

  /** The pointer's travel is measured on the screen, where the artboard is scaled down, so it is turned into percent of the artboard as it is shown. */
  protected move(e: PointerEvent, d: Decoration): void {
    if (!this.drag || this.drag.id !== d.id || this.drag.pointer !== e.pointerId) return;
    const box = this.stageRef().nativeElement.getBoundingClientRect();
    if (box.width <= 0 || box.height <= 0) return;
    const x = Math.min(100, Math.max(0, this.drag.x + ((e.clientX - this.drag.startX) / box.width) * 100));
    const y = Math.min(100, Math.max(0, this.drag.y + ((e.clientY - this.drag.startY) / box.height) * 100));
    this.decoMoved.emit({ id: d.id, x, y });
  }

  /** Keeps the pointer's moves coming to the icon while it is dragged past its own edge. A pointer that is already gone makes the browser throw; the drag still works without capture. */
  private capture(e: PointerEvent, on: boolean): void {
    const target = e.currentTarget as HTMLElement;
    try {
      if (on) target.setPointerCapture?.(e.pointerId);
      else target.releasePointerCapture?.(e.pointerId);
    } catch {
      /* the pointer is no longer active */
    }
  }

  protected up(e: PointerEvent): void {
    if (this.drag && this.drag.pointer === e.pointerId) {
      this.capture(e, false);
      this.drag = null;
      this.decoDropped.emit();
    }
  }

  protected key(e: KeyboardEvent, d: Decoration): void {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    const step = e.shiftKey ? 5 : 1;
    const dx = e.key === 'ArrowLeft' ? -step : e.key === 'ArrowRight' ? step : 0;
    const dy = e.key === 'ArrowUp' ? -step : e.key === 'ArrowDown' ? step : 0;
    if (dx || dy) {
      this.decoNudged.emit({ id: d.id, dx, dy });
      e.preventDefault();
    } else if (e.key === 'Delete' || e.key === 'Backspace') {
      this.decoRemoved.emit(d.id);
      e.preventDefault();
    }
  }

  protected lines(body: string): string[] {
    return body.split('\n').map((l) => l.trim()).filter(Boolean);
  }
}
