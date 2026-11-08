import { Component, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { LucideDynamicIcon } from '@lucide/angular';
import {
  CREATOR_DNA_LABELS,
  CREATOR_DNA_OPTIONS,
  CreatorDNA,
  DEFAULT_CREATOR_DNA,
} from '../models/creator-dna.model';

/** Creator DNA card: read-only preference grid + customize dialog. */
@Component({
  selector: 'app-creator-dna',
  standalone: true,
  imports: [FormsModule, LucideDynamicIcon],
  styles: [`
    :host { display: block; }
    .head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-xs); }
    dl { margin: var(--space-md) 0 0; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-xs); }
    dl div { padding: var(--space-xs) var(--space-sm); border-radius: var(--radius-control); background: var(--color-paper-3); min-width: 0; }
    dt { font-size: var(--type-xs); color: var(--color-muted); }
    dd { margin: var(--space-3xs) 0 0; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .on { color: var(--color-positive); } .off { color: var(--color-ink-2); }
    .actions { margin-top: var(--space-md); display: flex; flex-wrap: wrap; gap: var(--space-xs); }
    .fields { margin-top: var(--space-md); display: grid; gap: var(--space-md); }
    @media (min-width: 40rem) { .fields { grid-template-columns: 1fr 1fr; } }
    .field { display: flex; flex-direction: column; gap: var(--space-2xs); }
    .foot { margin-top: var(--space-lg); display: flex; justify-content: flex-end; gap: var(--space-xs); }
    .dialog { position: fixed; inset: 0; z-index: var(--z-modal); display: flex; align-items: flex-end; justify-content: center; padding: var(--space-md); }
    @media (min-width: 40rem) { .dialog { align-items: center; } }
    .scrim { position: absolute; inset: 0; background: color-mix(in oklab, var(--color-ink) 55%, transparent); }
    .panel { position: relative; width: 100%; max-width: 34rem; max-height: 85dvh; overflow-y: auto; }
    .panel-head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-sm); }
    .icon-btn { width: 44px; height: 44px; display: inline-flex; align-items: center; justify-content: center; border: 0; border-radius: var(--radius-control); background: transparent; color: var(--color-ink-2); cursor: pointer; }
    .icon-btn:hover { background: var(--color-paper-3); }
  `],
  template: `
    <section class="ui-card" aria-labelledby="dna-heading">
      <div class="head">
        <h2 id="dna-heading" class="ui-card-title">Creator DNA</h2>
        <span class="ui-pill" [class.on]="!isDefault()" [class.off]="isDefault()">{{ isDefault() ? 'Starter preset' : 'Customised' }}</span>
      </div>
      <p class="ui-card-sub">Your usual style, saved in this browser for reference. The video generators do not read it yet.</p>
      <dl>
        @for (field of fields(); track field.key) {
          <div><dt>{{ field.label }}</dt><dd>{{ field.value }}</dd></div>
        }
      </dl>
      <div class="actions">
        <button type="button" class="ui-btn ui-btn--secondary" (click)="openDialog()">
          <svg lucideIcon="pencil" [size]="16" aria-hidden="true"></svg>
          Edit Creator DNA
        </button>
        <button type="button" class="ui-btn ui-btn--quiet" (click)="useDna.emit()">
          Start a new quiz video
          <svg lucideIcon="arrow-right" [size]="16" aria-hidden="true"></svg>
        </button>
      </div>
    </section>
    @if (dialogOpen) {
      <div class="dialog hm" role="dialog" aria-modal="true" aria-labelledby="dna-dialog-title" (keydown.escape)="closeDialog()">
        <div class="scrim" (click)="closeDialog()" aria-hidden="true"></div>
        <form class="panel ui-card" (ngSubmit)="saveDialog()">
          <div class="panel-head">
            <h2 id="dna-dialog-title" class="ui-card-title">Edit Creator DNA</h2>
            <button type="button" class="icon-btn" (click)="closeDialog()" aria-label="Close">
              <svg lucideIcon="x" [size]="18" aria-hidden="true"></svg>
            </button>
          </div>
          <div class="fields">
            @for (key of dnaKeys; track key) {
              <div class="field">
                <label class="ui-label" [for]="'dna-' + key">{{ labels[key] }}</label>
                <select class="ui-field" [id]="'dna-' + key" [name]="key" [(ngModel)]="draft[key]">
                  @for (opt of options[key]; track opt) {
                    <option [value]="opt">{{ opt }}</option>
                  }
                </select>
              </div>
            }
          </div>
          <div class="foot">
            <button type="button" class="ui-btn ui-btn--secondary" (click)="resetDraft()">Reset to starter</button>
            <button type="submit" class="ui-btn ui-btn--primary">Save changes</button>
          </div>
        </form>
      </div>
    }
  `,
})
export class CreatorDnaComponent {
  readonly dna = input.required<CreatorDNA>();
  readonly isDefault = input(false);
  readonly dnaChange = output<CreatorDNA>();
  readonly useDna = output<void>();

  dialogOpen = false;
  draft: CreatorDNA = { ...DEFAULT_CREATOR_DNA };

  readonly options = CREATOR_DNA_OPTIONS;
  readonly labels = CREATOR_DNA_LABELS;
  readonly dnaKeys = Object.keys(CREATOR_DNA_OPTIONS) as (keyof CreatorDNA)[];

  fields(): { key: keyof CreatorDNA; label: string; value: string }[] {
    const d = this.dna();
    return this.dnaKeys.map((key) => ({ key, label: this.labels[key], value: d[key] }));
  }

  openDialog(): void {
    this.draft = { ...this.dna() };
    this.dialogOpen = true;
  }

  closeDialog(): void {
    this.dialogOpen = false;
  }

  resetDraft(): void {
    this.draft = {
      niche: 'Technology',
      audience: '18–34',
      language: 'English',
      tone: 'Educational',
      visualStyle: 'Cinematic',
      voice: 'Confident',
      captionStyle: 'Bold',
      format: '9:16',
      music: 'Upbeat',
    };
  }

  saveDialog(): void {
    this.dialogOpen = false;
    this.dnaChange.emit({ ...this.draft });
  }
}
