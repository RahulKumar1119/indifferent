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
  template: `
    <section class="glass-card spotlight-card relative overflow-hidden p-5 md:p-7" aria-label="Creator DNA">
      <div
        class="pointer-events-none absolute -bottom-24 -left-24 h-56 w-56 rounded-full opacity-15 blur-3xl"
        style="background: radial-gradient(circle, hsl(var(--primary)), transparent 70%)"
        aria-hidden="true"
      ></div>
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-lg font-semibold flex items-center gap-2">
          <svg lucideIcon="sparkles" [size]="19" class="text-[hsl(var(--primary))]" aria-hidden="true"></svg>
          Creator DNA
        </h2>
        @if (isDefault()) {
          <span class="rounded-full bg-[hsl(var(--secondary))] px-3 py-1 text-[11.5px] font-medium text-[hsl(var(--muted-foreground))]">
            Starter preset
          </span>
        } @else {
          <span class="rounded-full bg-emerald-500/10 text-emerald-500 px-3 py-1 text-[11.5px] font-semibold">
            Customized
          </span>
        }
      </div>
      <p class="mt-1 text-[13px] text-[hsl(var(--muted-foreground))]">
        Reusable AI preferences for every video you generate.
      </p>

      <dl class="mt-4 grid grid-cols-2 gap-x-4 gap-y-3">
        @for (field of fields(); track field.key) {
          <div class="rounded-xl bg-white/[0.03] border border-[hsl(var(--border))]/60 px-3.5 py-2.5">
            <dt class="text-[11px] uppercase tracking-[0.1em] text-[hsl(var(--muted-foreground))]">{{ field.label }}</dt>
            <dd class="mt-0.5 text-[14px] font-medium truncate">{{ field.value }}</dd>
          </div>
        }
      </dl>

      <div class="mt-5 flex flex-col sm:flex-row gap-2">
        <button
          type="button"
          (click)="openDialog()"
          class="btn-interactive flex-1 inline-flex items-center justify-center gap-2 rounded-full bg-[hsl(var(--primary))] text-white py-2.5 text-[13.5px] font-semibold"
        >
          <svg lucideIcon="wand-2" [size]="15" aria-hidden="true"></svg>
          Customize Creator DNA
        </button>
        <button
          type="button"
          (click)="useDna.emit()"
          class="btn-interactive flex-1 inline-flex items-center justify-center gap-2 rounded-full border border-[hsl(var(--border))] py-2.5 text-[13.5px] font-medium hover:bg-white/5"
          title="Start a new video reusing these preferences"
        >
          Generate using my DNA
          <svg lucideIcon="arrow-right" [size]="15" aria-hidden="true"></svg>
        </button>
      </div>
    </section>

    @if (dialogOpen) {
      <div
        class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4"
        role="dialog"
        aria-modal="true"
        aria-label="Customize Creator DNA"
      >
        <div class="absolute inset-0 bg-black/60" (click)="closeDialog()" aria-hidden="true"></div>
        <div
          class="relative w-full max-w-lg max-h-[85dvh] overflow-y-auto rounded-2xl border border-[hsl(var(--border))] bg-[hsl(var(--card))] p-6 shadow-2xl"
        >
          <div class="flex items-center justify-between">
            <h2 class="text-lg font-semibold flex items-center gap-2">
              <svg lucideIcon="sparkles" [size]="18" class="text-[hsl(var(--primary))]" aria-hidden="true"></svg>
              Customize Creator DNA
            </h2>
            <button
              type="button"
              (click)="closeDialog()"
              class="w-9 h-9 rounded-lg hover:bg-white/5 inline-flex items-center justify-center"
              aria-label="Close Creator DNA editor"
            >
              <svg lucideIcon="x" [size]="18"></svg>
            </button>
          </div>
          <div class="mt-4 grid grid-cols-1 sm:grid-cols-2 gap-4">
            @for (key of dnaKeys; track key) {
              <label class="block">
                <span class="text-[13px] font-medium">{{ labels[key] }}</span>
                <select
                  [(ngModel)]="draft[key]"
                  class="mt-1.5 w-full rounded-xl border border-[hsl(var(--border))] bg-[hsl(var(--secondary))] px-3 py-2.5 text-[14px] outline-none focus:border-[hsl(var(--primary))]"
                >
                  @for (opt of options[key]; track opt) {
                    <option [value]="opt">{{ opt }}</option>
                  }
                </select>
              </label>
            }
          </div>
          <div class="mt-5 flex justify-end gap-2">
            <button
              type="button"
              (click)="resetDraft()"
              class="rounded-full px-5 py-2.5 text-[13.5px] font-medium border border-[hsl(var(--border))] hover:bg-white/5"
            >
              Reset
            </button>
            <button
              type="button"
              (click)="saveDialog()"
              class="btn-interactive rounded-full px-5 py-2.5 text-[13.5px] font-semibold bg-[hsl(var(--primary))] text-white"
            >
              Save DNA
            </button>
          </div>
        </div>
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
    return this.dnaKeys.slice(0, 8).map((key) => ({ key, label: this.labels[key], value: d[key] }));
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
