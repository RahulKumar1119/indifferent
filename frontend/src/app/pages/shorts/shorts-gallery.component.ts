import { Component, OnDestroy, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { firstValueFrom, forkJoin } from 'rxjs';
import { ActivatedRoute, Router } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import JSZip from 'jszip';
import { Clip, ShortsService, CaptionStyle, DEFAULT_CAPTION_STYLE, CaptionWord, ClipEdit } from './shorts.service';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { ToastsComponent } from '../../shared/components/toast/toasts.component';
import { ToastService } from '../../shared/components/toast/toast.service';
import { ConfirmDialogComponent } from '../../shared/components/confirm-dialog/confirm-dialog.component';
import { ShortsEffectsPanelComponent } from './shorts-effects-panel.component';

/** Hover previews play at most this long before pausing again. */
const HOVER_PREVIEW_MS = 2000;

/** Fractions sampled for the thumbnail frame strip. */
const STRIP_FRACS = [0.1, 0.25, 0.4, 0.55, 0.7];

interface CaptionPresetMeta {
  id: NonNullable<CaptionStyle['preset']>;
  label: string;
  blurb: string;
}

const CAPTION_PRESETS: CaptionPresetMeta[] = [
  { id: 'minimal', label: 'Minimal', blurb: 'Small, subtle, no box' },
  { id: 'bold', label: 'Bold', blurb: 'Large white on dark box' },
  { id: 'karaoke', label: 'Karaoke Highlight', blurb: 'Word-by-word sweep' },
  { id: 'clean', label: 'Clean Subtitles', blurb: 'Classic outlined text' },
];

@Component({
  selector: 'app-shorts-gallery',
  standalone: true,
  imports: [CommonModule, FormsModule, LucideDynamicIcon, NavbarComponent, ToastsComponent, ConfirmDialogComponent, ShortsEffectsPanelComponent],
  styles: [`
    :host { display: block; }
    @keyframes card-in { from { opacity: 0; transform: translateY(14px); } to { opacity: 1; transform: none; } }
    .card-enter { opacity: 0; animation: card-in .45s var(--ease-out) forwards; }
    .tools { display: flex; flex-wrap: wrap; gap: var(--space-2xs); }
    .tools .ui-btn[aria-expanded='true'] { background: var(--color-ink); border-color: var(--color-ink); color: var(--color-paper); }
    .cs-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-xs); }
    .presets { margin-top: var(--space-md); display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-xs); }
    @media (min-width: 64rem) { .presets { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
    .preset { display: flex; flex-direction: column; gap: var(--space-3xs); min-height: 44px; padding: var(--space-sm); border-radius: var(--radius-control); border: 1px solid var(--color-rule); background: transparent; color: var(--color-ink); text-align: left; font: inherit; cursor: pointer; }
    .preset:hover { background: var(--color-paper-3); }
    .preset[aria-pressed='true'] { border: 2px solid var(--color-accent); background: color-mix(in oklab, var(--color-accent) 7%, var(--app-card)); }
    .preset b { font-size: var(--type-sm); }
    .preset small { font-size: var(--type-xs); color: var(--color-muted); }
    .sample { margin-top: var(--space-2xs); display: block; padding: var(--space-sm) var(--space-xs); border-radius: 6px; text-align: center; background: var(--color-paper); }
    .sample.light { background: var(--color-paper-3); }
    .unlit { color: var(--color-muted); }
    .cs-grid { margin-top: var(--space-md); display: grid; gap: var(--space-md); }
    @media (min-width: 40rem) { .cs-grid { grid-template-columns: 1fr 1fr; } }
    .cs-foot { margin-top: var(--space-md); display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-xs); }
    .bad-text { margin: var(--space-xs) 0 0; color: var(--color-negative); font-size: var(--type-sm); font-weight: 500; }
    .selbar { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-xs); }
    .box { width: 1.25rem; height: 1.25rem; flex: none; border-radius: 4px; border: 2px solid var(--color-muted); display: inline-flex; align-items: center; justify-content: center; }
    .box.on { background: var(--color-accent); border-color: var(--color-accent); color: var(--color-accent-ink); }
    .count { font-size: var(--type-sm); color: var(--color-ink-2); font-variant-numeric: tabular-nums; }
    .bulk-head { display: flex; justify-content: space-between; font-size: var(--type-sm); color: var(--color-ink-2); }
    .track { margin-top: var(--space-xs); height: 8px; border-radius: var(--radius-pill); background: var(--color-paper-3); overflow: hidden; }
    .track div { height: 100%; border-radius: inherit; background: var(--color-accent); transition: width var(--dur-long) var(--ease-out); }
    .clips { display: grid; gap: var(--space-lg); grid-template-columns: minmax(0, 1fr); align-items: start; }
    @media (min-width: 36rem) { .clips { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
    @media (min-width: 64rem) { .clips { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
    @media (min-width: 110rem) { .clips { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
    .clip { display: flex; flex-direction: column; gap: var(--space-sm); padding: var(--space-sm); min-width: 0; }
    .clip.sel { outline: 2px solid var(--color-accent); outline-offset: -1px; }
    .player { position: relative; width: 100%; max-width: 26rem; margin-inline: auto; aspect-ratio: 9 / 16; border-radius: var(--radius-control); overflow: hidden; background: var(--color-paper); }
    .player video, .player img { position: absolute; inset: 0; width: 100%; height: 100%; }
    .player video { object-fit: contain; cursor: pointer; }
    .player img { object-fit: cover; }
    .cover { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: var(--space-xs); border: 0; background: transparent; color: var(--color-ink-2); cursor: pointer; font: inherit; font-size: var(--type-xs); }
    .cover:hover { color: var(--color-ink); }
    .cover:focus-visible { outline: 2px solid var(--color-focus); outline-offset: -3px; }
    .play { width: 3.5rem; height: 3.5rem; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; background: var(--color-accent); color: var(--color-accent-ink); box-shadow: 0 6px 20px -6px color-mix(in oklab, var(--color-ink) 60%, transparent); }
    .pick { position: absolute; top: 0; left: 0; z-index: 2; width: 44px; height: 44px; border: 0; background: transparent; cursor: pointer; display: inline-flex; align-items: center; justify-content: center; }
    .pick > span { width: 2rem; height: 2rem; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; background: color-mix(in oklab, var(--color-paper) 75%, transparent); color: var(--color-ink); border: 1px solid var(--color-rule); }
    .pick[aria-checked='true'] > span { background: var(--color-accent); border-color: var(--color-accent); color: var(--color-accent-ink); }
    .pick:focus-visible { outline: 2px solid var(--color-focus); outline-offset: -4px; border-radius: 50%; }
    .ring { position: absolute; top: var(--space-2xs); right: var(--space-2xs); width: 2.5rem; height: 2.5rem; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; background: color-mix(in oklab, var(--color-paper) 75%, transparent); }
    .ring svg { transform: rotate(-90deg); }
    .ring b { position: absolute; font-size: 0.625rem; color: var(--color-ink); font-variant-numeric: tabular-nums; }
    .rank { position: absolute; bottom: var(--space-xs); left: var(--space-xs); padding: var(--space-3xs) var(--space-xs); border-radius: var(--radius-pill); font-size: var(--type-xs); font-weight: 700; background: color-mix(in oklab, var(--color-paper) 75%, transparent); color: var(--color-ink); }
    .rank.top { background: var(--color-accent); color: var(--color-accent-ink); }
    .info { display: flex; flex-direction: column; gap: var(--space-3xs); padding-inline: var(--space-3xs); }
    .hook { margin: 0; font-weight: 600; line-height: 1.35; overflow-wrap: anywhere; }
    .meta { display: flex; flex-wrap: wrap; gap: 0 var(--space-sm); font-size: var(--type-xs); color: var(--color-muted); font-variant-numeric: tabular-nums; }
    .tag { padding: 0 var(--space-2xs); border-radius: var(--radius-pill); background: color-mix(in oklab, var(--color-accent) 14%, var(--app-card)); color: var(--color-accent-text); font-weight: 600; }
    .acts { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: var(--space-3xs); border-top: 1px solid var(--color-rule); padding-top: var(--space-2xs); }
    .act { min-height: 44px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 2px; border: 0; border-radius: var(--radius-control); background: transparent; color: var(--color-ink-2); font: 500 0.6875rem/1.1 var(--font-body); cursor: pointer; }
    .act:hover:not(:disabled) { background: var(--color-paper-3); color: var(--color-ink); }
    .act[aria-expanded='true'] { color: var(--color-accent-text); background: color-mix(in oklab, var(--color-accent) 9%, var(--app-card)); }
    .act:disabled { opacity: 0.5; cursor: progress; }
    .ok { color: var(--color-positive); }
    .strip { display: flex; gap: var(--space-xs); overflow-x: auto; padding-bottom: var(--space-3xs); }
    .frame { flex: none; padding: 0; border-radius: 6px; overflow: hidden; border: 2px solid transparent; background: transparent; cursor: pointer; }
    .frame[aria-pressed='true'] { border-color: var(--color-accent); }
    .frame img { display: block; width: 54px; height: 96px; object-fit: cover; }
    .frame-sk { flex: none; width: 54px; height: 96px; }
    .editor { padding: var(--space-md); border-radius: var(--radius-control); border: 1px solid var(--color-rule); background: color-mix(in oklab, var(--color-paper-3) 40%, transparent); display: flex; flex-direction: column; gap: var(--space-sm); }
    .editor p { margin: 0; }
    .small { font-size: var(--type-xs); color: var(--color-muted); }
    .words { margin: 0; padding: 0 var(--space-3xs) 0 0; list-style: none; max-height: 13rem; overflow-y: auto; display: flex; flex-direction: column; gap: var(--space-2xs); }
    .words li { display: flex; align-items: center; gap: var(--space-xs); }
    .words time { flex: none; width: 5.5rem; font-size: var(--type-xs); color: var(--color-muted); font-variant-numeric: tabular-nums; }
    .words .ui-field { flex: 1; min-width: 0; }
    .warn-text { color: var(--color-caution); font-size: var(--type-sm); }
    @media (prefers-reduced-motion: reduce) { .card-enter { opacity: 1; animation: none; } .track div { transition: none; } }
  `],
  template: `
    <div class="ui-app">
      <app-navbar></app-navbar>
      <app-toasts></app-toasts>
      <main id="main" class="ui-app-main hm" style="width: min(90rem, 100%)">
        <div class="ui-page-head">
          <div>
            <h1 class="ui-h1">Clips</h1>
            <p class="ui-lead">Ranked by predicted engagement. Hover to preview, download or share.</p>
          </div>
          <div class="tools">
            @if (clips.length > 1) {
              <button type="button" class="ui-btn ui-btn--secondary" [disabled]="zipping" (click)="downloadAll()">
                @if (zipping) {
                  <svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>Zipping
                } @else {
                  <svg lucideIcon="download" [size]="16" aria-hidden="true"></svg>Download all (ZIP)
                }
              </button>
              <button type="button" class="ui-btn ui-btn--secondary" (click)="copyAllCaptions()">
                <svg lucideIcon="copy" [size]="16" aria-hidden="true"></svg>{{ allCopied ? 'Copied' : 'Copy all captions' }}
              </button>
            }
            <button type="button" class="ui-btn ui-btn--secondary" (click)="goToThumbnails()">
              <svg lucideIcon="sparkles" [size]="16" aria-hidden="true"></svg>AI thumbnails
            </button>
            <button type="button" class="ui-btn ui-btn--secondary" (click)="toggleStylePanel()" [attr.aria-expanded]="styleOpen" aria-label="Caption styles">
              <svg lucideIcon="captions" [size]="16" aria-hidden="true"></svg>Captions
            </button>
            <button type="button" class="ui-btn ui-btn--secondary" (click)="toggleEffectsPanel()" [attr.aria-expanded]="effectsOpen" aria-label="Video effects">
              <svg lucideIcon="film" [size]="16" aria-hidden="true"></svg>Effects
            </button>
            <button type="button" class="ui-btn ui-btn--secondary" (click)="goToUpload()">New Shorts</button>
            <button type="button" class="ui-btn ui-btn--danger" (click)="pendingDelete = true" [disabled]="isLoading || !!error" aria-label="Delete this short and all its assets">
              <svg lucideIcon="trash-2" [size]="16" aria-hidden="true"></svg>Delete
            </button>
          </div>
        </div>

        @if (isLoading) {
          <div class="clips" role="status" aria-label="Loading clips">
            @for (i of [1, 2, 3]; track i) {
              <div class="ui-skeleton" style="aspect-ratio: 9 / 16"></div>
            }
          </div>
        }

        @if (error) {
          <div class="ui-card ui-error-card" role="alert">
            <p><svg lucideIcon="circle-x" [size]="16" aria-hidden="true"></svg>{{ error }}</p>
            <button type="button" class="ui-btn ui-btn--secondary" (click)="reload()">Try again</button>
          </div>
        }

        @if (!isLoading && !error && clips.length === 0) {
          <div class="ui-card ui-empty">
            <span class="ui-chip" aria-hidden="true"><svg lucideIcon="clapperboard" [size]="18"></svg></span>
            <p><strong>No clips were generated for this job.</strong></p>
            <p>Try a longer video for better results.</p>
          </div>
        }

        @if (clips.length > 0) {
          @if (styleOpen) {
            <section class="ui-card" aria-label="Caption styles">
              <div class="cs-head">
                <h2 class="ui-card-title">Caption style</h2>
                <span class="small">Saved per job, used by renders and retries</span>
              </div>
              @if (styleLoaded && styleLoadError) {
                <p class="ui-alert ui-alert--bad" role="alert">Couldn't load the saved style. Check your connection and reopen this panel.</p>
              } @else if (styleLoaded && !styleEditable) {
                <p class="ui-alert ui-alert--warn" role="note">This job is already rendered. Styles apply to future renders. Retry the job to re-render with a new style.</p>
              }
              <div class="presets" role="group" aria-label="Caption presets">
                @for (p of captionPresets; track p.id) {
                  <button type="button" class="preset" (click)="selectPreset(p.id)" [attr.aria-pressed]="styleDraft.preset === p.id">
                    <b>{{ p.label }}</b>
                    <small>{{ p.blurb }}</small>
                    <span class="sample hm-on-dark" aria-hidden="true">
                      @if (p.id === 'karaoke') {
                        <span [ngStyle]="swatchStyle(p.id)">Sample</span><span class="unlit" [style.font-size]="swatchStyle(p.id)['font-size']"> caption</span>
                      } @else {
                        <span [ngStyle]="swatchStyle(p.id)">Sample caption</span>
                      }
                    </span>
                    <span class="sample light hm-on-light" aria-hidden="true">
                      @if (p.id === 'karaoke') {
                        <span [ngStyle]="swatchStyle(p.id)">Sample</span><span class="unlit" [style.font-size]="swatchStyle(p.id)['font-size']"> caption</span>
                      } @else {
                        <span [ngStyle]="swatchStyle(p.id)">Sample caption</span>
                      }
                    </span>
                  </button>
                }
              </div>
              <div class="cs-grid">
                <label class="ui-f">
                  <span class="ui-flabel">Size <span class="ui-val">{{ styleDraft.fontSize }}</span></span>
                  <input type="range" class="ui-range" min="8" max="72" step="1" [(ngModel)]="styleDraft.fontSize" aria-label="Caption font size" />
                </label>
                <div class="ui-colors">
                  <label class="ui-col">Text <input type="color" class="ui-swatch" [(ngModel)]="styleDraft.color" aria-label="Caption text color" /></label>
                  <label class="ui-col">Highlight <input type="color" class="ui-swatch" [(ngModel)]="styleDraft.highlight" aria-label="Karaoke highlight color" /></label>
                </div>
              </div>
              <div class="cs-foot">
                <span class="ui-flabel" id="cap-pos-label">Position</span>
                <div class="ui-row" role="group" aria-labelledby="cap-pos-label">
                  @for (pos of positionOptions; track pos) {
                    <button type="button" class="ui-btn ui-btn--secondary ui-seg" (click)="styleDraft.position = pos" [attr.aria-pressed]="styleDraft.position === pos">{{ pos }}</button>
                  }
                </div>
                <span class="ui-spacer"></span>
                <button type="button" class="ui-btn ui-btn--secondary" (click)="resetStyleDraft()">Defaults</button>
                <button type="button" class="ui-btn ui-btn--primary" (click)="saveStyle()" [disabled]="styleSaving || !styleEditable"
                  [title]="!styleEditable && styleLoaded ? 'Styles apply to future renders only' : 'Save caption style'">{{ styleSaving ? 'Saving' : 'Save style' }}</button>
              </div>
              @if (styleError) {
                <p class="bad-text" role="alert">{{ styleError }}</p>
              }
            </section>
          }

          @if (effectsOpen) {
            <app-shorts-effects-panel></app-shorts-effects-panel>
          }

          <div class="selbar" aria-label="Clip selection">
            <button type="button" class="ui-btn ui-btn--secondary" (click)="toggleSelectAll()" [attr.aria-pressed]="allSelected">
              <span class="box" [class.on]="allSelected" aria-hidden="true">
                @if (allSelected) {
                  <svg lucideIcon="check" [size]="14"></svg>
                }
              </span>
              {{ allSelected ? 'Deselect all' : 'Select all' }}
            </button>
            <span class="count" role="status" aria-live="polite">{{ selectedCount }} of {{ clips.length }} selected</span>
            <span class="ui-spacer"></span>
            @if (selectedCount > 0) {
              <button type="button" class="ui-btn ui-btn--quiet" (click)="clearSelection()">Clear</button>
            }
            <button type="button" class="ui-btn ui-btn--primary" (click)="downloadSelected()" [disabled]="selectedCount === 0 || bulkActive">
              @if (bulkActive) {
                <svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>{{ bulkLabel }}
              } @else {
                <svg lucideIcon="download" [size]="16" aria-hidden="true"></svg>Download selected
              }
            </button>
          </div>

          @if (bulkActive || bulkFailed.length > 0) {
            <div class="ui-card" role="status" aria-live="polite">
              <div class="bulk-head"><span>{{ bulkLabel }}</span><span>{{ bulkDone }} / {{ bulkTotal }}</span></div>
              <div class="track"><div [style.width.%]="bulkTotal ? (bulkDone / bulkTotal) * 100 : 0"></div></div>
              @if (bulkFailed.length > 0 && !bulkActive) {
                <p class="bad-text">Couldn't download: {{ bulkFailed.join(', ') }}. The rest saved successfully. Retry these individually.</p>
              }
            </div>
          }

          <div class="clips">
            @for (clip of clips; track clip.clipId; let i = $index) {
              <article class="ui-card clip card-enter" [style.animation-delay]="(i * 0.05) + 's'" [attr.data-clip]="clip.clipId" [class.sel]="isSelected(clip)" [attr.aria-label]="'Clip ' + clip.rank">
                <div class="player hm-on-dark" (mouseenter)="hoverPlay(clip, $event)" (mouseleave)="hoverStop(clip)">
                  @if (showVideo[clip.clipId] && previewUrls[clip.clipId]) {
                    <video [src]="previewUrls[clip.clipId]" muted playsinline preload="metadata" (click)="togglePlay(clip, $event)"></video>
                  } @else if (thumbs[clip.clipId]) {
                    <img [src]="thumbs[clip.clipId]" [alt]="'Thumbnail for clip ' + clip.rank" loading="lazy" />
                    <button type="button" class="cover" (click)="playThumb(clip)" [attr.aria-label]="'Play clip ' + clip.rank">
                      <span class="play" aria-hidden="true"><svg lucideIcon="play" [size]="22"></svg></span>
                    </button>
                  } @else {
                    <button type="button" class="cover" [disabled]="loadingPreview[clip.clipId]" (click)="playThumb(clip)" [attr.aria-label]="'Preview clip ' + clip.rank">
                      @if (loadingPreview[clip.clipId]) {
                        <svg lucideIcon="loader-2" [size]="32" class="ui-spin" aria-hidden="true"></svg>
                      } @else {
                        <span class="play" aria-hidden="true"><svg lucideIcon="play" [size]="22"></svg></span>
                        <span>Tap to preview</span>
                      }
                    </button>
                  }
                  <button type="button" class="pick" (click)="toggleSelect(clip)" role="checkbox" [attr.aria-checked]="isSelected(clip)" [attr.aria-label]="(isSelected(clip) ? 'Deselect' : 'Select') + ' clip ' + clip.rank">
                    <span>
                      @if (isSelected(clip)) {
                        <svg lucideIcon="check" [size]="16" aria-hidden="true"></svg>
                      }
                    </span>
                  </button>
                  <span class="rank" [class]="rankBadgeClass(clip.rank)">#{{ clip.rank }}</span>
                  <span class="ring" [attr.aria-label]="'Engagement score ' + scorePercent(clip) + '%'" [title]="'Engagement score ' + scorePercent(clip) + '%'" role="img">
                    <svg width="36" height="36" viewBox="0 0 36 36" aria-hidden="true">
                      <circle cx="18" cy="18" r="15.5" fill="none" stroke="var(--color-rule)" stroke-width="3.5" />
                      <circle cx="18" cy="18" r="15.5" fill="none" [attr.stroke]="scoreColor(clip)" stroke-width="3.5" stroke-linecap="round" [attr.stroke-dasharray]="ringDash(clip)" stroke-dashoffset="0" />
                    </svg>
                    <b aria-hidden="true">{{ scorePercent(clip) }}</b>
                  </span>
                </div>

                <div class="info">
                  @if (clip.hookText) {
                    <p class="hook">{{ clip.hookText }}</p>
                  }
                  <p class="meta" style="margin: 0">
                    <span>{{ formatDuration(clip.duration) }}</span>
                    @if (boundsFor(clip); as bounds) {
                      <span>{{ formatTime(bounds.start) }} to {{ formatTime(bounds.end) }}</span>
                      @if (boundsEdited(clip)) {
                        <span class="tag">edited</span>
                      }
                    }
                  </p>
                </div>

                <div class="acts">
                  <button type="button" class="act" [disabled]="downloading[clip.clipId]" (click)="download(clip)" aria-label="Download clip">
                    @if (downloading[clip.clipId]) {
                      <svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>
                    } @else {
                      <svg lucideIcon="download" [size]="16" aria-hidden="true"></svg>
                    }
                    <span aria-hidden="true">Download</span>
                  </button>
                  <button type="button" class="act" (click)="copyCaption(clip)" aria-label="Copy caption">
                    @if (copied[clip.clipId]) {
                      <svg lucideIcon="check" [size]="16" class="ok" aria-hidden="true"></svg>
                    } @else {
                      <svg lucideIcon="copy" [size]="16" aria-hidden="true"></svg>
                    }
                    <span aria-hidden="true">{{ copied[clip.clipId] ? 'Copied' : 'Caption' }}</span>
                  </button>
                  <button type="button" class="act" (click)="share(clip)" aria-label="Share clip">
                    <svg lucideIcon="share-2" [size]="16" aria-hidden="true"></svg>
                    <span aria-hidden="true">Share</span>
                  </button>
                  <button type="button" class="act" (click)="toggleStrip(clip)" [attr.aria-expanded]="stripFor === clip.clipId" [attr.aria-label]="(stripFor === clip.clipId ? 'Hide' : 'Choose') + ' thumbnail frames for clip ' + clip.rank">
                    <svg lucideIcon="image" [size]="16" aria-hidden="true"></svg>
                    <span aria-hidden="true">Frames</span>
                  </button>
                  <button type="button" class="act" (click)="toggleEditor(clip)" [attr.aria-expanded]="editorFor === clip.clipId" [attr.aria-label]="'Edit boundaries and captions for clip ' + clip.rank">
                    <svg lucideIcon="pencil" [size]="16" aria-hidden="true"></svg>
                    <span aria-hidden="true">Edit</span>
                  </button>
                </div>

                @if (stripFor === clip.clipId) {
                  <div class="strip" role="group" aria-label="Thumbnail frame choices">
                    @if (stripLoading && !(stripThumbs[clip.clipId]?.length)) {
                      @for (i of [1, 2, 3, 4, 5]; track i) {
                        <div class="ui-skeleton frame-sk"></div>
                      }
                    } @else {
                      @for (f of stripThumbs[clip.clipId] ?? []; track $index; let fi = $index) {
                        <button type="button" class="frame" (click)="selectStripFrame(clip, fi)" [attr.aria-pressed]="thumbs[clip.clipId] === f" [attr.aria-label]="'Use frame ' + (fi + 1) + ' as thumbnail for clip ' + clip.rank">
                          <img [src]="f" [alt]="'Frame option ' + (fi + 1)" loading="lazy" />
                        </button>
                      }
                      @if (!(stripThumbs[clip.clipId]?.length)) {
                        <p class="small" style="margin: 0; padding: var(--space-xs) 0">Play the clip first, then pick a frame.</p>
                      }
                    }
                  </div>
                }

                @if (editorFor === clip.clipId) {
                  <div class="editor" aria-label="Clip editor">
                    @if (editorLoading || !editor) {
                      <div style="display: flex; flex-direction: column; gap: var(--space-xs)" aria-label="Loading editor">
                        <div class="ui-skeleton" style="height: 1rem; width: 66%"></div>
                        <div class="ui-skeleton" style="height: 2.5rem"></div>
                        <div class="ui-skeleton" style="height: 2.5rem"></div>
                      </div>
                    } @else {
                      <p class="small">
                        AI suggestion: <span style="font-variant-numeric: tabular-nums">{{ formatTime(editor.aiStart) }} to {{ formatTime(editor.aiEnd) }}</span>
                        @if (editor.hasSavedEdit) {
                          <span class="tag">customized</span>
                        }
                      </p>
                      <label class="ui-f">
                        <span class="ui-flabel">Start <span class="ui-val">{{ formatTime(editor.start) }}</span></span>
                        <input type="range" class="ui-range" min="0" [max]="editor.sourceDuration" step="0.1" [(ngModel)]="editor.start" aria-label="Clip start time in seconds" />
                      </label>
                      <label class="ui-f">
                        <span class="ui-flabel">End <span class="ui-val">{{ formatTime(editor.end) }}</span></span>
                        <input type="range" class="ui-range" min="0" [max]="editor.sourceDuration" step="0.1" [(ngModel)]="editor.end" aria-label="Clip end time in seconds" />
                      </label>
                      <div><button type="button" class="ui-btn ui-btn--secondary" (click)="previewRange()"><svg lucideIcon="play" [size]="14" aria-hidden="true"></svg>Preview range</button></div>
                      <div>
                        <p class="ui-flabel" style="justify-content: flex-start">Captions <span class="small" style="font-weight: 400">Timings preserved, text is editable</span></p>
                        @if (editor.words.length === 0) {
                          <p class="small">No captions in this range.</p>
                        } @else {
                          <ul class="words">
                            @for (w of editor.words; track $index; let wi = $index) {
                              <li>
                                <time>{{ formatTime(w.start) }}</time>
                                <input type="text" class="ui-field" [(ngModel)]="editor.words[wi].text" maxlength="60" [attr.aria-label]="'Caption word ' + (wi + 1) + ' at ' + formatTime(w.start)" />
                              </li>
                            }
                          </ul>
                        }
                      </div>
                      @if (!editorValid() && !editor.saving) {
                        <p class="bad-text" role="alert">Check the range: need 0 ≤ start &lt; end ≤ {{ formatTime(editor.sourceDuration) }}, at least 1s long, and no empty captions.</p>
                      }
                      @if (editor.dropped !== null && editor.dropped > 0) {
                        <p class="warn-text">{{ editor.dropped }} caption{{ editor.dropped === 1 ? '' : 's' }} fall outside the new range and won't render.</p>
                      }
                      @if (editor.error) {
                        <p class="bad-text" role="alert">{{ editor.error }}</p>
                      }
                      <div class="ui-row">
                        <button type="button" class="ui-btn ui-btn--primary" (click)="saveEdit()" [disabled]="editor.saving || !editorValid()">{{ editor.saving ? 'Saving' : 'Save changes' }}</button>
                        @if (editor.hasSavedEdit) {
                          <button type="button" class="ui-btn ui-btn--secondary" (click)="resetEdit()" [disabled]="editor.saving">Reset to AI suggestion</button>
                        }
                      </div>
                      <p class="small">Saved edits apply on retry or re-render. The original clip stays until the new render succeeds.</p>
                    }
                  </div>
                }
              </article>
            }
          </div>
        }
      </main>
    </div>

    <app-confirm-dialog
      [open]="pendingDelete"
      title="Delete Short?"
      [message]="'Permanently delete Short ' + jobLabel + ' (' + clips.length + (clips.length === 1 ? ' clip' : ' clips') + ') and all of its assets?'"
      [details]="['Original uploaded media', 'AI transcript and ranking data', 'All rendered vertical clips', 'Linked project references are cleaned up']"
      [busy]="deleting"
      (confirmed)="confirmDelete()"
      (cancelled)="pendingDelete = false"
    />
  `,
})
export class ShortsGalleryComponent implements OnInit, OnDestroy {
  private jobId = '';

  clips: Clip[] = [];
  isLoading = true;
  error = '';

  previewUrls: Record<string, string> = {};
  loadingPreview: Record<string, boolean> = {};
  downloading: Record<string, boolean> = {};
  copied: Record<string, boolean> = {};
  zipping = false;
  allCopied = false;
  pendingDelete = false;
  deleting = false;

  /** Per-card poster thumbnails captured from the video (data URLs). */
  thumbs: Record<string, string> = {};
  /** Cards currently showing the live video element instead of the poster. */
  showVideo: Record<string, boolean> = {};
  /** Open frame strip (clipId) + captured strip frames + loading flag. */
  stripFor: string | null = null;
  stripThumbs: Record<string, string[]> = {};
  stripLoading = false;

  /** Per-card clip editor (one open at a time): trim + caption corrections. */
  editorFor: string | null = null;
  editorLoading = false;
  editor: {
    clipId: string;
    aiStart: number;
    aiEnd: number;
    start: number;
    end: number;
    sourceDuration: number;
    words: CaptionWord[];
    hasSavedEdit: boolean;
    saving: boolean;
    error: string;
    dropped: number | null;
  } | null = null;
  /** Applied bounds shown on cards (from saves this session). */
  appliedEdits: Record<string, { start: number; end: number }> = {};
  private previewToken = 0;

  /** Caption studio state (per-job reusable style). */
  styleOpen = false;
  styleLoaded = false;
  styleLoadError = false;
  styleSaving = false;
  styleError = '';
  styleJobStatus = '';
  styleDraft: Required<CaptionStyle> = { ...DEFAULT_CAPTION_STYLE };
  readonly captionPresets = CAPTION_PRESETS;
  readonly positionOptions: NonNullable<CaptionStyle['position']>[] = ['top', 'middle', 'bottom'];

  /** Video effects panel (its own component loads and saves the settings). */
  effectsOpen = false;

  /** Selected clip IDs. Keyed by stable clipId so selection survives
   * re-sorts and list refreshes; pruned when the list changes. */
  private readonly selectedIds = new Set<string>();

  bulkActive = false;
  bulkDone = 0;
  bulkTotal = 0;
  bulkFailed: string[] = [];

  get jobLabel(): string {
    return this.jobId ? this.jobId.slice(0, 8) : 'this job';
  }

  get selectedCount(): number {
    return this.selectedIds.size;
  }

  get allSelected(): boolean {
    return this.clips.length > 0 && this.clips.every((c) => this.selectedIds.has(c.clipId));
  }

  get bulkLabel(): string {
    if (!this.bulkActive) return '';
    return `Fetching ${Math.min(this.bulkDone + 1, this.bulkTotal)} of ${this.bulkTotal}…`;
  }

  isSelected(clip: Clip): boolean {
    return this.selectedIds.has(clip.clipId);
  }

  toggleSelect(clip: Clip): void {
    if (this.selectedIds.has(clip.clipId)) {
      this.selectedIds.delete(clip.clipId);
    } else {
      this.selectedIds.add(clip.clipId);
    }
  }

  toggleSelectAll(): void {
    if (this.allSelected) {
      this.selectedIds.clear();
    } else {
      this.clips.forEach((c) => this.selectedIds.add(c.clipId));
    }
  }

  clearSelection(): void {
    this.selectedIds.clear();
  }

  private pruneSelection(): void {
    const live = new Set(this.clips.map((c) => c.clipId));
    [...this.selectedIds].forEach((id) => {
      if (!live.has(id)) this.selectedIds.delete(id);
    });
  }

  private hoverTimers: Record<string, ReturnType<typeof setTimeout>> = {};

  constructor(
    private readonly route: ActivatedRoute,
    private readonly router: Router,
    private readonly shorts: ShortsService,
    private readonly toasts: ToastService,
  ) {}

  ngOnInit(): void {
    this.jobId = this.route.snapshot.paramMap.get('id') || '';
    if (!this.jobId) {
      this.router.navigate(['/shorts']);
      return;
    }
    this.reload();
  }

  reload(): void {
    if (!this.jobId) return;
    this.isLoading = true;
    this.error = '';
    this.shorts.listClips(this.jobId).subscribe({
      next: (clips) => {
        this.clips = [...clips].sort((a, b) => a.rank - b.rank);
        this.pruneSelection();
        this.isLoading = false;
        // Warm poster thumbnails in the background (sequential, quiet):
        // cards show a representative frame without any tap.
        void this.warmThumbnails();
      },
      error: (err) => {
        this.isLoading = false;
        this.error = err?.error?.message || 'Failed to load clips. Please try again.';
      },
    });
  }

  /** Fetch URLs + capture posters for all clips, skipping restoring ones
   * silently (their tap-to-preview fallback stays). Never throws. */
  private async warmThumbnails(): Promise<void> {
    for (const clip of this.clips) {
      if (this.thumbs[clip.clipId] || this.previewUrls[clip.clipId]) continue;
      try {
        const res = await firstValueFrom(this.shorts.getClipUrl(this.jobId, clip.clipId));
        if (!res.url) continue;
        this.previewUrls[clip.clipId] = res.url;
        await this.captureThumb(clip);
      } catch {
        // Poster stays on the tap-to-preview fallback.
      }
    }
  }

  scorePercent(clip: Clip): number {
    return Math.round(clip.score * 100);
  }

  scoreColor(clip: Clip): string {
    const pct = this.scorePercent(clip);
    if (pct > 80) return 'var(--color-positive)';
    if (pct >= 50) return 'var(--color-caution)';
    return 'var(--color-negative)';
  }

  ringDash(clip: Clip): string {
    const circ = 2 * Math.PI * 15.5;
    return `${(this.scorePercent(clip) / 100) * circ} ${circ}`;
  }

  rankBadgeClass(rank: number): string {
    return rank === 1 ? 'rank top' : 'rank';
  }
  preview(clip: Clip): Promise<void> {
    if (this.previewUrls[clip.clipId] || this.loadingPreview[clip.clipId]) {
      return Promise.resolve();
    }
    this.loadingPreview[clip.clipId] = true;
    return new Promise((resolve) => {
      this.shorts.getClipUrl(this.jobId, clip.clipId).subscribe({
        next: (res) => {
          this.loadingPreview[clip.clipId] = false;
          if (!res.url) {
            // 202 RESTORING lands here (2xx): archived clip is being restored.
            this.error = res.message || 'This clip is in cold storage and is being restored. Please check back in a few hours.';
            resolve();
            return;
          }
          this.previewUrls[clip.clipId] = res.url;
          // Capture the poster thumbnail lazily (async, non-blocking).
          void this.captureThumb(clip).catch(() => {});
          resolve();
        },
        error: () => {
          this.loadingPreview[clip.clipId] = false;
          this.error = 'Failed to load clip preview. Please try again.';
          resolve();
        },
      });
    });
  }

  /** Tap the poster: reveal the player and start playback (with sound). */
  playThumb(clip: Clip): void {
    if (this.showVideo[clip.clipId]) return;
    void this.preview(clip).then(() => {
      if (!this.previewUrls[clip.clipId]) return;
      this.showVideo[clip.clipId] = true;
      requestAnimationFrame(() => {
        const el = document.querySelector(`[data-clip="${clip.clipId}"] video`);
        if (el instanceof HTMLVideoElement) {
          el.muted = false;
          el.play().catch(() => {
            el.muted = true;
            el.play().catch(() => {});
          });
        }
      });
    });
  }

  /** Capture one poster frame at a fraction of the duration (data URL). */
  private captureFrame(url: string, frac: number): Promise<string> {
    return new Promise((resolve, reject) => {
      const video = document.createElement('video');
      video.muted = true;
      video.preload = 'auto';
      video.crossOrigin = 'anonymous';
      const done = (fn: () => void) => {
        window.clearTimeout(timer);
        video.pause();
        video.removeAttribute('src');
        fn();
      };
      const timer = window.setTimeout(() => done(() => reject(new Error('frame timeout'))), 10000);
      video.onloadedmetadata = () => {
        const dur = video.duration || 0;
        if (!isFinite(dur) || dur <= 0) {
          done(() => reject(new Error('no duration')));
          return;
        }
        video.currentTime = Math.min(Math.max(0, frac * dur), Math.max(0, dur - 0.1));
      };
      video.onseeked = () => {
        try {
          const canvas = document.createElement('canvas');
          canvas.width = 270;
          canvas.height = 480;
          const ctx = canvas.getContext('2d');
          if (!ctx) {
            done(() => reject(new Error('no ctx')));
            return;
          }
          // Cover-crop the frame into the 9:16 poster.
          const vw = video.videoWidth || 270;
          const vh = video.videoHeight || 480;
          const scale = Math.max(270 / vw, 480 / vh);
          const dw = vw * scale;
          const dh = vh * scale;
          ctx.drawImage(video, (270 - dw) / 2, (480 - dh) / 2, dw, dh);
          const url = canvas.toDataURL('image/jpeg', 0.7);
          done(() => resolve(url));
        } catch (e) {
          done(() => reject(e instanceof Error ? e : new Error('capture failed')));
        }
      };
      video.onerror = () => done(() => reject(new Error('video error')));
      video.src = url;
    });
  }

  private async captureThumb(clip: Clip): Promise<void> {
    const url = this.previewUrls[clip.clipId];
    if (!url || this.thumbs[clip.clipId]) return;
    try {
      this.thumbs[clip.clipId] = await this.captureFrame(url, 0.25);
    } catch {
      // Poster stays on the tap-to-preview fallback.
    }
  }

  toggleStrip(clip: Clip): void {
    if (this.stripFor === clip.clipId) {
      this.stripFor = null;
      return;
    }
    this.stripFor = clip.clipId;
    if (this.stripThumbs[clip.clipId] || this.stripLoading) return;
    const url = this.previewUrls[clip.clipId];
    if (!url) return;
    this.stripLoading = true;
    (async () => {
      const frames: string[] = [];
      for (const frac of STRIP_FRACS) {
        try {
          frames.push(await this.captureFrame(url, frac));
        } catch {
          // Skip undecodable positions; keep the strip usable.
        }
      }
      // Only apply if the user hasn't moved on (or the job reloaded).
      if (this.stripFor === clip.clipId) {
        this.stripThumbs[clip.clipId] = frames;
      }
      this.stripLoading = false;
    })();
  }

  selectStripFrame(clip: Clip, index: number): void {
    const frames = this.stripThumbs[clip.clipId];
    if (frames && frames[index]) {
      this.thumbs[clip.clipId] = frames[index];
    }
  }

  // ------------------------------------------------------------------ editor

  /** Effective bounds for display: saved edit wins, else AI bounds. */
  boundsFor(clip: Clip): { start: number; end: number } | null {
    const applied = this.appliedEdits[clip.clipId];
    if (applied) return applied;
    if (typeof clip.start === 'number' && typeof clip.end === 'number' && clip.end > clip.start) {
      return { start: clip.start, end: clip.end };
    }
    return null;
  }

  boundsEdited(clip: Clip): boolean {
    return this.appliedEdits[clip.clipId] !== undefined;
  }

  toggleEditor(clip: Clip): void {
    if (this.editorFor === clip.clipId) {
      this.editorFor = null;
      this.editor = null;
      return;
    }
    this.editorFor = clip.clipId;
    this.editor = null;
    this.editorLoading = true;
    const aiStart = typeof clip.start === 'number' ? clip.start : 0;
    const aiEnd = typeof clip.end === 'number' && clip.end > aiStart ? clip.end : 0;
    forkJoin({
      job: this.shorts.getStatus(this.jobId),
      captions: this.shorts.getClipCaptions(this.jobId, clip.clipId),
    }).subscribe({
      next: ({ job, captions }) => {
        if (this.editorFor !== clip.clipId) return;
        const saved = job.clipEdits?.[clip.clipId];
        this.editorLoading = false;
        this.editor = {
          clipId: clip.clipId,
          aiStart,
          aiEnd,
          start: saved?.start ?? captions.start,
          end: saved?.end ?? captions.end,
          sourceDuration: job.sourceDuration || Math.max(aiEnd, captions.end, 1),
          words: (saved?.words?.length ? saved.words : captions.words).map((w) => ({ ...w })),
          hasSavedEdit: !!saved,
          saving: false,
          error: '',
          dropped: null,
        };
      },
      error: () => {
        if (this.editorFor !== clip.clipId) return;
        this.editorLoading = false;
        this.editor = {
          clipId: clip.clipId,
          aiStart,
          aiEnd,
          start: aiStart,
          end: aiEnd,
          sourceDuration: Math.max(aiEnd, 1),
          words: [],
          hasSavedEdit: false,
          saving: false,
          error: 'Could not load captions. Boundary trim is still available.',
          dropped: null,
        };
      },
    });
  }

  editorValid(): boolean {
    const ed = this.editor;
    if (!ed) return false;
    if (!isFinite(ed.start) || !isFinite(ed.end)) return false;
    if (!(ed.start >= 0 && ed.end > ed.start && ed.end <= ed.sourceDuration + 0.5)) return false;
    if (ed.end - ed.start < 1) return false;
    return ed.words.every((w) => w.text.trim().length > 0 && isFinite(w.start) && isFinite(w.end) && w.end > w.start);
  }

  saveEdit(): void {
    const ed = this.editor;
    if (!ed || ed.saving || !this.editorValid()) return;
    ed.saving = true;
    ed.error = '';
    ed.dropped = null;
    this.shorts
      .updateClipEdit(this.jobId, ed.clipId, {
        start: ed.start,
        end: ed.end,
        words: ed.words.map((w) => ({ text: w.text.trim(), start: w.start, end: w.end })),
      })
      .subscribe({
        next: (res) => {
          ed.saving = false;
          ed.dropped = res.droppedWords ?? 0;
          ed.hasSavedEdit = true;
          this.appliedEdits[ed.clipId] = { start: ed.start, end: ed.end };
          this.toasts.show(
            ed.dropped > 0
              ? `Saved. ${ed.dropped} caption${ed.dropped === 1 ? '' : 's'} fall outside the new range and won't render.`
              : 'Clip edit saved. Applies on retry or re-render.',
            'success',
          );
        },
        error: (err) => {
          ed.saving = false;
          ed.error = err?.error?.message || 'Could not save. Please try again.';
        },
      });
  }

  resetEdit(): void {
    const ed = this.editor;
    if (!ed || ed.saving) return;
    ed.saving = true;
    ed.error = '';
    this.shorts.updateClipEdit(this.jobId, ed.clipId, { reset: true }).subscribe({
      next: () => {
        ed.saving = false;
        ed.hasSavedEdit = false;
        ed.dropped = null;
        ed.start = ed.aiStart;
        ed.end = ed.aiEnd;
        delete this.appliedEdits[ed.clipId];
        this.toasts.show('Restored the AI suggestion.', 'success');
      },
      error: (err) => {
        ed.saving = false;
        ed.error = err?.error?.message || 'Could not reset. Please try again.';
      },
    });
  }

  /** Preview the edited range in the card player (pauses at the new end). */
  async previewRange(): Promise<void> {
    const ed = this.editor;
    if (!ed) return;
    const clip = this.clips.find((c) => c.clipId === ed.clipId);
    if (!clip) return;
    const token = ++this.previewToken;
    await this.preview(clip);
    if (token !== this.previewToken || this.editorFor !== clip.clipId) return;
    if (!this.previewUrls[clip.clipId]) return;
    this.showVideo[clip.clipId] = true;
    requestAnimationFrame(() => {
      if (token !== this.previewToken) return;
      const el = document.querySelector(`[data-clip="${clip.clipId}"] video`);
      if (!(el instanceof HTMLVideoElement)) return;
      el.currentTime = Math.min(Math.max(0, ed.start), Math.max(0, (el.duration || ed.end) - 0.1));
      const stopAtEnd = () => {
        if (el.currentTime >= ed.end) {
          el.pause();
          el.removeEventListener('timeupdate', stopAtEnd);
        }
      };
      el.addEventListener('timeupdate', stopAtEnd);
      el.muted = false;
      el.play().catch(() => {
        el.muted = true;
        el.play().catch(() => {});
      });
    });
  }

  /** Desktop hover: ensure a URL, then autoplay muted for a short burst. */
  hoverPlay(clip: Clip, event: MouseEvent): void {
    if (window.matchMedia('(hover: none)').matches) return;
    const video = (event.currentTarget as HTMLElement).querySelector('video');
    if (video) {
      video.muted = true;
      video.play().catch(() => {});
      this.clearHoverTimer(clip.clipId);
      this.hoverTimers[clip.clipId] = setTimeout(() => video.pause(), HOVER_PREVIEW_MS);
      return;
    }
    if (!this.previewUrls[clip.clipId] && !this.loadingPreview[clip.clipId]) {
      void this.preview(clip).then(() => {
        this.showVideo[clip.clipId] = true;
        requestAnimationFrame(() => {
          const el = document.querySelector(`[data-clip="${clip.clipId}"] video`);
          if (el instanceof HTMLVideoElement) {
            el.muted = true;
            el.play().catch(() => {});
            this.clearHoverTimer(clip.clipId);
            this.hoverTimers[clip.clipId] = setTimeout(() => el.pause(), HOVER_PREVIEW_MS);
          }
        });
      });
    }
  }

  hoverStop(clip: Clip): void {
    this.clearHoverTimer(clip.clipId);
    const el = document.querySelector(`[data-clip="${clip.clipId}"] video`);
    if (el instanceof HTMLVideoElement && !el.paused) {
      el.pause();
    }
  }

  /** Mobile tap: toggle play on the loaded video. */
  togglePlay(clip: Clip, event: MouseEvent): void {
    const video = event.target as HTMLVideoElement;
    if (video.paused) {
      video.muted = false;
      video.play().catch(() => {});
    } else {
      video.pause();
    }
    this.clearHoverTimer(clip.clipId);
  }

  download(clip: Clip): void {
    if (this.downloading[clip.clipId]) return;
    this.downloading[clip.clipId] = true;
    this.withClipUrl(clip, (url) => {
      this.downloading[clip.clipId] = false;
      this.triggerDownload(url, `short-${clip.rank}.mp4`);
    }, () => {
      this.downloading[clip.clipId] = false;
      this.error = 'Failed to download clip. Please try again.';
    });
  }

  copyCaption(clip: Clip): void {
    this.copyText(this.captionText(clip)).then((ok) => {
      if (ok) {
        this.copied[clip.clipId] = true;
        this.toasts.show('Caption copied to clipboard.', 'success');
        setTimeout(() => { this.copied[clip.clipId] = false; }, 2000);
      } else {
        this.error = 'Copy failed. Please try again.';
      }
    });
  }

  copyAllCaptions(): void {
    const text = this.clips.map((c) => this.captionText(c)).join('\n\n');
    this.copyText(text).then((ok) => {
      if (ok) {
        this.allCopied = true;
        this.toasts.show(`${this.clips.length} captions copied to clipboard.`, 'success');
        setTimeout(() => { this.allCopied = false; }, 2000);
      } else {
        this.error = 'Copy failed. Please try again.';
      }
    });
  }

  async downloadSelected(): Promise<void> {
    const targets = this.clips.filter((c) => this.selectedIds.has(c.clipId));
    if (targets.length === 0 || this.bulkActive) return;
    this.bulkActive = true;
    this.bulkDone = 0;
    this.bulkTotal = targets.length;
    this.bulkFailed = [];
    this.error = '';
    try {
      const zip = new JSZip();
      for (const clip of targets) {
        try {
          const url = await this.clipUrl(clip);
          const res = await fetch(url);
          if (!res.ok) throw new Error(`fetch ${res.status}`);
          zip.file(`short-${clip.rank}.mp4`, await res.blob());
        } catch {
          this.bulkFailed.push(`clip #${clip.rank}`);
        } finally {
          this.bulkDone++;
        }
      }
      if (this.bulkDone - this.bulkFailed.length === 0) {
        throw new Error('all-failed');
      }
      const content = await zip.generateAsync({ type: 'blob' });
      const objectUrl = URL.createObjectURL(content);
      this.triggerDownload(objectUrl, `shorts-selected-${targets.length}.zip`);
      setTimeout(() => URL.revokeObjectURL(objectUrl), 60_000);
      if (this.bulkFailed.length === 0) {
        this.toasts.show(`Downloaded ${targets.length} selected ${targets.length === 1 ? 'clip' : 'clips'}.`, 'success');
      } else {
        this.error = `Downloaded ${targets.length - this.bulkFailed.length} of ${targets.length}. Failed: ${this.bulkFailed.join(', ')}.`;
        this.toasts.show('Some clips failed to download. See details above.', 'error');
      }
    } catch {
      this.bulkFailed = targets.map((c) => `clip #${c.rank}`);
      this.error = 'Selected download failed. Please try clips individually.';
    } finally {
      this.bulkActive = false;
    }
  }

  async downloadAll(): Promise<void> {
    if (this.zipping) return;
    this.zipping = true;
    try {
      const zip = new JSZip();
      const urls = await Promise.all(this.clips.map((c) => this.clipUrl(c)));
      const blobs = await Promise.all(
        urls.map(async (url, i) => {
          const res = await fetch(url);
          if (!res.ok) throw new Error(`fetch ${i}`);
          return res.blob();
        }),
      );
      blobs.forEach((blob, i) => {
        zip.file(`short-${this.clips[i].rank}.mp4`, blob);
      });
      const content = await zip.generateAsync({ type: 'blob' });
      const objectUrl = URL.createObjectURL(content);
      this.triggerDownload(objectUrl, 'shorts.zip');
      setTimeout(() => URL.revokeObjectURL(objectUrl), 60_000);
      this.toasts.show(`Downloaded ${this.clips.length} clips as ZIP.`, 'success');
    } catch {
      this.error = 'ZIP download failed. Please try clips individually.';
    } finally {
      this.zipping = false;
    }
  }

  async share(clip: Clip): Promise<void> {
    const text = this.captionText(clip);
    try {
      const url = await this.clipUrl(clip);
      if (navigator.share) {
        await navigator.share({ title: `Short #${clip.rank}`, text, url });
        return;
      }
      throw new Error('no-share');
    } catch (err) {
      // A dismissed share sheet rejects with AbortError: the user declined,
      // so don't touch the clipboard. Fall back to copy only when the
      // native share API is missing or really failed.
      if (err instanceof DOMException && err.name === 'AbortError') {
        return;
      }
      const ok = await this.copyText(text);
      this.error = ok ? '' : 'Share failed. Please try again.';
      if (ok) {
        this.copied[clip.clipId] = true;
        setTimeout(() => { this.copied[clip.clipId] = false; }, 2000);
      }
    }
  }

  formatDuration(seconds: number): string {
    const total = Math.round(seconds);
    const mins = Math.floor(total / 60);
    const secs = total % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  }

  formatTime(seconds: number): string {
    if (!isFinite(seconds) || seconds < 0) return '0:00';
    const total = Math.floor(seconds);
    const mins = Math.floor(total / 60);
    const secs = total % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  }

  goToThumbnails(): void {
    this.router.navigate(['/shorts', this.jobId, 'thumbnails']);
  }

  goToUpload(): void {
    this.router.navigate(['/shorts']);
  }

  private captionText(clip: Clip): string {
    const hook = (clip.hookText || '').trim();
    const base = hook || `Short #${clip.rank}`;
    return `${base}\n#shorts #viral #fyp`;
  }

  private clipUrl(clip: Clip): Promise<string> {
    if (this.previewUrls[clip.clipId]) {
      return Promise.resolve(this.previewUrls[clip.clipId]);
    }
    return new Promise((resolve, reject) => {
      this.shorts.getClipUrl(this.jobId, clip.clipId).subscribe({
        next: (res) => {
          if (!res.url) {
            this.error = res.message || 'This clip is in cold storage and is being restored. Please check back in a few hours.';
            reject(new Error('restoring'));
            return;
          }
          this.previewUrls[clip.clipId] = res.url;
          resolve(res.url);
        },
        error: (err) => reject(err),
      });
    });
  }

  private withClipUrl(clip: Clip, onUrl: (url: string) => void, onError: () => void): void {
    this.clipUrl(clip).then(onUrl).catch(onError);
  }

  private async copyText(text: string): Promise<boolean> {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Fallback for non-secure contexts / older browsers.
      try {
        const area = document.createElement('textarea');
        area.value = text;
        area.style.position = 'fixed';
        area.style.opacity = '0';
        document.body.appendChild(area);
        area.select();
        const ok = document.execCommand('copy');
        document.body.removeChild(area);
        return ok;
      } catch {
        return false;
      }
    }
  }

  private triggerDownload(url: string, filename: string): void {
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = filename;
    anchor.rel = 'noopener';
    document.body.appendChild(anchor);
    anchor.click();
    document.body.removeChild(anchor);
  }

  private clearHoverTimer(clipId: string): void {
    const timer = this.hoverTimers[clipId];
    if (timer) {
      clearTimeout(timer);
      delete this.hoverTimers[clipId];
    }
  }

  ngOnDestroy(): void {
    Object.keys(this.hoverTimers).forEach((id) => this.clearHoverTimer(id));
  }

  // ---------------------------------------------------------- caption studio

  toggleStylePanel(): void {
    this.styleOpen = !this.styleOpen;
    if (this.styleOpen && !this.styleLoaded) {
      this.loadStyle();
    }
  }

  toggleEffectsPanel(): void {
    this.effectsOpen = !this.effectsOpen;
  }

  selectPreset(preset: NonNullable<CaptionStyle['preset']>): void {
    this.styleDraft = { ...this.styleDraft, preset };
  }

  resetStyleDraft(): void {
    this.styleDraft = { ...DEFAULT_CAPTION_STYLE };
  }

  saveStyle(): void {
    if (this.styleSaving || !this.jobId) return;
    if (this.styleLoaded && !this.styleEditable) return;
    this.styleSaving = true;
    this.styleError = '';
    this.shorts.updateCaption(this.jobId, { ...this.styleDraft }).subscribe({
      next: () => {
        this.styleSaving = false;
        this.toasts.show('Caption style saved. Applies to future renders and retries.', 'success');
      },
      error: (err) => {
        this.styleSaving = false;
        this.styleError = err?.error?.message || 'Could not save caption style.';
      },
    });
  }

  private loadStyle(): void {
    if (!this.jobId) return;
    this.styleLoadError = false;
    this.shorts.getStatus(this.jobId).subscribe({
      next: (job) => {
        this.styleDraft = { ...DEFAULT_CAPTION_STYLE, ...(job.caption ?? {}) };
        this.styleJobStatus = job.status ?? '';
        this.styleLoaded = true;
      },
      error: () => {
        this.styleDraft = { ...DEFAULT_CAPTION_STYLE };
        this.styleJobStatus = '';
        this.styleLoadError = true;
        this.styleLoaded = true;
      },
    });
  }

  /** Styles apply to future renders: completed/in-flight jobs are read-only. */
  get styleEditable(): boolean {
    return this.styleJobStatus === 'uploaded' || this.styleJobStatus === 'failed';
  }

  /** HTML approximation of a preset for the studio preview cards. */
  swatchStyle(presetId: string): Record<string, string> {
    const px = Math.round(10 + this.styleDraft.fontSize * 0.35);
    const color = this.styleDraft.color || '#FFFFFF';
    switch (presetId) {
      case 'minimal':
        return { color, 'font-size': `${Math.max(11, px - 3)}px`, 'text-shadow': '0 1px 3px rgba(0,0,0,0.8)' };
      case 'karaoke':
        return { color: this.styleDraft.highlight || '#FFD60A', 'font-size': `${px + 3}px`, 'font-weight': '800', 'text-shadow': '0 2px 4px rgba(0,0,0,0.9)' };
      case 'clean':
        return { color, 'font-size': `${px}px`, 'text-shadow': '-1px 0 0 #000, 1px 0 0 #000, 0 -1px 0 #000, 0 1px 0 #000' };
      default:
        return { color, 'font-size': `${px + 3}px`, 'font-weight': '800', background: 'rgba(0,0,0,0.55)', padding: '2px 8px', 'border-radius': '6px' };
    }
  }

  confirmDelete(): void {
    if (!this.jobId || this.deleting) return;
    this.deleting = true;
    this.shorts.deleteJob(this.jobId).subscribe({
      next: () => {
        this.toasts.show('Short deleted successfully.', 'success');
        this.router.navigate(['/shorts/history']);
      },
      error: () => {
        this.deleting = false;
        this.pendingDelete = false;
        this.toasts.show('Could not delete this short. Please try again.', 'error');
      },
    });
  }
}
