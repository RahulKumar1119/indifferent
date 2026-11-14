import { DOCUMENT } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, HostListener, Injector, OnInit, afterNextRender, computed, inject, signal, viewChild } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { PublicHeaderComponent } from '../../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../../shared/components/site-footer/site-footer.component';
import { ROBOTS_NOINDEX, setPageSeo, setRobots } from '../../../shared/seo';
import { dataIssues } from './chart-data';
import { DraftStore } from './draft-store';
import { docText, removeDecoration, replacesSections, resetDesign, startNew, updateDecoration } from './doc-ops';
import { DocHistory } from './history';
import { DocChange, InfographicDesignPanelComponent } from './infographic-design-panel.component';
import { InfographicContentPanelComponent } from './infographic-content-panel.component';
import { InfographicDecorPanelComponent } from './infographic-decor-panel.component';
import { FitResult, InfographicCanvasComponent } from './infographic-canvas.component';
import { ExportKind, InfographicExporter, downloadBlob } from './infographic-export.service';
import { InfographicFonts } from './infographic-fonts.service';
import { FORMATS, InfographicDoc } from './infographic.model';
import { unreleasedScripts } from './languages';
import { TEMPLATES, Template, newDoc, templateById } from './templates';
import { SCRIPT_NAMES, detectLanguage, scriptsIn } from './text-utils';

const SAVE_DELAY_MS = 500;
type Message = { kind: 'ok' | 'warn' | 'error'; text: string } | null;

/** The infographic maker page (#217): a template, the words, a size and colours, then a PNG or JPG made in this browser. */
@Component({
  selector: 'app-infographic-maker',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [LucideDynamicIcon, PublicHeaderComponent, SiteFooterComponent, InfographicCanvasComponent, InfographicDesignPanelComponent, InfographicContentPanelComponent, InfographicDecorPanelComponent],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper-2); color: var(--color-ink); font-family: var(--font-body); min-height: 100dvh; -webkit-font-smoothing: antialiased; }
    .wrap { width: min(80rem, 100%); margin-inline: auto; padding: calc(56px + var(--space-lg)) var(--space-md) var(--space-2xl); }
    @media (min-width: 48rem) { .wrap { padding-inline: var(--space-xl); } }
    .intro { display: flex; flex-direction: column; gap: var(--space-sm); margin-block-end: var(--space-md); }
    .local { margin: 0; display: flex; align-items: flex-start; gap: var(--space-xs); padding: var(--space-sm) var(--space-md); border-radius: var(--radius-control); background: color-mix(in oklab, var(--color-positive) 9%, var(--color-paper)); border: 1px solid color-mix(in oklab, var(--color-positive) 30%, transparent); font-size: var(--type-sm); }
    .local svg { flex: none; margin-block-start: 2px; color: var(--color-positive); }
    .bar { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-xs); margin-block-end: var(--space-md); }
    .bar .sp { flex: 1; }
    .status { font-size: var(--type-xs); color: var(--color-ink-2); }
    .confirm { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-xs); padding: var(--space-xs) var(--space-sm); border: 1px solid var(--color-rule); border-radius: var(--radius-control); background: var(--color-paper); font-size: var(--type-sm); }
    .grid { display: grid; gap: var(--space-md); align-items: start; grid-template-columns: minmax(0, 1fr); }
    @media (min-width: 64rem) { .grid { grid-template-columns: minmax(0, 1fr) 26rem; } .preview { position: sticky; top: calc(56px + var(--space-md)); } .controls { order: 2; } }
    .card { padding: var(--space-md); }
    .card h2 { margin: 0 0 var(--space-md); font-size: var(--type-lg); font-weight: 600; }
    .controls { display: flex; flex-direction: column; gap: var(--space-md); min-width: 0; }
    .preview { min-width: 0; }
    .acts { display: flex; flex-wrap: wrap; gap: var(--space-xs); margin-block-start: var(--space-md); }
    .note { margin: var(--space-sm) 0 0; }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />
      <main id="main" class="wrap">
        <header class="intro">
          <h1 class="ui-h1">Infographic maker</h1>
          <p class="ui-lead">Pick a template, write your text, and download a PNG, JPG or PDF. Free, no account.</p>
          <p class="local" data-testid="privacy">
            <svg lucideIcon="shield-check" [size]="18" aria-hidden="true"></svg>
            <span>
              Everything happens in this browser tab. Your text and your images are never uploaded. Your draft is kept in this browser's local storage,
              so a refresh does not lose it. Nothing is stored anywhere else.
            </span>
          </p>
        </header>

        <p class="sr-only" role="status" aria-live="polite" data-testid="announcement">{{ announcement() }}</p>

        <div class="bar" role="toolbar" aria-label="Edit">
          <button type="button" class="ui-btn ui-btn--secondary" [disabled]="!canUndo()" (click)="undo()">
            <svg lucideIcon="undo-2" [size]="16" aria-hidden="true"></svg> Undo
          </button>
          <button type="button" class="ui-btn ui-btn--secondary" [disabled]="!canRedo()" (click)="redo()">
            <svg lucideIcon="redo-2" [size]="16" aria-hidden="true"></svg> Redo
          </button>
          <button type="button" class="ui-btn ui-btn--secondary" (click)="resetDesign()">
            <svg lucideIcon="rotate-ccw" [size]="16" aria-hidden="true"></svg> Reset design
          </button>
          @if (!confirmingNew()) {
            <button type="button" id="ig-new" class="ui-btn ui-btn--secondary" (click)="askStartNew()">Start new</button>
          } @else {
            <div class="confirm" role="group" aria-label="Confirm start new">
              <span>Replace this infographic with a new one? You can still undo it.</span>
              <button type="button" id="ig-keep" class="ui-btn ui-btn--secondary" (click)="cancelStartNew()">Keep editing</button>
              <button type="button" class="ui-btn ui-btn--danger" (click)="startNew()">Start new</button>
            </div>
          }
          <span class="sp"></span>
          <span class="status" data-testid="save-status">{{ saveText() }}</span>
          @if (hasDraft()) {
            <button type="button" class="ui-btn ui-btn--quiet" (click)="clearDraft()">Clear saved draft</button>
          }
        </div>

        @if (restored()) {
          <p class="ui-alert" role="status" data-testid="restored">We restored your draft from this browser. Use Start new to begin again.</p>
        }

        <div class="grid">
          <section class="ui-card card preview" aria-labelledby="ig-preview-title">
            <h2 id="ig-preview-title">Preview</h2>
            <app-infographic-canvas
              [doc]="doc()"
              [selected]="selectedDeco()"
              (fitted)="onFit($event)"
              (decoMoved)="moveDeco($event)"
              (decoNudged)="nudgeDeco($event)"
              (decoSelected)="selectedDeco.set($event)"
              (decoRemoved)="removeDeco($event)"
              (decoDropped)="endDrag()"
            />
            @if (fit().overflowPx > 0) {
              <p class="ui-alert ui-alert--warn note" role="status" data-testid="overflow">
                There is too much text for this size, so part of it is cut off. Shorten some text, remove a section, or choose a taller size.
              </p>
            } @else if (fit().k < 1) {
              <p class="status note" data-testid="shrunk">The text was made smaller to fit this size.</p>
            }
            @if (issues().length > 0) {
              <div class="ui-alert ui-alert--warn note" role="status" data-testid="data-issues">
                <p>These numbers cannot be drawn as typed:</p>
                <ul>
                  @for (m of issues(); track m) {
                    <li>{{ m }}</li>
                  }
                </ul>
              </div>
            }
            @if (unsupported().length > 0) {
              <p class="ui-alert ui-alert--warn note" role="status" data-testid="unsupported">
                Fonts for {{ unsupported().join(', ') }} are not available yet. Your device's font is used instead, so the image may look different on other devices.
              </p>
            }
            <div class="acts">
              <button type="button" class="ui-btn ui-btn--primary" [disabled]="!!exporting()" [attr.aria-busy]="exporting() === 'png'" (click)="exportAs('png')" data-testid="export-png">
                <svg lucideIcon="download" [size]="16" aria-hidden="true"></svg>
                {{ exporting() === 'png' ? 'Making the PNG…' : 'Download PNG' }}
              </button>
              <button type="button" class="ui-btn ui-btn--secondary" [disabled]="!!exporting()" [attr.aria-busy]="exporting() === 'jpg'" (click)="exportAs('jpg')" data-testid="export-jpg">
                <svg lucideIcon="download" [size]="16" aria-hidden="true"></svg>
                {{ exporting() === 'jpg' ? 'Making the JPG…' : 'Download JPG' }}
              </button>
              <button type="button" class="ui-btn ui-btn--secondary" [disabled]="!!exporting()" [attr.aria-busy]="exporting() === 'pdf'" (click)="exportAs('pdf')" data-testid="export-pdf">
                <svg lucideIcon="download" [size]="16" aria-hidden="true"></svg>
                {{ exporting() === 'pdf' ? 'Making the PDF…' : 'Download PDF' }}
              </button>
            </div>
            <p class="status note">{{ size().width }} &times; {{ size().height }} pixels, made on your device.</p>
            @if (message(); as m) {
              <p class="ui-alert note" [class.ui-alert--warn]="m.kind === 'warn'" [class.ui-alert--bad]="m.kind === 'error'" role="status" data-testid="export-message">{{ m.text }}</p>
            }
          </section>

          <div class="controls">
            <section class="ui-card card" aria-labelledby="ig-design-title">
              <h2 id="ig-design-title">Design</h2>
              <app-infographic-design-panel [doc]="doc()" [detected]="detected()" (docChange)="commit($event)" />
            </section>
            <section class="ui-card card" aria-labelledby="ig-decor-title">
              <h2 id="ig-decor-title">Icons and shapes</h2>
              <app-infographic-decor-panel
                [doc]="doc()"
                [selected]="selectedDeco()"
                (docChange)="commit($event)"
                (announce)="announcement.set($event)"
                (pick)="selectedDeco.set($event)"
              />
            </section>
            <section class="ui-card card" aria-labelledby="ig-content-title">
              <h2 id="ig-content-title">Content</h2>
              <app-infographic-content-panel [doc]="doc()" [template]="template()" (docChange)="commit($event)" (announce)="announcement.set($event)" />
            </section>
          </div>
        </div>
      </main>
      <app-site-footer />
    </div>
  `,
})
export class InfographicMakerComponent implements OnInit {
  private readonly document = inject(DOCUMENT);
  private readonly injector = inject(Injector);
  private readonly store = inject(DraftStore);
  private readonly fonts = inject(InfographicFonts);
  private readonly exporter = inject(InfographicExporter);
  private readonly canvas = viewChild.required(InfographicCanvasComponent);

  private readonly history: DocHistory;
  protected readonly doc = signal<InfographicDoc>(newDoc(TEMPLATES[0]));
  protected readonly canUndo = signal(false);
  protected readonly canRedo = signal(false);
  protected readonly restored = signal(false);
  protected readonly hasDraft = signal(false);
  protected readonly saveState = signal<'idle' | 'saved' | 'blocked'>('idle');
  protected readonly confirmingNew = signal(false);
  protected readonly announcement = signal('');
  protected readonly fit = signal<FitResult>({ k: 1, overflowPx: 0 });
  protected readonly exporting = signal<ExportKind | null>(null);
  protected readonly message = signal<Message>(null);
  protected readonly selectedDeco = signal<string | null>(null);

  protected readonly template = computed<Template>(() => templateById(this.doc().templateId) ?? TEMPLATES[0]);
  protected readonly size = computed(() => FORMATS[this.doc().format]);
  private readonly scripts = computed(() => scriptsIn(docText(this.doc())));
  protected readonly issues = computed(() => dataIssues(this.doc(), this.template()));
  protected readonly detected = computed(() => detectLanguage(docText(this.doc())));
  protected readonly unsupported = computed(() => unreleasedScripts(this.scripts()).map((s) => SCRIPT_NAMES[s]));
  protected readonly saveText = computed(() => {
    if (!this.store.available || this.saveState() === 'blocked') return "This browser won't keep a draft.";
    return this.saveState() === 'saved' ? 'Draft saved in this browser.' : '';
  });

  private saveTimer: ReturnType<typeof setTimeout> | undefined;
  private dirty = false;

  constructor() {
    const draft = this.store.load();
    this.history = new DocHistory(draft ?? this.doc());
    if (draft) {
      this.doc.set(draft);
      this.restored.set(true);
    }
    this.hasDraft.set(this.store.has());
    inject(DestroyRef).onDestroy(() => this.flushSave());
  }

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Infographic maker | Indifferent',
      description: 'Make an infographic in your browser. Free, no account, nothing is uploaded.',
      canonical: 'https://indifferent.fun/tools/infographic-maker',
    });
    // Not listed or indexed until the tool is finished (#217).
    setRobots(this.document, ROBOTS_NOINDEX);
    void this.fonts.load();
  }

  // --- editing

  protected commit(change: DocChange): void {
    const swapped = change.doc.templateId !== this.doc().templateId && replacesSections(templateById(this.doc().templateId), templateById(change.doc.templateId) ?? TEMPLATES[0]);
    if (this.history.push(change.doc, change.key)) {
      if (swapped) this.announcement.set('Sections replaced with sample data for this kind of template. Undo brings your sections back.');
      this.restored.set(false);
      this.syncFromHistory();
    }
  }

  private syncFromHistory(): void {
    this.doc.set(this.history.present);
    this.canUndo.set(this.history.canUndo);
    this.canRedo.set(this.history.canRedo);
    this.scheduleSave();
  }

  protected undo(): void {
    if (this.history.undo()) {
      this.syncFromHistory();
      this.announcement.set('Undone.');
    }
  }

  protected redo(): void {
    if (this.history.redo()) {
      this.syncFromHistory();
      this.announcement.set('Redone.');
    }
  }

  protected resetDesign(): void {
    const next = resetDesign(this.doc());
    if (next === this.doc()) {
      this.announcement.set('The design is already the default.');
      return;
    }
    const hadItems = this.doc().items.length > 0;
    this.commit({ doc: next });
    this.announcement.set(hadItems ? 'Design reset. Icons and shapes were removed. Your text is unchanged.' : 'Design reset. Your text is unchanged.');
  }

  protected askStartNew(): void {
    this.confirmingNew.set(true);
    // The safe choice takes focus, as in the story editor.
    afterNextRender(() => this.document.getElementById('ig-keep')?.focus(), { injector: this.injector });
  }

  protected cancelStartNew(): void {
    this.confirmingNew.set(false);
    afterNextRender(() => this.document.getElementById('ig-new')?.focus(), { injector: this.injector });
  }

  protected startNew(): void {
    this.confirmingNew.set(false);
    this.commit({ doc: startNew(this.template()) });
    this.announcement.set('Started a new infographic. Undo brings the old one back.');
    afterNextRender(() => this.document.getElementById('ig-new')?.focus(), { injector: this.injector });
  }

  protected clearDraft(): void {
    clearTimeout(this.saveTimer);
    this.dirty = false;
    this.store.clear();
    this.hasDraft.set(false);
    this.saveState.set('idle');
    this.announcement.set('Saved draft cleared. Your next change will be saved again.');
  }

  /** Ctrl or Cmd with Z undoes, and with Shift (or Ctrl with Y) redoes, except while typing, where the box has its own undo. */
  @HostListener('document:keydown', ['$event'])
  protected onKey(e: KeyboardEvent): void {
    if (!(e.ctrlKey || e.metaKey) || e.altKey) return;
    const target = e.target as HTMLElement | null;
    if (target && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName))) return;
    const key = e.key.toLowerCase();
    if (key === 'z' && !e.shiftKey) this.undo();
    else if ((key === 'z' && e.shiftKey) || key === 'y') this.redo();
    else return;
    e.preventDefault();
  }

  // --- icons and shapes on the preview

  protected moveDeco(e: { id: string; x: number; y: number }): void {
    const next = updateDecoration(this.doc(), e.id, { x: e.x, y: e.y });
    // One drag is one undo step: moves of the same item close together are merged.
    if (next !== this.doc()) this.commit({ doc: next, key: `deco:${e.id}:move` });
  }

  /** Keys move an icon relative to where it is now, read from the latest document, so a held key adds up even before the screen has redrawn. */
  protected nudgeDeco(e: { id: string; dx: number; dy: number }): void {
    const item = this.doc().items.find((i) => i.id === e.id);
    if (!item) return;
    const next = updateDecoration(this.doc(), e.id, { x: item.x + e.dx, y: item.y + e.dy });
    if (next !== this.doc()) this.commit({ doc: next, key: `deco:${e.id}:move` });
  }

  /** Letting go of an icon ends the undo step, so two drags in a row are two steps. */
  protected endDrag(): void {
    this.history.endGroup();
  }

  protected removeDeco(id: string): void {
    const next = removeDecoration(this.doc(), id);
    if (next === this.doc()) return;
    this.commit({ doc: next });
    this.selectedDeco.set(null);
    this.announcement.set('Removed. Undo brings it back.');
  }

  // --- the draft

  /** Save a moment after the last change, not on every key. A draft just loaded is not saved again. */
  private scheduleSave(): void {
    this.dirty = true;
    clearTimeout(this.saveTimer);
    this.saveTimer = setTimeout(() => this.flushSave(), SAVE_DELAY_MS);
  }

  private flushSave(): void {
    clearTimeout(this.saveTimer);
    if (!this.dirty) return;
    this.dirty = false;
    const ok = this.store.save(this.doc());
    this.saveState.set(ok ? 'saved' : 'blocked');
    this.hasDraft.set(ok);
  }

  // --- the preview and the image

  protected onFit(result: FitResult): void {
    this.fit.set(result);
  }

  protected async exportAs(kind: ExportKind): Promise<void> {
    if (this.exporting()) return;
    this.exporting.set(kind);
    this.message.set(null);
    try {
      const image = await this.exporter.export(this.canvas().stageElement(), this.doc(), kind);
      downloadBlob(image.blob, image.filename, this.document);
      const shown = [...new Set(image.uncovered)].slice(0, 12).join(' ');
      this.message.set(
        image.uncovered.length > 0
          ? { kind: 'warn', text: `Saved ${image.filename}. These characters have no font here and use your device's font: ${shown}${new Set(image.uncovered).size > 12 ? ' and more' : ''}.` }
          : {
              kind: 'ok',
              text:
                kind === 'pdf'
                  ? `Saved ${image.filename}: one page, ${image.width} × ${image.height} pixels. The words are part of the picture, so they cannot be selected or searched.`
                  : `Saved ${image.filename} (${image.width} × ${image.height}).`,
            },
      );
    } catch {
      this.message.set({ kind: 'error', text: "The image could not be made in this browser. Try again, or try a smaller size." });
    } finally {
      this.exporting.set(null);
    }
  }
}
