import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { EFFECT_PRESETS } from './effect-presets';
import {
  ResolvedVideoEffects,
  ShortsJob,
  ShortsService,
  defaultVideoEffects,
  resolveVideoEffects,
} from './shorts.service';

/** Counts the API enforces. The panel stops adding rows at these limits. */
const MAX_PUNCH_INS = 10;
const MAX_FREEZES = 5;
const MAX_IMPACT_TEXTS = 5;
const MAX_SPEED_WINDOWS = 5;
const MAX_SHAKE_WINDOWS = 10;

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
  styles: [`
    :host { display: block; }
    .fx { text-align: left; }
    .fx-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-xs); }
    .fx-head-end { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-sm); }
    .groups { margin: var(--space-md) 0 0; padding: 0; border: 0; min-width: 0; display: flex; flex-direction: column; gap: var(--space-sm); }
    .groups:disabled { opacity: 0.6; }
    .grp { padding: var(--space-md); border: 1px solid var(--color-rule); border-radius: var(--radius-control); background: color-mix(in oklab, var(--color-paper-3) 35%, transparent); }
    .g-head { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: var(--space-sm); }
    .g-head > div { flex: 1; min-width: 12rem; }
    .g-title { margin: 0; font-size: var(--type-md); font-weight: 600; }
    .g-sub { margin: var(--space-3xs) 0 0; font-size: var(--type-xs); color: var(--color-muted); line-height: 1.45; }
    .g-hint { margin: var(--space-xs) 0 0; font-size: var(--type-xs); color: var(--color-muted); }
    .chips { margin-top: var(--space-sm); display: flex; flex-wrap: wrap; gap: var(--space-2xs); }
    .chips.tight { margin-top: var(--space-2xs); }
    .tog { display: inline-flex; align-items: center; gap: var(--space-xs); min-height: 44px; font-size: var(--type-sm); font-weight: 600; cursor: pointer; }
    .sw { appearance: none; -webkit-appearance: none; margin: 0; position: relative; width: 48px; height: 44px; flex: none; border: 0; background: transparent; cursor: pointer; }
    .sw::before { content: ''; position: absolute; left: 2px; top: 9px; width: 44px; height: 26px; border-radius: var(--radius-pill); background: var(--color-paper-3); border: 1px solid var(--color-muted); box-sizing: border-box; transition: background-color var(--dur-short) var(--ease-out); }
    .sw::after { content: ''; position: absolute; top: 13px; left: 6px; width: 18px; height: 18px; border-radius: 50%; background: var(--color-muted); transition: transform var(--dur-short) var(--ease-out); }
    .sw:checked::before { background: var(--color-accent); border-color: var(--color-accent); }
    .sw:checked::after { background: var(--color-accent-ink); transform: translateX(18px); }
    .sw:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .grid2, .grid3 { margin-top: var(--space-md); display: grid; gap: var(--space-md); }
    @media (min-width: 40rem) { .grid2 { grid-template-columns: 1fr 1fr; } .grid3 { grid-template-columns: repeat(3, 1fr); } }
    .ui-f.narrow { margin-top: var(--space-md); max-width: 28rem; }
    .sub-h { margin: var(--space-md) 0 0; font-size: var(--type-sm); font-weight: 600; }
    .rowx { margin-top: var(--space-xs); display: flex; flex-wrap: wrap; align-items: flex-end; gap: var(--space-xs); }
    .num { display: flex; flex-direction: column; gap: var(--space-3xs); width: 7rem; }
    .num.grow { flex: 1; min-width: 10rem; }
    .f-err { font-size: var(--type-xs); font-weight: 500; color: var(--color-negative); }
    .add { margin-top: var(--space-sm); white-space: nowrap; }
    .fx-foot { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-sm); padding-top: var(--space-xs); }
    @media (prefers-reduced-motion: reduce) { .sw::before, .sw::after { transition: none; } }
  `],
  template: `
    <section class="ui-card fx" aria-label="Video effects">
      <div class="fx-head">
        <h2 class="ui-card-title">Video effects</h2>
        <span class="fx-head-end">
          <span class="g-sub">{{ startStep ? 'Off by default, saved when you start' : draftMode ? 'Saved when you start the render' : 'Saved per job, used by the next render or retry' }}</span>
          @if (draftMode) {
            <button type="button" (click)="cancelled.emit()" class="ui-btn ui-btn--secondary">Cancel</button>
          }
        </span>
      </div>

      @if (loadError) {
        <p class="ui-alert ui-alert--bad" role="alert">
          Couldn't load the saved effects. Check your connection and reopen this panel.
        </p>
      } @else if (loaded && !editable) {
        <p class="ui-alert ui-alert--warn" role="note">
          Effects can change only before a render starts, or after a render failed. This job is locked.
        </p>
      }

      @if (loaded && !loadError) {
        <fieldset class="groups" [disabled]="!editable">
          <!-- One-click presets: each fills the panel with a combination the user can then edit. -->
          <div class="grp">
            <h3 class="g-title">Quick presets</h3>
            <p class="g-sub">Start from a combination, then adjust anything below.</p>
            <div class="chips" role="group" aria-label="Effect presets">
              @for (p of presets; track p.id) {
                <button
                  type="button"
                  (click)="applyPreset(p.id)"
                  [title]="p.blurb"
                  class="ui-btn ui-btn--secondary"
                >{{ p.label }}</button>
              }
            </div>
            @if (appliedPreset) {
              <p class="g-hint" role="status">{{ appliedPreset }}</p>
            }
          </div>

          <!-- Animated captions -->
          <div class="grp">
            <div class="g-head">
              <div>
                <h3 class="g-title">Animated captions</h3>
                <p class="g-sub">Words light up as they are spoken. While on, these replace the Caption style.</p>
              </div>
              <label class="tog">
                <input type="checkbox" class="sw" [(ngModel)]="draft.captions.enabled" aria-label="Turn on animated captions" />
                On
              </label>
            </div>
            <div class="grid2">
              <label class="ui-f">
                <span class="ui-flabel">Dim unspoken words <span class="ui-val">{{ draft.captions.intensity }}</span></span>
                <input type="range" class="ui-range" min="0" max="100" step="1" [(ngModel)]="draft.captions.intensity" aria-label="Animated captions dimming" />
              </label>
              <label class="ui-f">
                <span class="ui-flabel">Font</span>
                <select class="ui-field" [(ngModel)]="draft.captions.font" aria-label="Animated captions font">
                  @for (f of fonts; track f.id) {
                    <option [ngValue]="f.id">{{ f.label }}</option>
                  }
                </select>
              </label>
              <label class="ui-f">
                <span class="ui-flabel">Size <span class="ui-val">{{ draft.captions.fontSize }}</span></span>
                <input type="range" class="ui-range" min="8" max="72" step="1" [(ngModel)]="draft.captions.fontSize" aria-label="Animated captions size" />
              </label>
              <label class="ui-f">
                <span class="ui-flabel">Outline <span class="ui-val">{{ draft.captions.outline }}</span></span>
                <input type="range" class="ui-range" min="0" max="12" step="1" [(ngModel)]="draft.captions.outline" aria-label="Animated captions outline" />
              </label>
              <div class="ui-colors">
                <label class="ui-col">
                  Text
                  <input type="color" class="ui-swatch" [(ngModel)]="draft.captions.color" aria-label="Animated captions text color" />
                </label>
                <label class="ui-col">
                  Spoken word
                  <input type="color" class="ui-swatch" [(ngModel)]="draft.captions.highlight" aria-label="Animated captions highlight color" />
                </label>
              </div>
              <label class="ui-f">
                <span class="ui-flabel">Background</span>
                <select class="ui-field" [(ngModel)]="draft.captions.background" aria-label="Animated captions background">
                  @for (b of backgrounds; track b.id) {
                    <option [ngValue]="b.id">{{ b.label }}</option>
                  }
                </select>
              </label>
              <div>
                <span class="ui-flabel">Position</span>
                <div class="chips tight" role="group" aria-label="Animated captions position">
                  @for (pos of positions; track pos) {
                    <button
                      type="button"
                      (click)="draft.captions.position = pos"
                      [attr.aria-pressed]="draft.captions.position === pos"
                      class="ui-btn ui-btn--secondary ui-seg"
                    >{{ pos }}</button>
                  }
                </div>
              </div>
            </div>
          </div>

          <!-- Dynamic zoom -->
          <div class="grp">
            <div class="g-head">
              <div>
                <h3 class="g-title">Dynamic zoom</h3>
                <p class="g-sub">Pushes in at each punch-in time, then eases back out.</p>
              </div>
              <label class="tog">
                <input type="checkbox" class="sw" [(ngModel)]="draft.zoom.enabled" aria-label="Turn on dynamic zoom" />
                On
              </label>
            </div>
            <label class="ui-f narrow">
              <span class="ui-flabel">Zoom strength <span class="ui-val">{{ draft.zoom.intensity }}</span></span>
              <input type="range" class="ui-range" min="0" max="100" step="1" [(ngModel)]="draft.zoom.intensity" aria-label="Zoom strength" />
            </label>
            <p class="sub-h">Punch-ins</p>
            @for (p of draft.zoom.punchIns; track $index; let i = $index) {
              <div class="rowx">
                <label class="num">
                  <span class="ui-flabel">At (s)</span>
                  <input type="number" min="0" max="90" step="0.1" class="ui-field" [(ngModel)]="p.at" [attr.aria-label]="'Punch-in ' + (i + 1) + ' start in seconds'" [attr.aria-invalid]="blank(p.at)" />
                  @if (blank(p.at)) {
                    <span class="f-err" role="alert">Enter a start time in seconds.</span>
                  }
                </label>
                <label class="num">
                  <span class="ui-flabel">Length (s)</span>
                  <input type="number" min="0.5" max="6" step="0.1" class="ui-field" [(ngModel)]="p.duration" [attr.aria-label]="'Punch-in ' + (i + 1) + ' length in seconds'" [attr.aria-invalid]="blank(p.duration)" />
                  @if (blank(p.duration)) {
                    <span class="f-err" role="alert">Enter a length in seconds.</span>
                  }
                </label>
                <button type="button" (click)="removePunchIn(i)" class="ui-btn ui-btn--quiet">Remove</button>
              </div>
            }
            <button
              type="button"
              (click)="addPunchIn()"
              [disabled]="draft.zoom.punchIns.length >= maxPunchIns"
              class="ui-btn ui-btn--secondary add"
            >Add punch-in</button>
            <p class="g-hint">Up to {{ maxPunchIns }}. Times are seconds from the start of each clip.</p>
          </div>

          <!-- Cinematic grading -->
          <div class="grp">
            <div class="g-head">
              <div>
                <h3 class="g-title">Color grading</h3>
                <p class="g-sub">A cinematic, warm, cool or high-contrast look over the whole clip.</p>
              </div>
              <label class="tog">
                <input type="checkbox" class="sw" [(ngModel)]="draft.grade.enabled" aria-label="Turn on color grading" />
                On
              </label>
            </div>
            <div class="grid2">
              <label class="ui-f">
                <span class="ui-flabel">Look</span>
                <select class="ui-field" [(ngModel)]="draft.grade.look" aria-label="Color grading look">
                  @for (l of looks; track l.id) {
                    <option [ngValue]="l.id">{{ l.label }}</option>
                  }
                </select>
              </label>
              <label class="ui-f">
                <span class="ui-flabel">Strength <span class="ui-val">{{ draft.grade.intensity }}</span></span>
                <input type="range" class="ui-range" min="0" max="100" step="1" [(ngModel)]="draft.grade.intensity" aria-label="Color grading strength" />
              </label>
            </div>
          </div>

          <!-- Freeze frame and impact text -->
          <div class="grp">
            <div class="g-head">
              <div>
                <h3 class="g-title">Freeze frame and impact text</h3>
                <p class="g-sub">Holds the picture and adds short text at chosen times. Audio is silent during each hold, and captions move with it.</p>
              </div>
              <label class="tog">
                <input type="checkbox" class="sw" [(ngModel)]="draft.freeze.enabled" aria-label="Turn on freeze frame and impact text" />
                On
              </label>
            </div>
            <label class="ui-f narrow">
              <span class="ui-flabel">Impact text size <span class="ui-val">{{ draft.freeze.intensity }}</span></span>
              <input type="range" class="ui-range" min="0" max="100" step="1" [(ngModel)]="draft.freeze.intensity" aria-label="Impact text size" />
            </label>

            <p class="sub-h">Freezes</p>
            @for (h of draft.freeze.freezes; track $index; let i = $index) {
              <div class="rowx">
                <label class="num">
                  <span class="ui-flabel">At (s)</span>
                  <input type="number" min="0" max="90" step="0.1" class="ui-field" [(ngModel)]="h.at" [attr.aria-label]="'Freeze ' + (i + 1) + ' time in seconds'" [attr.aria-invalid]="blank(h.at)" />
                  @if (blank(h.at)) {
                    <span class="f-err" role="alert">Enter a time in seconds.</span>
                  }
                </label>
                <label class="num">
                  <span class="ui-flabel">Hold (s)</span>
                  <input type="number" min="0.2" max="3" step="0.1" class="ui-field" [(ngModel)]="h.duration" [attr.aria-label]="'Freeze ' + (i + 1) + ' hold length in seconds'" [attr.aria-invalid]="blank(h.duration)" />
                  @if (blank(h.duration)) {
                    <span class="f-err" role="alert">Enter a hold length in seconds.</span>
                  }
                </label>
                <button type="button" (click)="removeFreeze(i)" class="ui-btn ui-btn--quiet">Remove</button>
              </div>
            }
            <button
              type="button"
              (click)="addFreeze()"
              [disabled]="draft.freeze.freezes.length >= maxFreezes"
              class="ui-btn ui-btn--secondary add"
            >Add freeze</button>

            <p class="sub-h">Impact text</p>
            @for (t of draft.freeze.impactTexts; track $index; let i = $index) {
              <div class="rowx">
                <label class="num">
                  <span class="ui-flabel">At (s)</span>
                  <input type="number" min="0" max="90" step="0.1" class="ui-field" [(ngModel)]="t.at" [attr.aria-label]="'Impact text ' + (i + 1) + ' time in seconds'" [attr.aria-invalid]="blank(t.at)" />
                  @if (blank(t.at)) {
                    <span class="f-err" role="alert">Enter a time in seconds.</span>
                  }
                </label>
                <label class="num">
                  <span class="ui-flabel">Length (s)</span>
                  <input type="number" min="0.3" max="4" step="0.1" class="ui-field" [(ngModel)]="t.duration" [attr.aria-label]="'Impact text ' + (i + 1) + ' length in seconds'" [attr.aria-invalid]="blank(t.duration)" />
                  @if (blank(t.duration)) {
                    <span class="f-err" role="alert">Enter a length in seconds.</span>
                  }
                </label>
                <label class="num grow">
                  <span class="ui-flabel">Text, 16 characters</span>
                  <input type="text" maxlength="16" class="ui-field" [(ngModel)]="t.text" [attr.aria-label]="'Impact text ' + (i + 1) + ' words'" />
                </label>
                <button type="button" (click)="removeImpactText(i)" class="ui-btn ui-btn--quiet">Remove</button>
              </div>
            }
            <button
              type="button"
              (click)="addImpactText()"
              [disabled]="draft.freeze.impactTexts.length >= maxImpactTexts"
              class="ui-btn ui-btn--secondary add"
            >Add impact text</button>
          </div>

          <!-- Fade in and out -->
          <div class="grp">
            <div class="g-head">
              <div>
                <h3 class="g-title">Fade in and out</h3>
                <p class="g-sub">Eases the clip up from black at the start and down at the end. Picture and sound fade together. Set a side to 0 to skip it.</p>
              </div>
              <label class="tog">
                <input type="checkbox" class="sw" [(ngModel)]="draft.fade.enabled" aria-label="Turn on fade in and out" />
                On
              </label>
            </div>
            <div class="grid2">
              <label class="ui-f">
                <span class="ui-flabel">Fade in <span class="ui-val">{{ draft.fade.inDuration }}s</span></span>
                <input type="range" class="ui-range" min="0" max="2" step="0.1" [(ngModel)]="draft.fade.inDuration" aria-label="Fade-in length in seconds" />
              </label>
              <label class="ui-f">
                <span class="ui-flabel">Fade out <span class="ui-val">{{ draft.fade.outDuration }}s</span></span>
                <input type="range" class="ui-range" min="0" max="2" step="0.1" [(ngModel)]="draft.fade.outDuration" aria-label="Fade-out length in seconds" />
              </label>
            </div>
          </div>

          <!-- Speed ramping -->
          <div class="grp">
            <div class="g-head">
              <div>
                <h3 class="g-title">Speed</h3>
                <p class="g-sub">Slow motion or fast motion over chosen moments. 1.0 is normal speed, below 1 is slower, above 1 is faster. Audio is retimed to match.</p>
              </div>
              <label class="tog">
                <input type="checkbox" class="sw" [(ngModel)]="draft.speed.enabled" aria-label="Turn on speed" />
                On
              </label>
            </div>
            @if (speedFreezeConflict) {
              <p class="ui-alert ui-alert--warn" role="alert">
                Speed and freeze frame can't both be on for the same clip yet. Turn one off before saving.
              </p>
            }
            @for (w of draft.speed.windows; track $index; let i = $index) {
              <div class="rowx">
                <label class="num">
                  <span class="ui-flabel">At (s)</span>
                  <input type="number" min="0" max="90" step="0.1" class="ui-field" [(ngModel)]="w.at" [attr.aria-label]="'Speed window ' + (i + 1) + ' start in seconds'" [attr.aria-invalid]="blank(w.at)" />
                  @if (blank(w.at)) {
                    <span class="f-err" role="alert">Enter a start time in seconds.</span>
                  }
                </label>
                <label class="num">
                  <span class="ui-flabel">Length (s)</span>
                  <input type="number" min="0.3" max="90" step="0.1" class="ui-field" [(ngModel)]="w.duration" [attr.aria-label]="'Speed window ' + (i + 1) + ' length in seconds'" [attr.aria-invalid]="blank(w.duration)" />
                  @if (blank(w.duration)) {
                    <span class="f-err" role="alert">Enter a length in seconds.</span>
                  }
                </label>
                <label class="num">
                  <span class="ui-flabel">Rate <span class="ui-val">{{ w.rate }}x</span></span>
                  <input type="range" class="ui-range" min="0.25" max="4" step="0.25" [(ngModel)]="w.rate" [attr.aria-label]="'Speed window ' + (i + 1) + ' rate'" />
                </label>
                <button type="button" (click)="removeSpeedWindow(i)" class="ui-btn ui-btn--quiet">Remove</button>
              </div>
            }
            <button
              type="button"
              (click)="addSpeedWindow()"
              [disabled]="draft.speed.windows.length >= maxSpeedWindows"
              class="ui-btn ui-btn--secondary add"
            >Add speed window</button>
            <p class="g-hint">Up to {{ maxSpeedWindows }}. Times are seconds from the start of each clip. Windows can't overlap.</p>
          </div>

          <!-- Finish: film grain, glow, light leaks -->
          <div class="grp">
            <div class="g-head">
              <div>
                <h3 class="g-title">Film look</h3>
                <p class="g-sub">Optional grain, glow and a warm light leak over the whole clip. Set any to 0 to skip it.</p>
              </div>
              <label class="tog">
                <input type="checkbox" class="sw" [(ngModel)]="draft.finish.enabled" aria-label="Turn on film look" />
                On
              </label>
            </div>
            <div class="grid3">
              <label class="ui-f">
                <span class="ui-flabel">Grain <span class="ui-val">{{ draft.finish.grain }}</span></span>
                <input type="range" class="ui-range" min="0" max="100" step="1" [(ngModel)]="draft.finish.grain" aria-label="Film grain strength" />
              </label>
              <label class="ui-f">
                <span class="ui-flabel">Glow <span class="ui-val">{{ draft.finish.glow }}</span></span>
                <input type="range" class="ui-range" min="0" max="100" step="1" [(ngModel)]="draft.finish.glow" aria-label="Glow strength" />
              </label>
              <label class="ui-f">
                <span class="ui-flabel">Light leak <span class="ui-val">{{ draft.finish.lightLeak }}</span></span>
                <input type="range" class="ui-range" min="0" max="100" step="1" [(ngModel)]="draft.finish.lightLeak" aria-label="Light leak strength" />
              </label>
            </div>
          </div>

          <!-- Audio visualizer -->
          <div class="grp">
            <div class="g-head">
              <div>
                <h3 class="g-title">Audio visualizer</h3>
                <p class="g-sub">Draws the sound as a waveform or frequency bars. Useful for audio-only clips.</p>
              </div>
              <label class="tog">
                <input type="checkbox" class="sw" [(ngModel)]="draft.waveform.enabled" aria-label="Turn on the audio visualizer" />
                On
              </label>
            </div>
            <div class="grid2">
              <label class="ui-f">
                <span class="ui-flabel">Style</span>
                <select class="ui-field" [(ngModel)]="draft.waveform.style" aria-label="Visualizer style">
                  @for (s of waveStyles; track s.id) {
                    <option [ngValue]="s.id">{{ s.label }}</option>
                  }
                </select>
              </label>
              <label class="ui-f">
                <span class="ui-flabel">Height <span class="ui-val">{{ draft.waveform.height }}%</span></span>
                <input type="range" class="ui-range" min="10" max="50" step="1" [(ngModel)]="draft.waveform.height" aria-label="Visualizer height percent" />
              </label>
              <label class="ui-col">
                Color
                <input type="color" class="ui-swatch" [(ngModel)]="draft.waveform.color" aria-label="Visualizer color" />
              </label>
              <div>
                <span class="ui-flabel">Position</span>
                <div class="chips tight" role="group" aria-label="Visualizer position">
                  @for (pos of positions; track pos) {
                    <button
                      type="button"
                      (click)="draft.waveform.position = pos"
                      [attr.aria-pressed]="draft.waveform.position === pos"
                      class="ui-btn ui-btn--secondary ui-seg"
                    >{{ pos }}</button>
                  }
                </div>
              </div>
            </div>
          </div>

          <!-- Camera shake -->
          <div class="grp">
            <div class="g-head">
              <div>
                <h3 class="g-title">Camera shake</h3>
                <p class="g-sub">Adds a subtle shake over chosen moments. The frame is zoomed slightly first, so no black edges show.</p>
              </div>
              <label class="tog">
                <input type="checkbox" class="sw" [(ngModel)]="draft.shake.enabled" aria-label="Turn on camera shake" />
                On
              </label>
            </div>
            <label class="ui-f narrow">
              <span class="ui-flabel">Strength <span class="ui-val">{{ draft.shake.intensity }}</span></span>
              <input type="range" class="ui-range" min="0" max="100" step="1" [(ngModel)]="draft.shake.intensity" aria-label="Shake strength" />
            </label>
            <p class="sub-h">Shake moments</p>
            @for (w of draft.shake.windows; track $index; let i = $index) {
              <div class="rowx">
                <label class="num">
                  <span class="ui-flabel">At (s)</span>
                  <input type="number" min="0" max="90" step="0.1" class="ui-field" [(ngModel)]="w.at" [attr.aria-label]="'Shake ' + (i + 1) + ' start in seconds'" [attr.aria-invalid]="blank(w.at)" />
                  @if (blank(w.at)) {
                    <span class="f-err" role="alert">Enter a start time in seconds.</span>
                  }
                </label>
                <label class="num">
                  <span class="ui-flabel">Length (s)</span>
                  <input type="number" min="0.2" max="6" step="0.1" class="ui-field" [(ngModel)]="w.duration" [attr.aria-label]="'Shake ' + (i + 1) + ' length in seconds'" [attr.aria-invalid]="blank(w.duration)" />
                  @if (blank(w.duration)) {
                    <span class="f-err" role="alert">Enter a length in seconds.</span>
                  }
                </label>
                <button type="button" (click)="removeShakeWindow(i)" class="ui-btn ui-btn--quiet">Remove</button>
              </div>
            }
            <button
              type="button"
              (click)="addShakeWindow()"
              [disabled]="draft.shake.windows.length >= maxShakeWindows"
              class="ui-btn ui-btn--secondary add"
            >Add shake moment</button>
            <p class="g-hint">Up to {{ maxShakeWindows }}. Times are seconds from the start of each clip. They can't overlap.</p>
          </div>

          <div class="fx-foot">
            <button type="button" (click)="resetDraft()" class="ui-btn ui-btn--secondary">Reset to defaults</button>
            <span class="ui-spacer"></span>
            <button
              type="button"
              (click)="save()"
              [disabled]="saving || hasBlankNumbers() || speedFreezeConflict"
              class="ui-btn ui-btn--primary" data-testid="fx-save"
            >{{ saving ? 'Saving' : startStep ? 'Start processing' : draftMode ? 'Start render' : 'Save effects' }}</button>
          </div>
        </fieldset>

        @if (error) {
          <p class="ui-alert ui-alert--bad" role="alert">{{ error }}</p>
        } @else if (saved) {
          <p class="g-hint" role="status">Effects saved. They apply to the next render or retry.</p>
        }
      }
    </section>
  `,
})
export class ShortsEffectsPanelComponent implements OnInit {
  /** Job to edit. Empty reads the job id from the route, as the gallery and progress pages do. */
  @Input() jobId = '';
  /** Upload step: the job was just uploaded and has no saved settings, so the panel starts from the defaults and its save button starts processing. */
  @Input() startStep = false;
  /** Job list draft: loads the saved settings, saves only when they changed, and its save button starts the render. */
  @Input() draftMode = false;
  /** Emits the saved job after a successful save. */
  @Output() effectsSaved = new EventEmitter<ShortsJob>();
  /** Draft mode: the user closed the panel without starting the render. */
  @Output() cancelled = new EventEmitter<void>();
  /** Draft mode: the job as the server last returned it, and its settings as JSON, to tell whether the draft changed. */
  private savedJob: ShortsJob | null = null;
  private savedSettings = '';
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
  readonly maxSpeedWindows = MAX_SPEED_WINDOWS;
  readonly maxShakeWindows = MAX_SHAKE_WINDOWS;
  readonly waveStyles: Option<ResolvedVideoEffects['waveform']['style']>[] = [
    { id: 'waves', label: 'Waveform' },
    { id: 'bars', label: 'Frequency bars' },
  ];
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
    if (this.startStep) {
      this.status = 'uploaded';
      this.loaded = true;
      return;
    }
    this.jobId = this.jobId || this.route.snapshot.paramMap.get('id') || '';
    if (!this.jobId) {
      this.loadError = true;
      this.loaded = true;
      return;
    }
    this.shorts.getStatus(this.jobId).subscribe({
      next: (job) => {
        this.remember(job);
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

  /** A cleared number input reads as null. Those fields need a value before anything is saved. */
  blank(value: number | null | undefined): boolean {
    return value === null || value === undefined || Number.isNaN(value);
  }

  hasBlankNumbers(): boolean {
    const d = this.draft;
    return (
      d.zoom.punchIns.some((p) => this.blank(p.at) || this.blank(p.duration)) ||
      d.freeze.freezes.some((h) => this.blank(h.at) || this.blank(h.duration)) ||
      d.freeze.impactTexts.some((t) => this.blank(t.at) || this.blank(t.duration)) ||
      d.speed.windows.some((w) => this.blank(w.at) || this.blank(w.duration) || this.blank(w.rate)) ||
      d.shake.windows.some((w) => this.blank(w.at) || this.blank(w.duration))
    );
  }

  addShakeWindow(): void {
    if (this.draft.shake.windows.length < MAX_SHAKE_WINDOWS) {
      this.draft.shake.windows.push({ at: 0, duration: 0.5 });
    }
  }

  removeShakeWindow(index: number): void {
    this.draft.shake.windows.splice(index, 1);
  }

  /** Freeze and speed both retime the clip and cannot both be on (the server rejects it). */
  get speedFreezeConflict(): boolean {
    return this.draft.freeze.enabled && this.draft.speed.enabled;
  }

  addSpeedWindow(): void {
    if (this.draft.speed.windows.length < MAX_SPEED_WINDOWS) {
      this.draft.speed.windows.push({ at: 0, duration: 1, rate: 0.5 });
    }
  }

  removeSpeedWindow(index: number): void {
    this.draft.speed.windows.splice(index, 1);
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

  readonly presets = EFFECT_PRESETS;
  /** The blurb of the last applied preset, shown as a hint. */
  appliedPreset = '';

  /** Replaces the draft with a preset combination. The user edits it afterward;
   * nothing is saved until Save. */
  applyPreset(id: string): void {
    const preset = EFFECT_PRESETS.find((p) => p.id === id);
    if (!preset) return;
    this.draft = preset.build();
    this.appliedPreset = `Applied "${preset.label}". ${preset.blurb} Edit anything below, then save.`;
    this.saved = false;
    this.error = '';
  }

  /** Puts every effect back to its default. Nothing is saved until Save effects. */
  resetDraft(): void {
    this.draft = defaultVideoEffects();
    this.appliedPreset = '';
    this.saved = false;
    this.error = '';
  }

  /** Loads the job's settings into the draft and keeps a snapshot of them for draft mode. */
  private remember(job: ShortsJob): void {
    this.savedJob = job;
    this.savedSettings = JSON.stringify(resolveVideoEffects(job.effects));
    this.draft = resolveVideoEffects(job.effects);
  }

  save(): void {
    if (this.saving || !this.editable || !this.jobId || this.hasBlankNumbers()) return;
    // Draft mode writes only a changed draft. The parent starts the render either way.
    if (this.draftMode && this.savedJob && JSON.stringify(this.draft) === this.savedSettings) {
      this.effectsSaved.emit(this.savedJob);
      return;
    }
    this.saving = true;
    this.saved = false;
    this.error = '';
    this.shorts.updateEffects(this.jobId, this.draft).subscribe({
      next: (job) => {
        this.saving = false;
        this.saved = true;
        this.remember(job);
        this.effectsSaved.emit(job);
      },
      error: (err) => {
        this.saving = false;
        this.error = err?.error?.message || 'Could not save video effects.';
      },
    });
  }
}
