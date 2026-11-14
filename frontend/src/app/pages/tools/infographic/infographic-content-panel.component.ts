import { CdkDrag, CdkDragDrop, CdkDragHandle, CdkDropList } from '@angular/cdk/drag-drop';
import { DOCUMENT } from '@angular/common';
import { ChangeDetectionStrategy, Component, Injector, afterNextRender, computed, inject, input, output } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { addSection, duplicateSection, moveSection, removeSection, reorder, setSectionField } from './doc-ops';
import { DocChange } from './infographic-design-panel.component';
import { InfographicDoc, LIMITS } from './infographic.model';
import { Template } from './templates';
import { normalizeText } from './text-utils';

type TopField = 'title' | 'subtitle' | 'footer';

/** The words: title, subtitle, the sections (add, copy, remove, move) and the footer. */
@Component({
  selector: 'app-infographic-content-panel',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CdkDropList, CdkDrag, CdkDragHandle, LucideDynamicIcon],
  styles: [`
    :host { display: block; }
    .stack { display: flex; flex-direction: column; gap: var(--space-md); }
    .sections { display: flex; flex-direction: column; gap: var(--space-sm); }
    .sec { padding: var(--space-md); display: flex; flex-direction: column; gap: var(--space-sm); background: var(--color-paper); }
    .sec.cdk-drag-placeholder { opacity: 0.35; }
    .sec-head { display: flex; align-items: center; gap: var(--space-xs); }
    .sec-head h3 { margin: 0; flex: 1; font-size: var(--type-md); font-weight: 600; }
    .grip { display: inline-flex; align-items: center; justify-content: center; width: 32px; min-height: 44px; color: var(--color-ink-2); cursor: grab; touch-action: none; }
    .acts { display: flex; flex-wrap: wrap; gap: 2px; }
    .acts .ui-btn { min-width: 44px; padding-inline: var(--space-xs); }
    .hint { margin: 0; font-size: var(--type-xs); color: var(--color-ink-2); }
    .add { align-self: flex-start; }
    textarea.ui-field { min-height: 5.5rem; resize: vertical; line-height: 1.5; }
  `],
  template: `
    <div class="stack">
      <div class="ui-f">
        <label class="ui-flabel" for="ig-title">Title</label>
        <input id="ig-title" class="ui-field" dir="auto" type="text" [attr.maxlength]="limits.title" [value]="doc().title" (input)="top('title', $event)" />
      </div>
      <div class="ui-f">
        <label class="ui-flabel" for="ig-subtitle">Subtitle <span class="ui-hint">(optional)</span></label>
        <textarea id="ig-subtitle" class="ui-field" dir="auto" rows="2" [attr.maxlength]="limits.subtitle" [value]="doc().subtitle" (input)="top('subtitle', $event)"></textarea>
      </div>

      <div class="sections" cdkDropList (cdkDropListDropped)="drop($event)" role="list" aria-label="Sections">
        @for (s of doc().sections; track s.id; let i = $index; let first = $first; let last = $last) {
          <div class="sec ui-card" cdkDrag cdkDragLockAxis="y" role="listitem" [attr.data-section]="s.id">
            <div class="sec-head">
              <span class="grip" cdkDragHandle aria-hidden="true" title="Drag to reorder"><svg lucideIcon="grip-vertical" [size]="18"></svg></span>
              <h3>Section {{ i + 1 }}</h3>
              <div class="acts">
                <button type="button" class="ui-btn ui-btn--quiet" [id]="'ig-up-' + s.id" [disabled]="first" (click)="move(s.id, -1)" [attr.aria-label]="'Move section ' + (i + 1) + ' up'">
                  <svg lucideIcon="chevron-up" [size]="18" aria-hidden="true"></svg>
                </button>
                <button type="button" class="ui-btn ui-btn--quiet" [id]="'ig-down-' + s.id" [disabled]="last" (click)="move(s.id, 1)" [attr.aria-label]="'Move section ' + (i + 1) + ' down'">
                  <svg lucideIcon="chevron-down" [size]="18" aria-hidden="true"></svg>
                </button>
                <button type="button" class="ui-btn ui-btn--quiet" [disabled]="!canAdd()" (click)="duplicate(s.id)" [attr.aria-label]="'Duplicate section ' + (i + 1)">
                  <svg lucideIcon="copy" [size]="18" aria-hidden="true"></svg>
                </button>
                <button type="button" class="ui-btn ui-btn--quiet" [disabled]="!canRemove()" (click)="remove(s.id)" [attr.aria-label]="'Remove section ' + (i + 1)">
                  <svg lucideIcon="trash-2" [size]="18" aria-hidden="true"></svg>
                </button>
              </div>
            </div>
            @if (template().labelName) {
              <div class="ui-f">
                <label class="ui-flabel" [for]="'ig-label-' + s.id">{{ template().labelName }}</label>
                <input [id]="'ig-label-' + s.id" class="ui-field" dir="auto" type="text" [attr.maxlength]="limits.label" [value]="s.label" (input)="sec(s.id, 'label', $event)" />
              </div>
            }
            <div class="ui-f">
              <label class="ui-flabel" [for]="'ig-heading-' + s.id">Heading</label>
              <input [id]="'ig-heading-' + s.id" class="ui-field" dir="auto" type="text" [attr.maxlength]="limits.heading" [value]="s.heading" (input)="sec(s.id, 'heading', $event)" />
            </div>
            <div class="ui-f">
              <label class="ui-flabel" [for]="'ig-body-' + s.id">{{ template().bullets ? 'Bullets' : 'Text' }}</label>
              <textarea [id]="'ig-body-' + s.id" class="ui-field" dir="auto" rows="3" [attr.maxlength]="limits.body" [value]="s.body" (input)="sec(s.id, 'body', $event)"></textarea>
              @if (template().bullets) {
                <p class="hint">One bullet per line.</p>
              }
            </div>
          </div>
        }
      </div>

      <button type="button" id="ig-add" class="ui-btn ui-btn--secondary add" [disabled]="!canAdd()" (click)="add()">
        <svg lucideIcon="plus" [size]="16" aria-hidden="true"></svg>
        Add section
      </button>
      @if (!canAdd()) {
        <p class="hint">This template holds up to {{ max() }} sections.</p>
      }

      <div class="ui-f">
        <label class="ui-flabel" for="ig-footer">Footer <span class="ui-hint">(optional)</span></label>
        <input id="ig-footer" class="ui-field" dir="auto" type="text" [attr.maxlength]="limits.footer" [value]="doc().footer" (input)="top('footer', $event)" />
      </div>
    </div>
  `,
})
export class InfographicContentPanelComponent {
  private readonly document = inject(DOCUMENT);
  private readonly injector = inject(Injector);

  readonly doc = input.required<InfographicDoc>();
  readonly template = input.required<Template>();
  readonly docChange = output<DocChange>();
  /** A sentence for the page's live region, such as "Section 2 moved up". */
  readonly announce = output<string>();

  protected readonly limits = LIMITS;
  protected readonly max = computed(() => Math.min(this.template().maxSections, LIMITS.sections));
  protected readonly canAdd = computed(() => this.doc().sections.length < this.max());
  protected readonly canRemove = computed(() => this.doc().sections.length > this.template().minSections);

  /** Cleans what was typed; when cleaning changed it (a ligature became letters), the box shows the clean text. */
  private clean(event: Event, multiline: boolean): string {
    const el = event.target as HTMLInputElement | HTMLTextAreaElement;
    const cleaned = normalizeText(el.value, multiline);
    if (cleaned !== el.value) el.value = cleaned;
    return cleaned;
  }

  protected top(field: TopField, event: Event): void {
    const value = this.clean(event, field === 'subtitle');
    if (this.doc()[field] === value) return;
    this.docChange.emit({ doc: { ...this.doc(), [field]: value }, key: field });
  }

  protected sec(id: string, field: 'label' | 'heading' | 'body', event: Event): void {
    const next = setSectionField(this.doc(), id, field, this.clean(event, field === 'body'));
    if (next !== this.doc()) this.docChange.emit({ doc: next, key: `${id}:${field}` });
  }

  private focusLater(id: string): void {
    afterNextRender(() => this.document.getElementById(id)?.focus(), { injector: this.injector });
  }

  protected move(id: string, delta: -1 | 1): void {
    const next = moveSection(this.doc(), id, delta);
    if (next === this.doc()) return;
    this.docChange.emit({ doc: next });
    const at = next.sections.findIndex((s) => s.id === id);
    this.announce.emit(`Section moved to position ${at + 1} of ${next.sections.length}.`);
    // The pressed button moves with its section; keep focus there unless the section is now at the end.
    this.focusLater(delta === -1 ? (at === 0 ? `ig-down-${id}` : `ig-up-${id}`) : at === next.sections.length - 1 ? `ig-up-${id}` : `ig-down-${id}`);
  }

  protected drop(e: CdkDragDrop<unknown>): void {
    const next = reorder(this.doc(), e.previousIndex, e.currentIndex);
    if (next === this.doc()) return;
    this.docChange.emit({ doc: next });
    this.announce.emit(`Section moved to position ${e.currentIndex + 1} of ${next.sections.length}.`);
  }

  protected add(): void {
    const next = addSection(this.doc(), this.template());
    if (next === this.doc()) return;
    this.docChange.emit({ doc: next });
    this.announce.emit(`Section ${next.sections.length} added.`);
    this.focusLater(`ig-heading-${next.sections[next.sections.length - 1].id}`);
  }

  protected duplicate(id: string): void {
    const at = this.doc().sections.findIndex((s) => s.id === id);
    const next = duplicateSection(this.doc(), this.template(), id);
    if (next === this.doc()) return;
    this.docChange.emit({ doc: next });
    this.announce.emit(`Section ${at + 1} duplicated. The copy is section ${at + 2}.`);
    this.focusLater(`ig-heading-${next.sections[at + 1].id}`);
  }

  protected remove(id: string): void {
    const at = this.doc().sections.findIndex((s) => s.id === id);
    const next = removeSection(this.doc(), this.template(), id);
    if (next === this.doc()) return;
    this.docChange.emit({ doc: next });
    this.announce.emit(`Section ${at + 1} removed.`);
    const near = next.sections[Math.min(at, next.sections.length - 1)];
    this.focusLater(near ? `ig-heading-${near.id}` : 'ig-add');
  }
}
