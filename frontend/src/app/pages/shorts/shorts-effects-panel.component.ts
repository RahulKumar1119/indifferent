import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import {
  ResolvedVideoEffects,
  ShortsService,
  defaultVideoEffects,
  resolveVideoEffects,
} from './shorts.service';

/** Counts the API enforces. The panel stops adding rows at these limits. */
const MAX_PUNCH_INS = 10;
const MAX_FREEZES = 5;
const MAX_IMPACT_TEXTS = 5;

interface Option<T extends string> {
  id: T;
  label: string;
}

/**
 * Video effects for one shorts job: a toggle and an intensity control per
 * phase 1 effect, plus the times and text each effect needs. Reads the job id
 * from the route, so the same panel works on the gallery and progress pages.
 * Settings change only while the job is uploaded or failed; the server
 * validates every value and its messages are shown here.
 */
@Component({
  selector: 'app-shorts-effects-panel',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <section class="glass-card p-5 text-left" aria-label="Video effects">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h2 class="font-semibold text-[15px]">Video effects</h2>
        <span class="text-[12px] text-[hsl(var(--muted-foreground))]">Saved per job · used by the next render or retry</span>
      </div>

      @if (loadError) {
        <p class="mt-3 rounded-xl border border-red-500/30 bg-red-500/[0.06] px-4 py-2.5 text-[13px] text-red-400" role="alert">
          Couldn't load the saved effects. Check your connection and reopen this panel.
        </p>
      } @else if (loaded && !editable) {
        <p class="mt-3 rounded-xl border border-amber-500/30 bg-amber-500/[0.07] px-4 py-2.5 text-[13px] text-[hsl(var(--muted-foreground))]" role="note">
          Effects can change only before a render starts, or after a render failed. This job is locked.
        </p>
      }

      @if (loaded && !loadError) {
        <fieldset class="mt-4 min-w-0 space-y-4" [disabled]="!editable">
          <!-- Animated captions -->
          <div class="rounded-xl border border-[hsl(var(--border))] p-4">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h3 class="text-[13.5px] font-semibold">Animated captions</h3>
                <p class="text-[12px] text-[hsl(var(--muted-foreground))]">Words light up as they are spoken. While on, these replace the Caption style.</p>
              </div>
              <label class="flex items-center gap-2 text-[13px] font-medium">
                <input type="checkbox" class="h-5 w-5 accent-[hsl(var(--primary))]" [(ngModel)]="draft.captions.enabled" aria-label="Turn on animated captions" />
                On
              </label>
            </div>
            <div class="mt-4 grid grid-cols-1 sm:grid-cols-2 gap-4">
              <label class="block">
                <span class="text-[12.5px] font-medium">Dim unspoken words · <span class="tabular-nums">{{ draft.captions.intensity }}</span></span>
                <input type="range" min="0" max="100" step="1" class="w-full" [(ngModel)]="draft.captions.intensity" aria-label="Animated captions dimming" />
              </label>
              <label class="block">
                <span class="text-[12.5px] font-medium">Font</span>
                <select class="mt-1 block min-h-[40px] w-full rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-2 text-[13px] text-[hsl(var(--foreground))]" [(ngModel)]="draft.captions.font" aria-label="Animated captions font">
                  @for (f of fonts; track f.id) {
                    <option [ngValue]="f.id">{{ f.label }}</option>
                  }
                </select>
              </label>
              <label class="block">
                <span class="text-[12.5px] font-medium">Size · <span class="tabular-nums">{{ draft.captions.fontSize }}</span></span>
                <input type="range" min="8" max="72" step="1" class="w-full" [(ngModel)]="draft.captions.fontSize" aria-label="Animated captions size" />
              </label>
              <label class="block">
                <span class="text-[12.5px] font-medium">Outline · <span class="tabular-nums">{{ draft.captions.outline }}</span></span>
                <input type="range" min="0" max="12" step="1" class="w-full" [(ngModel)]="draft.captions.outline" aria-label="Animated captions outline" />
              </label>
              <div class="flex items-center gap-4">
                <label class="flex items-center gap-2 text-[12.5px] font-medium">
                  Text
                  <input type="color" class="h-10 w-10 cursor-pointer rounded-lg border border-[hsl(var(--border))] bg-transparent" [(ngModel)]="draft.captions.color" aria-label="Animated captions text color" />
                </label>
                <label class="flex items-center gap-2 text-[12.5px] font-medium">
                  Spoken word
                  <input type="color" class="h-10 w-10 cursor-pointer rounded-lg border border-[hsl(var(--border))] bg-transparent" [(ngModel)]="draft.captions.highlight" aria-label="Animated captions highlight color" />
                </label>
              </div>
              <label class="block">
                <span class="text-[12.5px] font-medium">Background</span>
                <select class="mt-1 block min-h-[40px] w-full rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-2 text-[13px] text-[hsl(var(--foreground))]" [(ngModel)]="draft.captions.background" aria-label="Animated captions background">
                  @for (b of backgrounds; track b.id) {
                    <option [ngValue]="b.id">{{ b.label }}</option>
                  }
                </select>
              </label>
              <div>
                <span class="text-[12.5px] font-medium">Position</span>
                <div class="mt-1 flex flex-wrap gap-2" role="group" aria-label="Animated captions position">
                  @for (pos of positions; track pos) {
                    <button
                      type="button"
                      (click)="draft.captions.position = pos"
                      [attr.aria-pressed]="draft.captions.position === pos"
                      class="min-h-[40px] px-4 rounded-full border text-[13px] font-medium capitalize"
                      [class]="draft.captions.position === pos ? 'bg-[hsl(var(--foreground))] text-[hsl(var(--background))] border-transparent' : 'border-[hsl(var(--border))]'"
                    >{{ pos }}</button>
                  }
                </div>
              </div>
            </div>
          </div>

          <!-- Dynamic zoom -->
          <div class="rounded-xl border border-[hsl(var(--border))] p-4">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h3 class="text-[13.5px] font-semibold">Dynamic zoom</h3>
                <p class="text-[12px] text-[hsl(var(--muted-foreground))]">Pushes in at each punch-in time, then eases back out.</p>
              </div>
              <label class="flex items-center gap-2 text-[13px] font-medium">
                <input type="checkbox" class="h-5 w-5 accent-[hsl(var(--primary))]" [(ngModel)]="draft.zoom.enabled" aria-label="Turn on dynamic zoom" />
                On
              </label>
            </div>
            <label class="mt-4 block max-w-md">
              <span class="text-[12.5px] font-medium">Zoom strength · <span class="tabular-nums">{{ draft.zoom.intensity }}</span></span>
              <input type="range" min="0" max="100" step="1" class="w-full" [(ngModel)]="draft.zoom.intensity" aria-label="Zoom strength" />
            </label>
            <p class="mt-4 text-[12.5px] font-medium">Punch-ins</p>
            @for (p of draft.zoom.punchIns; track $index; let i = $index) {
              <div class="mt-2 flex flex-wrap items-end gap-2">
                <label class="block w-28">
                  <span class="text-[12px] text-[hsl(var(--muted-foreground))]">At (s)</span>
                  <input type="number" min="0" max="90" step="0.1" class="mt-1 block min-h-[40px] w-full rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-2 text-[13px] text-[hsl(var(--foreground))]" [(ngModel)]="p.at" [attr.aria-label]="'Punch-in ' + (i + 1) + ' start in seconds'" />
                </label>
                <label class="block w-28">
                  <span class="text-[12px] text-[hsl(var(--muted-foreground))]">Length (s)</span>
                  <input type="number" min="0.5" max="6" step="0.1" class="mt-1 block min-h-[40px] w-full rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-2 text-[13px] text-[hsl(var(--foreground))]" [(ngModel)]="p.duration" [attr.aria-label]="'Punch-in ' + (i + 1) + ' length in seconds'" />
                </label>
                <button type="button" (click)="removePunchIn(i)" class="min-h-[40px] px-4 rounded-full border border-[hsl(var(--border))] text-[13px] font-medium hover:bg-white/5">Remove</button>
              </div>
            }
            <button
              type="button"
              (click)="addPunchIn()"
              [disabled]="draft.zoom.punchIns.length >= maxPunchIns"
              class="mt-3 min-h-[40px] px-4 rounded-full border border-[hsl(var(--border))] text-[13px] font-medium hover:bg-white/5 disabled:opacity-50"
            >Add punch-in</button>
            <p class="mt-2 text-[11.5px] text-[hsl(var(--muted-foreground))]">Up to {{ maxPunchIns }}. Times are seconds from the start of each clip.</p>
          </div>

          <!-- Cinematic grading -->
          <div class="rounded-xl border border-[hsl(var(--border))] p-4">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h3 class="text-[13.5px] font-semibold">Color grading</h3>
                <p class="text-[12px] text-[hsl(var(--muted-foreground))]">A cinematic, warm, cool or high-contrast look over the whole clip.</p>
              </div>
              <label class="flex items-center gap-2 text-[13px] font-medium">
                <input type="checkbox" class="h-5 w-5 accent-[hsl(var(--primary))]" [(ngModel)]="draft.grade.enabled" aria-label="Turn on color grading" />
                On
              </label>
            </div>
            <div class="mt-4 grid grid-cols-1 sm:grid-cols-2 gap-4">
              <label class="block">
                <span class="text-[12.5px] font-medium">Look</span>
                <select class="mt-1 block min-h-[40px] w-full rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-2 text-[13px] text-[hsl(var(--foreground))]" [(ngModel)]="draft.grade.look" aria-label="Color grading look">
                  @for (l of looks; track l.id) {
                    <option [ngValue]="l.id">{{ l.label }}</option>
                  }
                </select>
              </label>
              <label class="block">
                <span class="text-[12.5px] font-medium">Strength · <span class="tabular-nums">{{ draft.grade.intensity }}</span></span>
                <input type="range" min="0" max="100" step="1" class="w-full" [(ngModel)]="draft.grade.intensity" aria-label="Color grading strength" />
              </label>
            </div>
          </div>

          <!-- Freeze frame and impact text -->
          <div class="rounded-xl border border-[hsl(var(--border))] p-4">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h3 class="text-[13.5px] font-semibold">Freeze frame and impact text</h3>
                <p class="text-[12px] text-[hsl(var(--muted-foreground))]">Holds the picture and adds short text at chosen times. Audio is silent during each hold, and captions move with it.</p>
              </div>
              <label class="flex items-center gap-2 text-[13px] font-medium">
                <input type="checkbox" class="h-5 w-5 accent-[hsl(var(--primary))]" [(ngModel)]="draft.freeze.enabled" aria-label="Turn on freeze frame and impact text" />
                On
              </label>
            </div>
            <label class="mt-4 block max-w-md">
              <span class="text-[12.5px] font-medium">Impact text size · <span class="tabular-nums">{{ draft.freeze.intensity }}</span></span>
              <input type="range" min="0" max="100" step="1" class="w-full" [(ngModel)]="draft.freeze.intensity" aria-label="Impact text size" />
            </label>

            <p class="mt-4 text-[12.5px] font-medium">Freezes</p>
            @for (h of draft.freeze.freezes; track $index; let i = $index) {
              <div class="mt-2 flex flex-wrap items-end gap-2">
                <label class="block w-28">
                  <span class="text-[12px] text-[hsl(var(--muted-foreground))]">At (s)</span>
                  <input type="number" min="0" max="90" step="0.1" class="mt-1 block min-h-[40px] w-full rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-2 text-[13px] text-[hsl(var(--foreground))]" [(ngModel)]="h.at" [attr.aria-label]="'Freeze ' + (i + 1) + ' time in seconds'" />
                </label>
                <label class="block w-28">
                  <span class="text-[12px] text-[hsl(var(--muted-foreground))]">Hold (s)</span>
                  <input type="number" min="0.2" max="3" step="0.1" class="mt-1 block min-h-[40px] w-full rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-2 text-[13px] text-[hsl(var(--foreground))]" [(ngModel)]="h.duration" [attr.aria-label]="'Freeze ' + (i + 1) + ' hold length in seconds'" />
                </label>
                <button type="button" (click)="removeFreeze(i)" class="min-h-[40px] px-4 rounded-full border border-[hsl(var(--border))] text-[13px] font-medium hover:bg-white/5">Remove</button>
              </div>
            }
            <button
              type="button"
              (click)="addFreeze()"
              [disabled]="draft.freeze.freezes.length >= maxFreezes"
              class="mt-3 min-h-[40px] px-4 rounded-full border border-[hsl(var(--border))] text-[13px] font-medium hover:bg-white/5 disabled:opacity-50"
            >Add freeze</button>

            <p class="mt-5 text-[12.5px] font-medium">Impact text</p>
            @for (t of draft.freeze.impactTexts; track $index; let i = $index) {
              <div class="mt-2 flex flex-wrap items-end gap-2">
                <label class="block w-24">
                  <span class="text-[12px] text-[hsl(var(--muted-foreground))]">At (s)</span>
                  <input type="number" min="0" max="90" step="0.1" class="mt-1 block min-h-[40px] w-full rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-2 text-[13px] text-[hsl(var(--foreground))]" [(ngModel)]="t.at" [attr.aria-label]="'Impact text ' + (i + 1) + ' time in seconds'" />
                </label>
                <label class="block w-24">
                  <span class="text-[12px] text-[hsl(var(--muted-foreground))]">Length (s)</span>
                  <input type="number" min="0.3" max="4" step="0.1" class="mt-1 block min-h-[40px] w-full rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-2 text-[13px] text-[hsl(var(--foreground))]" [(ngModel)]="t.duration" [attr.aria-label]="'Impact text ' + (i + 1) + ' length in seconds'" />
                </label>
                <label class="block min-w-[10rem] flex-1">
                  <span class="text-[12px] text-[hsl(var(--muted-foreground))]">Text, 16 characters</span>
                  <input type="text" maxlength="16" class="mt-1 block min-h-[40px] w-full rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-2 text-[13px] text-[hsl(var(--foreground))]" [(ngModel)]="t.text" [attr.aria-label]="'Impact text ' + (i + 1) + ' words'" />
                </label>
                <button type="button" (click)="removeImpactText(i)" class="min-h-[40px] px-4 rounded-full border border-[hsl(var(--border))] text-[13px] font-medium hover:bg-white/5">Remove</button>
              </div>
            }
            <button
              type="button"
              (click)="addImpactText()"
              [disabled]="draft.freeze.impactTexts.length >= maxImpactTexts"
              class="mt-3 min-h-[40px] px-4 rounded-full border border-[hsl(var(--border))] text-[13px] font-medium hover:bg-white/5 disabled:opacity-50"
            >Add impact text</button>
          </div>

          <div class="flex flex-wrap items-center gap-3">
            <button type="button" (click)="resetDraft()" class="min-h-[40px] px-4 rounded-full border border-[hsl(var(--border))] text-[13px] font-medium hover:bg-white/5">Reset to defaults</button>
            <span class="flex-1"></span>
            <button
              type="button"
              (click)="save()"
              [disabled]="saving"
              class="btn-interactive min-h-[40px] px-5 rounded-full bg-[hsl(var(--primary))] text-white text-[13.5px] font-semibold disabled:opacity-50"
            >{{ saving ? 'Saving…' : 'Save effects' }}</button>
          </div>
        </fieldset>

        @if (error) {
          <p class="mt-3 text-[13px] text-red-400" role="alert">{{ error }}</p>
        } @else if (saved) {
          <p class="mt-3 text-[13px] text-[hsl(var(--muted-foreground))]" role="status">Effects saved. They apply to the next render or retry.</p>
        }
      }
    </section>
  `,
})
export class ShortsEffectsPanelComponent implements OnInit {
  private jobId = '';
  /** Job status from the last load or save. Only uploaded and failed jobs can change. */
  private status = '';
  loaded = false;
  loadError = false;
  saving = false;
  saved = false;
  error = '';
  draft: ResolvedVideoEffects = defaultVideoEffects();

  readonly maxPunchIns = MAX_PUNCH_INS;
  readonly maxFreezes = MAX_FREEZES;
  readonly maxImpactTexts = MAX_IMPACT_TEXTS;
  readonly fonts: Option<ResolvedVideoEffects['captions']['font']>[] = [
    { id: 'sans', label: 'Sans' },
    { id: 'serif', label: 'Serif' },
    { id: 'condensed', label: 'Condensed' },
  ];
  readonly backgrounds: Option<ResolvedVideoEffects['captions']['background']>[] = [
    { id: 'none', label: 'None' },
    { id: 'dark', label: 'Dark box' },
    { id: 'light', label: 'Light box' },
  ];
  readonly looks: Option<ResolvedVideoEffects['grade']['look']>[] = [
    { id: 'cinematic', label: 'Cinematic' },
    { id: 'warm', label: 'Warm' },
    { id: 'cool', label: 'Cool' },
    { id: 'high-contrast', label: 'High contrast' },
  ];
  readonly positions: ResolvedVideoEffects['captions']['position'][] = ['top', 'middle', 'bottom'];

  constructor(
    private readonly route: ActivatedRoute,
    private readonly shorts: ShortsService,
  ) {}

  ngOnInit(): void {
    this.jobId = this.route.snapshot.paramMap.get('id') || '';
    if (!this.jobId) {
      this.loadError = true;
      this.loaded = true;
      return;
    }
    this.shorts.getStatus(this.jobId).subscribe({
      next: (job) => {
        this.draft = resolveVideoEffects(job.effects);
        this.status = job.status ?? '';
        this.loaded = true;
      },
      error: () => {
        this.loadError = true;
        this.loaded = true;
      },
    });
  }

  /** Effects apply to renders, so they change only before a render starts or after one failed. */
  get editable(): boolean {
    return this.status === 'uploaded' || this.status === 'failed';
  }

  addPunchIn(): void {
    if (this.draft.zoom.punchIns.length < MAX_PUNCH_INS) {
      this.draft.zoom.punchIns.push({ at: 0, duration: 1 });
    }
  }

  removePunchIn(index: number): void {
    this.draft.zoom.punchIns.splice(index, 1);
  }

  addFreeze(): void {
    if (this.draft.freeze.freezes.length < MAX_FREEZES) {
      this.draft.freeze.freezes.push({ at: 0, duration: 0.5 });
    }
  }

  removeFreeze(index: number): void {
    this.draft.freeze.freezes.splice(index, 1);
  }

  addImpactText(): void {
    if (this.draft.freeze.impactTexts.length < MAX_IMPACT_TEXTS) {
      this.draft.freeze.impactTexts.push({ at: 0, duration: 1, text: '' });
    }
  }

  removeImpactText(index: number): void {
    this.draft.freeze.impactTexts.splice(index, 1);
  }

  /** Puts every effect back to its default. Nothing is saved until Save effects. */
  resetDraft(): void {
    this.draft = defaultVideoEffects();
    this.saved = false;
    this.error = '';
  }

  save(): void {
    if (this.saving || !this.editable || !this.jobId) return;
    this.saving = true;
    this.saved = false;
    this.error = '';
    this.shorts.updateEffects(this.jobId, this.draft).subscribe({
      next: (job) => {
        this.saving = false;
        this.saved = true;
        this.draft = resolveVideoEffects(job.effects);
      },
      error: (err) => {
        this.saving = false;
        this.error = err?.error?.message || 'Could not save video effects.';
      },
    });
  }
}
