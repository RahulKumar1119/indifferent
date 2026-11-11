import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ApiService } from '../../core';
import { CreateProjectRequest, CreateProjectResponse, TEMPLATE_OPTIONS, DESIGN_ACCENTS, AccentOption, Template, Voice } from '../../shared';
import {
  DEFAULT_SHORTS_LANGUAGES,
  DEFAULT_SHORTS_LIMITS,
  LanguageOption,
  ShortsLimits,
  ShortsService,
  minSourceSeconds,
  tooShortMessage,
} from '../shorts/shorts.service';

import { probeMediaDuration } from '../../shared/media-probe';
import { FORMAT_GUIDE_PATH, SAMPLE_FILE_PATH } from '../progress/failure-message';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { StepIndicatorComponent } from '../../shared/components/step-indicator/step-indicator.component';
import { QUIZ_STEPS } from './quiz-steps';


@Component({
  selector: 'app-create-project',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, RouterLink, LucideDynamicIcon, NavbarComponent, StepIndicatorComponent],
  styles: [`
    :host { display: block; }
    .narrow { width: min(48rem, 100%); }
    .back { display: inline-flex; align-items: center; min-height: 44px; font-size: var(--type-sm); }
    .need { padding: var(--space-md) var(--space-lg); border-radius: var(--radius-surface); border: 1px solid color-mix(in oklab, var(--color-accent) 40%, transparent); background: color-mix(in oklab, var(--color-accent) 8%, var(--app-card)); }
    .need h2 { margin: 0; font-size: var(--type-md); font-weight: 600; }
    .need p { margin: var(--space-2xs) 0 0; font-size: var(--type-sm); color: var(--color-ink-2); line-height: 1.5; }
    .need-links { display: flex; flex-wrap: wrap; gap: 0 var(--space-lg); }
    .need-links a, .need-alt a { display: inline-flex; align-items: center; min-height: 44px; font-weight: 600; }
    .need-alt { border-top: 1px solid var(--color-rule); padding-top: var(--space-2xs); }
    form { display: flex; flex-direction: column; gap: var(--space-xl); }
    fieldset { margin: 0; padding: 0; border: 0; min-width: 0; }
    legend, .lbl { padding: 0; font-size: var(--type-sm); font-weight: 600; color: var(--color-ink); }
    .help { margin: var(--space-3xs) 0 var(--space-sm); font-size: var(--type-xs); color: var(--color-muted); }
    .count { display: flex; justify-content: space-between; gap: var(--space-sm); margin-top: var(--space-2xs); font-size: var(--type-xs); color: var(--color-muted); }
    .bad { color: var(--color-negative); font-weight: 500; }
    .grid { display: grid; gap: var(--space-sm); grid-template-columns: repeat(2, minmax(0, 1fr)); }
    @media (min-width: 40rem) { .grid.three { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
    .sr { position: absolute; width: 1px; height: 1px; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
    .opt { position: relative; display: block; cursor: pointer; min-width: 0; }
    .tile { display: flex; flex-direction: column; justify-content: center; height: 100%; min-height: 44px; border: 1px solid var(--color-muted); border-radius: var(--radius-control); background: var(--app-card); color: var(--color-ink); overflow: hidden; transition: border-color var(--dur-short) var(--ease-out), background-color var(--dur-short) var(--ease-out); }
    .opt:hover .tile { border-color: var(--color-ink-2); }
    .opt input:checked + .tile { border-color: var(--color-accent); box-shadow: inset 0 0 0 1px var(--color-accent); background: color-mix(in oklab, var(--color-accent) 8%, var(--app-card)); }
    .opt:has(input:focus-visible) { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: var(--radius-control); }
    .tpl .tile { justify-content: flex-start; }
    .tpl img { display: block; width: 100%; height: auto; border-bottom: 1px solid var(--color-rule); }
    .tname { padding: var(--space-xs) var(--space-sm); font-size: var(--type-sm); font-weight: 600; }
    .tname { display: flex; align-items: center; justify-content: space-between; gap: var(--space-2xs); }
    .chosen { display: inline-flex; align-items: center; gap: var(--space-3xs); padding: 2px var(--space-xs); border-radius: var(--radius-pill); background: var(--color-accent); color: var(--color-accent-ink); font-size: var(--type-xs); font-weight: 600; }
    .small .tile { flex-direction: row; align-items: center; gap: var(--space-xs); padding: var(--space-2xs) var(--space-sm); font-size: var(--type-sm); font-weight: 500; }
    .small .tile.center { justify-content: center; text-align: center; flex-direction: column; gap: 0; }
    .small .sub { font-size: var(--type-xs); color: var(--color-muted); font-weight: 400; }
    .dot { width: 1rem; height: 1rem; flex: none; border-radius: 50%; border: 1px solid var(--color-rule); }
    .ic { flex: none; color: var(--color-accent-text); }
    .more > summary { min-height: 64px; }
    .more-sum { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
    .more-sum .state { font-size: var(--type-xs); font-weight: 400; color: var(--color-ink-2); }
    .more-body { display: flex; flex-direction: column; gap: var(--space-lg); color: var(--color-ink); }
    .group { display: flex; flex-direction: column; gap: var(--space-md); padding-top: var(--space-md); border-top: 1px solid var(--color-rule); }
    .group h3 { margin: 0; font-size: var(--type-md); font-weight: 600; }
    .group h3 + p { margin: calc(-1 * var(--space-sm)) 0 0; font-size: var(--type-xs); color: var(--color-muted); }
    .check { display: flex; align-items: center; gap: var(--space-sm); min-height: 44px; cursor: pointer; font-size: var(--type-sm); font-weight: 500; }
    .check input { width: 1.25rem; height: 1.25rem; accent-color: var(--color-accent); flex: none; }
    .drop { display: flex; flex-direction: column; align-items: center; gap: var(--space-3xs); padding: var(--space-lg) var(--space-md); border: 1px dashed var(--color-muted); border-radius: var(--radius-control); text-align: center; cursor: pointer; }
    .drop:hover { border-color: var(--color-ink-2); }
    .drop:focus-within { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .file { display: flex; align-items: center; justify-content: space-between; gap: var(--space-sm); font-size: var(--type-sm); }
    .file span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 500; }
    .actions { display: flex; justify-content: flex-end; }
    @media (max-width: 39.99rem) { .actions .ui-btn { width: 100%; } }
    @media (prefers-reduced-motion: reduce) { .tile { transition: none; } }
  `],
  template: `
    <div class="ui-app">
      <app-navbar />
      <main id="main" class="ui-app-main narrow hm">
        <div>
          <a routerLink="/projects" class="ui-link back">All projects</a>
          <h1 class="ui-h1">New quiz video</h1>
          <p class="ui-lead">Name it, pick a design and a voice. Then you add your questions.</p>
        </div>
        <app-step-indicator [steps]="steps" [current]="0" label="Quiz video steps" />

        <!-- Say what the next step needs before the form, so nobody gets to the upload without a file. -->
        <section class="need" aria-labelledby="need-title" data-testid="file-needed">
          <h2 id="need-title">You'll need a TXT file of questions</h2>
          <p>Next you upload a plain text file. Number each question, list lettered options (A, B, C, D) and add an Answer line.</p>
          <p class="need-links">
            <a [href]="sampleFilePath" download="quiz-sample.txt" class="ui-link">Download a sample file</a>
            <a [routerLink]="formatGuidePath" class="ui-link">Read the format guide</a>
          </p>
          <p class="need-alt">Have a story or a script instead of questions? <a routerLink="/story" class="ui-link">Use Story video</a></p>
        </section>

        <form [formGroup]="projectForm" class="ui-card" (ngSubmit)="createProject()" novalidate>
          <div>
            <label for="project-name" class="lbl">Project name</label>
            <input
              id="project-name"
              type="text"
              formControlName="name"
              placeholder="My quiz video"
              maxlength="100"
              class="ui-field"
              style="margin-top: var(--space-2xs)"
              aria-describedby="name-count"
              [attr.aria-invalid]="nameMissing ? 'true' : null"
            />
            <div class="count">
              @if (nameMissing) {
                <span class="bad" role="alert">Project name is required</span>
              } @else {
                <span></span>
              }
              <span id="name-count">{{ projectForm.get('name')?.value?.length || 0 }}/100</span>
            </div>
          </div>

          <fieldset aria-describedby="design-help">
            <legend>Design</legend>
            <p id="design-help" class="help">Each image is a real slide from the renderer.</p>
            <div class="grid three">
              @for (tpl of templates; track tpl.value) {
                <label class="opt tpl">
                  <input type="radio" name="template" formControlName="template" [value]="tpl.value" [attr.value]="tpl.value" class="sr" />
                  <span class="tile">
                    <img [src]="'/templates/' + tpl.value + '.webp'" [alt]="tpl.label + ' design preview: ' + tpl.look" width="480" height="270" loading="lazy" />
                    <span class="tname">{{ tpl.label }}@if (projectForm.get('template')?.value === tpl.value) {<span class="chosen" aria-hidden="true"><svg lucideIcon="check" [size]="12"></svg>Selected</span>}</span>
                  </span>
                </label>
              }
            </div>
          </fieldset>

          <fieldset>
            <legend>Narration voice</legend>
            <div class="grid three small" style="margin-top: var(--space-sm)">
              @for (v of voices; track v) {
                <label class="opt">
                  <input type="radio" name="voice" formControlName="voice" [value]="v" [attr.value]="v" class="sr" />
                  <span class="tile"><svg lucideIcon="mic" [size]="16" class="ic" aria-hidden="true"></svg>{{ v }}</span>
                </label>
              }
            </div>
          </fieldset>

          <!-- Optional settings stay in the DOM while closed, so nothing chosen here is lost. -->
          <details class="ui-disclosure more" [open]="moreOpen" (toggle)="onMoreToggle($event)" data-testid="more-options">
            <summary>
              <span class="more-sum">
                <span>More options</span>
                <span class="state" data-testid="more-state">{{ moreState }}</span>
              </span>
            </summary>
            <div class="more-body">
              <p class="help" style="margin: 0">All optional. The defaults work for most quizzes.</p>

              <div class="group" formGroupName="designOptions">
                <h3>Design options</h3>
                <p>Applied to every question, answer, countdown and outro slide.</p>
                <fieldset>
                  <legend>Text size</legend>
                  <div class="grid three small" style="margin-top: var(--space-xs)">
                    @for (size of textSizes; track size.value) {
                      <label class="opt">
                        <input type="radio" name="textSize" formControlName="textSize" [value]="size.value" [attr.value]="size.value" class="sr" />
                        <span class="tile center"><span>{{ size.label }}</span><span class="sub">{{ size.hint }}</span></span>
                      </label>
                    }
                  </div>
                </fieldset>
                <fieldset>
                  <legend>Accent colour</legend>
                  <div class="grid small" style="margin-top: var(--space-xs)">
                    @for (accent of accentOptions; track accent.value) {
                      <label class="opt">
                        <input type="radio" name="accent" formControlName="accent" [value]="accent.value" [attr.value]="accent.value" class="sr" />
                        <span class="tile"><span class="dot" [style.background-color]="accent.value" aria-hidden="true"></span>{{ accent.label }}</span>
                      </label>
                    }
                  </div>
                </fieldset>
                <fieldset>
                  <legend>Answer labels</legend>
                  <div class="grid small" style="margin-top: var(--space-xs)">
                    @for (style of labelStyles; track style.value) {
                      <label class="opt">
                        <input type="radio" name="answerLabels" formControlName="answerLabels" [value]="style.value" [attr.value]="style.value" class="sr" />
                        <span class="tile center">{{ style.label }}</span>
                      </label>
                    }
                  </div>
                </fieldset>
                <label class="check"><input type="checkbox" formControlName="showQuestionNumber" />Show question number</label>
              </div>

              <div class="group">
                <h3>Watermark</h3>
                <p>Text shown on every slide of the video.</p>
                <div class="ui-f">
                  <label for="wm-text" class="ui-flabel">Watermark text</label>
                  <input id="wm-text" type="text" formControlName="watermarkText" placeholder="Your name" maxlength="100" class="ui-field" />
                </div>
                <div class="ui-f">
                  <label for="wm-opacity" class="ui-flabel">Opacity <span class="ui-val">{{ opacityPercent }}%</span></label>
                  <input id="wm-opacity" type="range" min="0.1" max="1" step="0.05" formControlName="watermarkOpacity" class="ui-range" />
                </div>
              </div>

              <div class="group">
                <h3>Channel branding</h3>
                <p>Your handle, burned into renders with your logo. Add the logo from the project page.</p>
                <div class="ui-f">
                  <label for="channel" class="ui-flabel">Channel handle</label>
                  <input id="channel" type="text" formControlName="channelName" placeholder="@yourchannel" maxlength="60" class="ui-field" />
                </div>
              </div>

              <div class="group">
                <h3>AI Shorts</h3>
                <p>Cut vertical clips from a video or audio file in the same project.</p>
                <label class="check"><input type="checkbox" formControlName="includeShorts" data-testid="include-shorts" />Also create AI Shorts</label>
                @if (projectForm.get('includeShorts')?.value) {
                  @if (!shortsFile) {
                    <label class="drop">
                      <input type="file" accept=".mp4,.mov,.mp3,.wav" class="sr" (change)="onShortsFileSelected($event)" />
                      <span style="font-weight: 600">Choose video or audio</span>
                      <span class="ui-hint">MP4, MOV, MP3 or WAV, {{ shortsMinSeconds }} seconds to {{ shortsLimits.maxDurationMinutes }} minutes</span>
                    </label>
                  } @else {
                    <div class="file">
                      <span>{{ shortsFile.name }}</span>
                      <button type="button" class="ui-btn ui-btn--quiet" (click)="removeShortsFile()">Remove</button>
                    </div>
                  }
                  @if (shortsProbing) {
                    <p class="ui-hint" role="status" style="margin: 0">Reading duration…</p>
                  }
                  @if (shortsError) {
                    <p class="ui-alert ui-alert--bad" role="alert" style="margin: 0">{{ shortsError }}</p>
                  }
                  <div class="ui-f">
                    <label for="shorts-lang" class="ui-flabel">Transcript language</label>
                    <select id="shorts-lang" class="ui-field" [value]="shortsLanguage" (change)="onShortsLanguageChange($event)">
                      @for (lang of shortsLanguages; track lang.code) {
                        <option [value]="lang.code">{{ lang.label }}</option>
                      }
                    </select>
                  </div>
                }
              </div>
            </div>
          </details>

          @if (errorMessage) {
            <p class="ui-alert ui-alert--bad" role="alert" style="margin: 0">{{ errorMessage }}</p>
          }
          <div class="actions">
            <button type="submit" class="ui-btn ui-btn--primary" [disabled]="projectForm.invalid || isSubmitting" [attr.aria-busy]="isSubmitting">
              @if (isSubmitting) {
                <svg lucideIcon="loader-2" [size]="18" class="ui-spin" aria-hidden="true"></svg>
              }
              Create and add questions
              <svg lucideIcon="arrow-right" [size]="18" aria-hidden="true"></svg>
            </button>
          </div>
        </form>
      </main>
    </div>
  `,
})
export class CreateProjectComponent implements OnInit {
  readonly steps = QUIZ_STEPS;
  projectForm: FormGroup;
  /** Whether "More options" is open. It starts closed; closing it keeps every choice. */
  moreOpen = false;
  isSubmitting = false;
  errorMessage = '';

  shortsFile: File | null = null;
  shortsFileType = '';
  shortsDuration = 0;
  shortsError = '';
  shortsProbing = false;
  shortsLanguage = 'auto';
  /** The limits the pipeline enforces, read from GET /shorts/limits. */
  shortsLimits: ShortsLimits = DEFAULT_SHORTS_LIMITS;

  readonly templates = TEMPLATE_OPTIONS;
  readonly textSizes: { value: 's' | 'm' | 'l'; label: string; hint: string }[] = [
    { value: 's', label: 'S', hint: 'Base' },
    { value: 'm', label: 'M', hint: 'Medium' },
    { value: 'l', label: 'L', hint: 'Large' },
  ];
  readonly labelStyles: { value: 'letters' | 'numbers'; label: string }[] = [
    { value: 'letters', label: 'A B C D' },
    { value: 'numbers', label: '1 2 3 4' },
  ];

  voices: Voice[] = ['Joanna', 'Matthew', 'Amy', 'Brian', 'Aditi'];

  constructor(
    private readonly fb: FormBuilder,
    private readonly api: ApiService,
    private readonly router: Router,
    private readonly shorts: ShortsService,
  ) {
    this.projectForm = this.fb.group({
      name: ['', [Validators.required, Validators.maxLength(100)]],
      template: ['classic', Validators.required],
      designOptions: this.fb.group({
        accent: [DESIGN_ACCENTS.classic[0].value],
        textSize: ['m'],
        showQuestionNumber: [true],
        answerLabels: ['letters'],
      }),
      voice: ['Joanna', Validators.required],
      watermarkText: ['', [Validators.maxLength(100)]],
      watermarkOpacity: [0.8],
      channelName: ['', [Validators.maxLength(60)]],
      includeShorts: [false],
    });

    this.projectForm.get('template')?.valueChanges.subscribe((value: Template) => {
      const accents = DESIGN_ACCENTS[value] ?? [];
      this.projectForm.get('designOptions.accent')?.setValue(accents[0]?.value ?? '');
    });
  }

  get accentOptions(): readonly AccentOption[] {
    return DESIGN_ACCENTS[this.projectForm.get('template')?.value as Template] ?? [];
  }

  get nameMissing(): boolean {
    const name = this.projectForm.get('name');
    return !!name?.hasError('required') && !!name?.touched;
  }

  get opacityPercent(): number {
    return Math.round((Number(this.projectForm.value.watermarkOpacity) || 0) * 100);
  }

  /** The optional groups that differ from the defaults, so a closed panel still says what it holds. */
  get changedOptions(): string[] {
    const v = this.projectForm.value;
    const d = v.designOptions ?? {};
    const defaultAccent = DESIGN_ACCENTS[v.template as Template]?.[0]?.value ?? '';
    const changed: string[] = [];
    if (d.textSize !== 'm' || d.showQuestionNumber !== true || d.answerLabels !== 'letters' || d.accent !== defaultAccent) {
      changed.push('Design options');
    }
    if ((v.watermarkText || '').trim()) changed.push('Watermark');
    if ((v.channelName || '').trim()) changed.push('Branding');
    if (v.includeShorts) changed.push('AI Shorts');
    return changed;
  }

  get moreState(): string {
    const changed = this.changedOptions;
    return changed.length ? `Changed: ${changed.join(', ')}` : 'Using the defaults';
  }

  onMoreToggle(event: Event): void {
    this.moreOpen = (event.target as HTMLDetailsElement).open;
  }

  ngOnInit(): void {
    // The enforced limits come from the API so the copy and the validation
    // can never drift from what the backend applies.
    this.shorts.getLimits().subscribe({
      next: (limits) => {
        if (limits && limits.maxDurationSeconds > 0 && limits.maxUploadBytes > 0) {
          this.shortsLimits = {
            ...DEFAULT_SHORTS_LIMITS,
            ...limits,
            languages: limits.languages?.length ? limits.languages : DEFAULT_SHORTS_LANGUAGES,
          };
        }
      },
      error: () => {},
    });
  }

  /** Shortest Shorts source the API accepts, from GET /shorts/limits (30 until it answers). */
  get shortsMinSeconds(): number {
    return minSourceSeconds(this.shortsLimits);
  }

  get shortsLanguages(): LanguageOption[] {
    return this.shortsLimits.languages?.length ? this.shortsLimits.languages : DEFAULT_SHORTS_LANGUAGES;
  }

  onShortsLanguageChange(event: Event): void {
    this.shortsLanguage = (event.target as HTMLSelectElement).value;
  }

  onShortsFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    if (input.files && input.files.length > 0) {
      this.handleShortsFile(input.files[0]);
    }
  }

  removeShortsFile(): void {
    this.shortsFile = null;
    this.shortsError = '';
  }

  private handleShortsFile(file: File): void {
    this.shortsFile = null;
    this.shortsError = '';
    const ext = file.name.split('.').pop()?.toLowerCase() ?? '';
    if (!['mp4', 'mov', 'mp3', 'wav'].includes(ext)) {
      this.shortsError = 'Unsupported file type. Use MP4, MOV, MP3 or WAV.';
      return;
    }
    if (file.size === 0) {
      this.shortsError = 'File is empty.';
      return;
    }
    this.shortsFile = file;
    this.shortsFileType = ext;
    this.probeShortsDuration(file, ext);
  }

  /** Read the media duration without uploading anything (shared off-screen probe). */
  private probeShortsDuration(file: File, ext: string): void {
    this.shortsProbing = true;
    const isAudio = ext === 'mp3' || ext === 'wav';
    probeMediaDuration(file, isAudio ? 'audio' : 'video').then(
      (duration) => {
        this.shortsProbing = false;
        if (!isFinite(duration) || duration <= 0 || duration > this.shortsLimits.maxDurationSeconds) {
          this.shortsError = `Could not use this file (bad duration or over ${this.shortsLimits.maxDurationMinutes} minutes).`;
          this.shortsFile = null;
          return;
        }
        // A file shorter than one clip can never make a clip (issue #286).
        const tooShort = tooShortMessage(duration, this.shortsLimits);
        if (tooShort) {
          this.shortsError = tooShort;
          this.shortsFile = null;
          return;
        }
        this.shortsDuration = duration;
      },
      () => {
        this.shortsProbing = false;
        this.shortsError = 'Could not read this media file.';
        this.shortsFile = null;
      },
    );
  }

  readonly sampleFilePath = SAMPLE_FILE_PATH;
  readonly formatGuidePath = FORMAT_GUIDE_PATH;

  createProject(): void {
    if (this.projectForm.invalid) return;

    const includeShorts = this.projectForm.value.includeShorts === true;
    if (includeShorts && (!this.shortsFile || this.shortsError || this.shortsProbing)) {
      // The Shorts file lives in "More options": open it so the reason is in view.
      this.moreOpen = true;
      if (!this.shortsFile && !this.shortsError && !this.shortsProbing) {
        this.shortsError = 'Choose a video or audio file for AI Shorts, or turn AI Shorts off.';
      }
      return;
    }

    this.isSubmitting = true;
    this.errorMessage = '';

    const request: CreateProjectRequest = {
      name: this.projectForm.value.name,
      template: this.projectForm.value.template,
      designOptions: this.projectForm.value.designOptions,
      voice: this.projectForm.value.voice,
    };
    const wmText = (this.projectForm.value.watermarkText || '').trim();
    if (wmText) {
      request.watermark = { text: wmText, opacity: this.projectForm.value.watermarkOpacity ?? 0.8 };
    }
    const channel = (this.projectForm.value.channelName || '').trim();
    if (channel) {
      request.branding = { channelName: channel };
    }
    if (includeShorts && this.shortsFile) {
      request.shorts = {
        fileType: this.shortsFileType,
        duration: this.shortsDuration,
        language: this.shortsLanguage,
        size: this.shortsFile.size,
      };
    }

    this.api.post<CreateProjectResponse>('/projects', request).subscribe({
      next: (project) => {
        if (project.shorts && this.shortsFile) {
          this.uploadShortsFile(project.shorts.uploadUrl, project.id, project.shorts.jobId);
        } else {
          this.isSubmitting = false;
          // Straight to the upload: the file is the step people need next.
          this.router.navigate(['/projects', project.id, 'upload']);
        }
      },
      error: (err) => {
        this.isSubmitting = false;
        this.errorMessage = err?.error?.message || 'Failed to create project. Please try again.';
      },
    });
  }

  private uploadShortsFile(uploadUrl: string, projectId: string, jobId: string): void {
    this.api.putAbsolute<void>(uploadUrl, this.shortsFile!).subscribe({
      next: () => {
        // Kick off the linked Shorts pipeline: without this the job would
        // sit in "uploaded" forever.
        this.api.post<unknown>(`/shorts/${jobId}/start`).subscribe({
          next: () => {
            this.isSubmitting = false;
            this.router.navigate(['/projects', projectId, 'upload']);
          },
          error: () => {
            this.isSubmitting = false;
            this.errorMessage =
              'Project created and file uploaded, but the Shorts pipeline failed to start. Retry it from Shorts history.';
            this.router.navigate(['/projects', projectId]);
          },
        });
      },
      error: (err) => {
        this.isSubmitting = false;
        this.errorMessage = err?.error?.message || 'Project created, but the shorts file upload failed. You can retry from the project page.';
        this.router.navigate(['/projects', projectId]);
      },
    });
  }
}
