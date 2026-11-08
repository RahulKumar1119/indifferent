import { Component, ElementRef, ViewChild, AfterViewInit, OnInit, OnDestroy, inject } from '@angular/core';
import { CommonModule, DOCUMENT } from '@angular/common';
import { LucideDynamicIcon } from '@lucide/angular';
import { PublicHeaderComponent } from '../../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../../shared/components/site-footer/site-footer.component';
import { FormsModule } from '@angular/forms';
import gsap from 'gsap';
import { setJsonLd, setPageSeo } from '../../../shared/seo';
import { loadVerifiedCore } from './ffmpeg-core';
import { WatermarkAboutComponent, WatermarkHeroComponent } from './watermark-sections.component';

// FFmpeg.wasm single-thread core (no COOP/COEP headers needed). Fetched lazily
// on first video export, never on page load, and hash-checked first
// (see ffmpeg-core.ts).
// The -st core exports _main (cwrap name 'main'), not the default proxy_main.
const FF_MAIN_NAME = 'main';
// Client-side processing gate: larger files need ~2GB free RAM.
const MAX_CLIENT_BYTES = 500 * 1024 * 1024;

type WatermarkKind = 'text' | 'logo';
type AspectOption = 'original' | '16:9' | '9:16' | '1:1';

interface WmFont {
  label: string;
  stack: string;
}

const FONTS: WmFont[] = [
  { label: 'Arial', stack: 'Arial, Helvetica, sans-serif' },
  { label: 'Georgia', stack: 'Georgia, serif' },
  { label: 'Times New Roman', stack: '"Times New Roman", Times, serif' },
  { label: 'Courier New', stack: '"Courier New", Courier, monospace' },
  { label: 'Verdana', stack: 'Verdana, Geneva, sans-serif' },
  { label: 'Trebuchet MS', stack: '"Trebuchet MS", Verdana, sans-serif' },
  { label: 'Impact', stack: 'Impact, Haettenschweiler, sans-serif' },
  { label: 'Comic Sans MS', stack: '"Comic Sans MS", cursive' },
  { label: 'Palatino', stack: '"Palatino Linotype", Palatino, serif' },
  { label: 'Lucida Console', stack: '"Lucida Console", Monaco, monospace' },
];

const COLOR_PRESETS = ['#ffffff', '#000000', '#ff3b30', '#ffcc00', '#00c7ff', '#ff2fd6'];

// 3x3 snap positions as fractions of the frame.
const GRID_SPOTS = [0.15, 0.5, 0.85];

// Minimal surface of @ffmpeg/ffmpeg 0.11 used by the video export.
interface FfmpegFactory {
  createFFmpeg(options: { corePath: string; wasmPath?: string; workerPath?: string; mainName?: string; log?: boolean }): FfmpegEngine;
  fetchFile(input: File | Blob | string): Promise<Uint8Array>;
}

interface FfmpegEngine {
  isLoaded(): boolean;
  load(): Promise<void>;
  run(...args: string[]): Promise<void>;
  FS(op: 'writeFile', name: string, data: Uint8Array): void;
  FS(op: 'readFile', name: string): Uint8Array;
  FS(op: 'unlink', name: string): void;
  setProgress(cb: (progress: { ratio: number }) => void): void;
}

@Component({
  selector: 'app-watermark',
  standalone: true,
  imports: [CommonModule, FormsModule, LucideDynamicIcon, PublicHeaderComponent, SiteFooterComponent, WatermarkHeroComponent, WatermarkAboutComponent],
  styles: [`
    :host { display: block; }
    .page { --page: var(--color-paper-2); --card: var(--color-paper); background: var(--page); color: var(--color-ink); font-family: var(--font-body); min-height: 100dvh; -webkit-font-smoothing: antialiased; }
    .wrap { width: min(80rem, 100%); margin-inline: auto; padding-inline: var(--space-md); }
    @media (min-width: 48rem) { .wrap { padding-inline: var(--space-xl); } }
    .stage { margin-top: calc(-1 * var(--space-2xl)); position: relative; padding-bottom: var(--space-2xl); }
    .card { background: var(--card); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); box-shadow: 0 1px 2px color-mix(in oklab, var(--color-ink) 5%, transparent), 0 12px 32px -18px color-mix(in oklab, var(--color-ink) 28%, transparent); }
    .drop { display: flex; flex-direction: column; align-items: center; text-align: center; gap: var(--space-sm); padding: var(--space-2xl) var(--space-lg); border: 2px dashed color-mix(in oklab, var(--color-ink) 22%, transparent); border-radius: calc(var(--radius-surface) - 4px); margin: var(--space-xs); cursor: pointer; transition: border-color var(--dur-short) var(--ease-out), background-color var(--dur-short) var(--ease-out); }
    .drop:hover, .drop.over { border-color: var(--color-accent); background: color-mix(in oklab, var(--color-accent) 6%, var(--card)); }
    @media (max-width: 40rem), (max-height: 30rem) { .drop { padding: var(--space-lg) var(--space-md); } }
    .drop h2 { margin: 0; font-size: var(--type-xl); font-weight: 600; }
    .drop-icon { width: 3.5rem; height: 3.5rem; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; background: color-mix(in oklab, var(--color-accent) 12%, var(--card)); color: var(--color-accent-text); }
    .muted { color: var(--color-muted); font-size: var(--type-sm); margin: 0; }
    .local { display: flex; align-items: flex-start; gap: var(--space-xs); padding: var(--space-sm) var(--space-md); border-radius: var(--radius-control); background: color-mix(in oklab, var(--color-positive) 9%, var(--card)); color: var(--color-ink); font-size: var(--type-sm); line-height: 1.45; text-align: left; }
    .local svg { flex: none; margin-top: 2px; color: var(--color-positive); }
    .err { color: var(--color-negative); font-size: var(--type-sm); font-weight: 500; margin: 0; }
    .editor { display: grid; gap: var(--space-md); align-items: start; }
    @media (min-width: 64rem) { .editor { grid-template-columns: 22rem minmax(0, 1fr); gap: var(--space-lg); } .preview { order: 2; } .controls { position: sticky; top: calc(56px + var(--space-xl)); } }
    .preview { padding: var(--space-sm); min-width: 0; }
    @media (min-width: 40rem) { .preview { padding: var(--space-md); } }
    .bar { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-xs); margin-bottom: var(--space-sm); }
    .seg { display: inline-flex; flex-wrap: wrap; gap: var(--space-2xs); }
    .chipbtn { min-height: 44px; padding: 0 var(--space-sm); border-radius: var(--radius-pill); border: 1px solid var(--color-rule); background: transparent; color: var(--color-ink-2); font: 600 var(--type-xs)/1 var(--font-body); cursor: pointer; display: inline-flex; align-items: center; gap: var(--space-2xs); transition: background-color var(--dur-short) var(--ease-out), color var(--dur-short) var(--ease-out); }
    .chipbtn:hover { background: var(--color-paper-3); color: var(--color-ink); }
    .chipbtn[aria-pressed='true'] { background: var(--color-ink); border-color: var(--color-ink); color: var(--color-paper); }
    .chipbtn.ba[aria-pressed='true'] { background: var(--color-accent); border-color: var(--color-accent); color: var(--color-accent-ink); }
    .canvas-box { position: relative; display: flex; align-items: center; justify-content: center; overflow: hidden; min-height: 280px; border-radius: var(--radius-control); background: var(--color-paper-3); cursor: crosshair; touch-action: none; }
    canvas { max-width: 100%; max-height: 62vh; border-radius: 6px; }
    .tag { position: absolute; bottom: var(--space-sm); left: 50%; transform: translateX(-50%); pointer-events: none; white-space: nowrap; padding: var(--space-3xs) var(--space-sm); border-radius: var(--radius-pill); background: var(--card); color: var(--color-ink-2); font-size: var(--type-xs); border: 1px solid var(--color-rule); }
    .tag.ba { background: var(--color-accent); color: var(--color-accent-ink); border-color: var(--color-accent); }
    .player { margin-top: var(--space-sm); display: flex; align-items: center; gap: var(--space-sm); }
    input[type='range'] { accent-color: var(--color-accent); min-height: 44px; flex: 1; min-width: 0; }
    .controls { padding: var(--space-lg); display: flex; flex-direction: column; gap: var(--space-md); }
    .ctl-head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-sm); }
    .ctl-head h2 { margin: 0; font-size: var(--type-lg); font-weight: 600; }
    .field { display: flex; flex-direction: column; gap: var(--space-2xs); }
    .row { display: flex; align-items: center; gap: var(--space-sm); }
    .val { min-width: 3rem; text-align: right; font-size: var(--type-xs); font-weight: 600; color: var(--color-accent-text); font-variant-numeric: lining-nums tabular-nums; }
    .two { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-xs); }
    .opt { min-height: 44px; border-radius: var(--radius-control); border: 1px solid var(--color-rule); background: transparent; color: var(--color-ink); font: 600 var(--type-sm)/1 var(--font-body); cursor: pointer; }
    .opt:hover { border-color: var(--color-ink-2); }
    .opt[aria-pressed='true'] { background: var(--color-ink); border-color: var(--color-ink); color: var(--color-paper); }
    .menu { position: absolute; z-index: var(--z-dropdown); margin-top: var(--space-2xs); width: 100%; max-height: 14rem; overflow-y: auto; padding: var(--space-2xs) 0; list-style: none; border-radius: var(--radius-control); border: 1px solid var(--color-rule); background: var(--card); box-shadow: 0 12px 32px -12px color-mix(in oklab, var(--color-ink) 35%, transparent); }
    .menu button { width: 100%; min-height: 44px; padding: 0 var(--space-md); text-align: left; border: 0; background: transparent; color: var(--color-ink); font-size: var(--type-md); cursor: pointer; }
    .menu button:hover, .menu button[aria-selected='true'] { background: var(--color-paper-3); }
    .swatches { display: flex; flex-wrap: wrap; gap: var(--space-2xs); }
    .sw { width: 44px; height: 44px; border-radius: var(--radius-control); border: 2px solid var(--color-rule); cursor: pointer; padding: 0; }
    .sw[aria-pressed='true'] { border-color: var(--color-accent); box-shadow: 0 0 0 2px var(--card) inset; }
    input[type='color'].sw { padding: 3px; background: transparent; }
    .spots { display: grid; grid-template-columns: repeat(3, 44px); gap: var(--space-2xs); }
    .spot { width: 44px; height: 44px; border-radius: var(--radius-control); border: 1px solid var(--color-rule); background: transparent; cursor: pointer; }
    .spot:hover { border-color: var(--color-ink-2); }
    .spot[aria-pressed='true'] { background: var(--color-accent); border-color: var(--color-accent); }
    .gate { padding: var(--space-md); border-radius: var(--radius-control); border: 1px solid color-mix(in oklab, var(--color-caution) 45%, transparent); background: color-mix(in oklab, var(--color-caution) 8%, var(--card)); display: flex; flex-direction: column; gap: var(--space-xs); font-size: var(--type-sm); }
    .gate p { margin: 0; }
    .export { padding-top: var(--space-md); border-top: 1px solid var(--color-rule); display: flex; flex-direction: column; gap: var(--space-sm); }
    .export .ui-btn { width: 100%; }
    .big { min-height: 52px; font-size: var(--type-md); }
    .fixed { min-height: 44px; display: flex; align-items: center; padding: 0 var(--space-md); border-radius: var(--radius-control); border: 1px solid var(--color-rule); font-size: var(--type-sm); font-weight: 600; }
    .spin { width: 1rem; height: 1rem; border-radius: 50%; border: 2px solid color-mix(in oklab, var(--color-accent-ink) 40%, transparent); border-top-color: var(--color-accent-ink); animation: spin 0.8s linear infinite; }
    @keyframes spin { to { transform: rotate(360deg); } }
    .track { height: 8px; border-radius: var(--radius-pill); background: var(--color-paper-3); overflow: hidden; }
    .track div { height: 100%; background: var(--color-accent); border-radius: inherit; }
    .share { margin-top: var(--space-md); padding: var(--space-md); border-radius: var(--radius-control); background: var(--color-paper-3); }
    .share h3 { margin: 0; font-size: var(--type-md); font-weight: 600; }
    .share .seg { margin-top: var(--space-sm); }
    @media (prefers-reduced-motion: reduce) { .spin { animation: none; } .drop, .chipbtn { transition: none; } }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />
      <app-watermark-hero />

      <main id="main" class="stage">
        <div class="wrap">
          @if (!file) {
            <div class="card" data-testid="upload">
              <div
                class="drop"
                [class.over]="isDragOver"
                (click)="fileInput.click()"
                (dragover)="onDragOver($event)"
                (dragleave)="onDragLeave($event)"
                (drop)="onDrop($event)"
              >
                <span class="drop-icon" aria-hidden="true"><svg lucideIcon="upload" [size]="24"></svg></span>
                <h2>Drop an image or video here</h2>
                <p class="muted">JPG, PNG or WebP images, MP4, WebM or MOV video.</p>
                <button type="button" class="ui-btn ui-btn--primary" (click)="$event.stopPropagation(); fileInput.click()">Choose a file</button>
                <p class="local" data-testid="local-note">
                  <svg lucideIcon="shield-check" [size]="18" aria-hidden="true"></svg>
                  <span>Your file is processed in this browser tab and is never uploaded. Video export downloads a processing engine of about 30&nbsp;MB the first time.</span>
                </p>
                @if (fileError) {
                  <p class="err" role="alert">{{ fileError }}</p>
                }
              </div>
              <input
                #fileInput
                type="file"
                accept="image/jpeg,image/png,image/webp,video/mp4,video/webm,video/quicktime"
                (change)="onFileSelected($event)"
                aria-label="Choose an image or video to watermark"
                hidden
              />
            </div>
          }

          @if (file) {
            <div class="editor">
              <section class="card preview" aria-label="Preview">
                <div class="bar">
                  <div class="seg" role="group" aria-label="Aspect ratio">
                    @for (a of aspectOptions; track a) {
                      <button type="button" class="chipbtn" (click)="toggleAspect(a)" [attr.aria-pressed]="aspect === a">{{ a === 'original' ? 'Original' : a }}</button>
                    }
                  </div>
                  <button type="button" class="chipbtn ba" (click)="showOriginal = !showOriginal; updatePreview()" [attr.aria-pressed]="showOriginal">
                    <svg lucideIcon="rotate-ccw" [size]="14" aria-hidden="true"></svg>
                    {{ showOriginal ? 'Showing before' : 'Before / After' }}
                  </button>
                </div>
                <div
                  #fxLayer
                  class="canvas-box"
                  (pointerdown)="onPointerDown($event)"
                  (pointermove)="onPointerMove($event)"
                  (pointerup)="onPointerUp()"
                  (pointercancel)="onPointerUp()"
                  (pointerleave)="onPointerUp()"
                >
                  <canvas #previewCanvas [attr.aria-label]="showOriginal ? 'Original file, before the watermark' : 'Preview with the watermark'" role="img"></canvas>
                  @if (fileType === 'video') {
                    <video #videoEl hidden [src]="fileUrl" muted playsInline preload="auto" (loadedmetadata)="onVideoLoaded()" (seeked)="renderPreview()" (ended)="onVideoEnded()" (error)="onVideoError()"></video>
                  }
                  <span class="tag" [class.ba]="showOriginal">{{ showOriginal ? 'Before' : 'After: drag to move the watermark' }}</span>
                </div>

                @if (fileType === 'video') {
                  <div class="player">
                    <button type="button" class="ui-btn ui-btn--secondary" style="padding: 0; width: 44px; flex: none" (click)="togglePlay()" [attr.aria-label]="playing ? 'Pause video' : 'Play video'">
                      <svg [lucideIcon]="playing ? 'pause' : 'play'" [size]="18" aria-hidden="true"></svg>
                    </button>
                    <input type="range" min="0" [max]="videoDuration || 0" step="0.1" [ngModel]="videoTime" (ngModelChange)="scrubTo($event)" aria-label="Video timeline scrubber" />
                    <span class="ui-hint" style="flex: none; font-variant-numeric: tabular-nums">{{ formatTime(videoTime) }} / {{ formatTime(videoDuration) }}</span>
                  </div>
                }

                @if (showShare) {
                  <div class="share" data-testid="share">
                    <h3>Share this tool</h3>
                    <p class="muted">Your file stayed on your device, so there is nothing to link to. Send the tool instead.</p>
                    <div class="seg">
                      <a [href]="xShareHref" target="_blank" rel="noopener" class="chipbtn">Share on X</a>
                      <a [href]="facebookShareHref" target="_blank" rel="noopener" class="chipbtn">Facebook</a>
                      <a [href]="whatsappShareHref" target="_blank" rel="noopener" class="chipbtn">WhatsApp</a>
                      <button type="button" class="chipbtn" (click)="copyShareLink()">{{ copiedLink ? 'Link copied' : 'Copy tool link' }}</button>
                    </div>
                  </div>
                }
              </section>

              <section class="card controls" aria-labelledby="wm-settings">
                <div class="ctl-head">
                  <h2 id="wm-settings">Watermark</h2>
                  <button type="button" class="ui-btn ui-btn--quiet" (click)="resetFile()">New file</button>
                </div>

                <div class="field">
                  <span class="ui-label" id="wm-type-label">Type</span>
                  <div class="two" role="group" aria-labelledby="wm-type-label">
                    <button type="button" class="opt" (click)="setKind('text')" [attr.aria-pressed]="wmKind === 'text'">Text</button>
                    <button type="button" class="opt" (click)="setKind('logo')" [attr.aria-pressed]="wmKind === 'logo'">Logo</button>
                  </div>
                </div>

                @if (wmKind === 'text') {
                  <div class="field">
                    <label class="ui-label" for="wm-text">Watermark text</label>
                    <input id="wm-text" type="text" class="ui-field" [(ngModel)]="watermarkText" (ngModelChange)="updatePreview()" maxlength="60" />
                  </div>
                  <div class="field" style="position: relative">
                    <span class="ui-label" id="wm-font-label">Font</span>
                    <button type="button" id="wm-font" class="ui-field" style="display: flex; align-items: center; justify-content: space-between; text-align: left; cursor: pointer" (click)="fontOpen = !fontOpen" [attr.aria-expanded]="fontOpen" aria-labelledby="wm-font-label wm-font" [style.font-family]="fontStack">
                      <span>{{ fontLabel }}</span><svg lucideIcon="chevron-right" [size]="16" aria-hidden="true" style="transform: rotate(90deg)"></svg>
                    </button>
                    @if (fontOpen) {
                      <ul class="menu" role="listbox" aria-label="Font choices" style="top: 100%">
                        @for (f of fonts; track f.label) {
                          <li>
                            <button type="button" role="option" [attr.aria-selected]="fontStack === f.stack" (click)="fontStack = f.stack; fontLabel = f.label; fontOpen = false; updatePreview()" [style.font-family]="f.stack">{{ f.label }}</button>
                          </li>
                        }
                      </ul>
                    }
                  </div>
                } @else {
                  <div class="field">
                    <span class="ui-label">Logo image</span>
                    <button type="button" class="ui-btn ui-btn--secondary" (click)="logoInput.click()">
                      <svg lucideIcon="upload" [size]="16" aria-hidden="true"></svg>
                      {{ logoUrl ? 'Change logo' : 'Upload a logo' }}
                    </button>
                    <span class="ui-hint">PNG, JPG or WebP. A PNG with transparency works best.</span>
                    <input #logoInput type="file" accept="image/png,image/jpeg,image/webp" (change)="onLogoSelected($event)" aria-label="Choose a logo image" hidden />
                  </div>
                }

                <div class="field">
                  <label class="ui-label" for="wm-opacity">Opacity</label>
                  <div class="row"><input id="wm-opacity" type="range" min="0" max="100" step="1" [(ngModel)]="opacityPct" (ngModelChange)="updatePreview()" /><span class="val">{{ opacityPct }}%</span></div>
                </div>
                <div class="field">
                  <label class="ui-label" for="wm-size">Size, as a share of the frame width</label>
                  <div class="row"><input id="wm-size" type="range" min="10" max="50" step="1" [(ngModel)]="sizePct" (ngModelChange)="updatePreview()" /><span class="val">{{ sizePct }}%</span></div>
                </div>
                <div class="field">
                  <span class="ui-label" id="wm-color-label">Colour</span>
                  <div class="swatches" role="group" aria-labelledby="wm-color-label">
                    <input id="wm-color" type="color" class="sw" [(ngModel)]="color" (ngModelChange)="updatePreview()" aria-label="Custom watermark colour" />
                    @for (preset of colorPresets; track preset) {
                      <button type="button" class="sw" (click)="color = preset; updatePreview()" [attr.aria-label]="'Use colour ' + preset" [attr.aria-pressed]="color === preset" [style.background]="preset"></button>
                    }
                  </div>
                </div>
                <div class="field">
                  <label class="ui-label" for="wm-rotation">Rotation</label>
                  <div class="row"><input id="wm-rotation" type="range" min="-180" max="180" step="1" [(ngModel)]="rotation" (ngModelChange)="updatePreview()" /><span class="val">{{ rotation }}&deg;</span></div>
                </div>
                <div class="field">
                  <span class="ui-label" id="wm-pos-label">Position</span>
                  <div class="spots" role="group" aria-labelledby="wm-pos-label">
                    @for (row of gridRows; track row; let ri = $index) {
                      @for (col of row; track col; let ci = $index) {
                        <button type="button" class="spot" (click)="snapToSpot(ri, ci)" [attr.aria-label]="'Move watermark to ' + spotName(ri, ci)" [attr.aria-pressed]="isActiveSpot(ri, ci)"></button>
                      }
                    }
                  </div>
                  <span class="ui-hint">You can also drag it on the preview.</span>
                </div>

                @if (sizeGate && !sizeGateDismissed) {
                  <div class="gate" role="alert">
                    <p><strong>Large file (over 500&nbsp;MB)</strong></p>
                    <p>Rendering happens on your own machine, so it needs roughly 2&nbsp;GB of free memory and a browser tab that stays open until it finishes. You can try anyway, or pick a smaller file.</p>
                    <button type="button" class="ui-btn ui-btn--secondary" (click)="dismissGate()">Process anyway</button>
                    <button type="button" class="ui-btn ui-btn--quiet" (click)="reset()">Choose a smaller file</button>
                  </div>
                }
                @if (fileError) {
                  <p class="err" role="alert">{{ fileError }}</p>
                }
                @if (exportError) {
                  <p class="err" role="alert">{{ exportError }}</p>
                }

                <div class="export">
                  @if (fileType === 'image') {
                    <div class="field">
                      <span class="ui-label" id="wm-format-label">Format</span>
                      <div class="two" role="group" aria-labelledby="wm-format-label">
                        <button type="button" class="opt" (click)="setImgFormat('png')" [attr.aria-pressed]="imgFormat === 'png'">PNG</button>
                        <button type="button" class="opt" (click)="setImgFormat('jpeg')" [attr.aria-pressed]="imgFormat === 'jpeg'">JPEG</button>
                      </div>
                      <span class="ui-hint">JPEG flattens transparency onto white. Images export at full resolution.</span>
                    </div>
                    <button type="button" class="ui-btn ui-btn--primary big" (click)="download()" [disabled]="!downloadUrl || exportBusy()">
                      <svg [lucideIcon]="exportState === 'done' ? 'check' : 'download'" [size]="18" aria-hidden="true"></svg>
                      {{ exportState === 'done' ? 'Downloaded' : 'Download ' + (imgFormat === 'jpeg' ? 'JPEG' : 'PNG') }}
                    </button>
                  } @else {
                    <div class="field">
                      <span class="ui-label">Format</span>
                      <div class="fixed">MP4 with H.264 video and AAC audio</div>
                      <span class="ui-hint">Video always exports as MP4, whatever you uploaded.</span>
                    </div>
                    <button type="button" class="ui-btn ui-btn--primary big" (click)="exportVideo()" [disabled]="!videoReady || exportBusy() || (sizeGate && !sizeGateDismissed)">
                      @if (exportState === 'loading') {
                        <span class="spin" aria-hidden="true"></span>Loading engine
                      } @else if (exportState === 'rendering') {
                        <span class="spin" aria-hidden="true"></span>Rendering{{ showProgressBar ? ' ' + exportProgress + '%' : '' }}
                      } @else if (exportState === 'done') {
                        <svg lucideIcon="check" [size]="18" aria-hidden="true"></svg>Downloaded
                      } @else {
                        <svg lucideIcon="download" [size]="18" aria-hidden="true"></svg>Export watermarked video
                      }
                    </button>
                    @if (showProgressBar && exportState === 'rendering') {
                      <div class="track" role="progressbar" [attr.aria-valuenow]="exportProgress" aria-valuemin="0" aria-valuemax="100" aria-label="Render progress">
                        <div [style.width.%]="exportProgress"></div>
                      </div>
                    }
                    @if (!engineReady) {
                      <span class="ui-hint">The first export downloads a processing engine (about 30&nbsp;MB) and caches it for later exports.</span>
                    }
                  }
                  <p class="local" data-testid="local-note">
                    <svg lucideIcon="shield-check" [size]="18" aria-hidden="true"></svg>
                    <span>Processed in your browser. Nothing is uploaded.</span>
                  </p>
                  <button type="button" class="ui-btn ui-btn--secondary" (click)="reset()">Reset</button>
                </div>
              </section>
            </div>
          }
        </div>
      </main>

      <app-watermark-about [faqs]="faqs" />
      <app-site-footer />
    </div>
  `,
})
export class WatermarkComponent implements OnInit, AfterViewInit, OnDestroy {
  private readonly document = inject(DOCUMENT);

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Watermark Images and Videos Free | Indifferent',
      description:
        'Add text or logo watermarks to JPG, PNG, WebP, MP4, WebM and MOV files. Free, private, processed in your browser. Nothing uploads anywhere.',
      canonical: 'https://indifferent.fun/tools/add-watermark',
    });
    this.addJsonLd({
      '@context': 'https://schema.org',
      '@type': 'SoftwareApplication',
      name: 'Indifferent Watermark Tool',
      url: 'https://indifferent.fun/tools/add-watermark',
      applicationCategory: 'MultimediaApplication',
      operatingSystem: 'Web',
      description:
        'Add text or logo watermarks to images and video in your browser. Nothing is uploaded.',
      offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD', description: 'Free, no account' },
      featureList: [
        'Text and logo watermarks',
        'Image and video support',
        'Runs entirely in the browser',
        'PNG or JPEG export at the original size',
      ],
    });
    this.addJsonLd({
      '@context': 'https://schema.org',
      '@type': 'FAQPage',
      mainEntity: this.faqs.map((f) => ({
        '@type': 'Question',
        name: f.q,
        acceptedAnswer: { '@type': 'Answer', text: f.a },
      })),
    });
  }

  private addJsonLd(schema: Record<string, unknown>): void {
    // One block per schema type, so revisiting the page never stacks duplicates.
    setJsonLd(this.document, String(schema['@type']).toLowerCase(), schema);
  }

  @ViewChild('previewCanvas') previewCanvas!: ElementRef<HTMLCanvasElement>;
  @ViewChild('videoEl') videoEl?: ElementRef<HTMLVideoElement>;
  @ViewChild('fxLayer') fxLayer?: ElementRef<HTMLDivElement>;

  currentYear = new Date().getFullYear();

  faqs: { q: string; a: string }[] = [
    { q: 'Does my file get uploaded?', a: 'No. Images are drawn on a canvas in your browser and videos are rendered with FFmpeg compiled to WebAssembly, running in the same tab. The file is read from your disk and never sent anywhere, so there is no copy of it on a server.' },
    { q: 'Do I need an account?', a: 'No. The tool has no login, no signup and no quota. Open the page, use it, close the tab.' },
    { q: 'Which formats can I open?', a: 'JPG, PNG and WebP for images. MP4, WebM and MOV for video. Video is always exported as MP4 with H.264 and AAC audio, whatever format you uploaded.' },
    { q: 'How large a file can I use?', a: 'Images are limited only by your device memory. Video is practical up to a few hundred megabytes, roughly the 500 MB the tool warns about, and a large file needs about 2 GB of free memory to render.' },
    { q: 'Will the video export be lossy?', a: 'The video is re-encoded to H.264 with AAC audio. That is a fresh encode, so a little quality is lost compared with the source, which is normal for any re-encode.' },
    { q: 'Which browsers does it work in?', a: 'Any current Chrome, Edge, Firefox or Safari. The video path needs WebAssembly, which all of them support.' },
    { q: 'Can I use a logo instead of text?', a: 'Yes. Upload a PNG with transparency for the cleanest result. JPG and WebP work too, but any background they carry will be baked into the mark.' },
    { q: 'Why does the first video export take longer?', a: 'It downloads a processing engine of about 30 MB the first time, then caches it in your browser. Later exports start straight away.' },
  ];

  file: File | null = null;
  fileType: 'image' | 'video' | null = null;
  fileUrl = '';
  fileError = '';

  // Watermark settings
  wmKind: WatermarkKind = 'text';
  watermarkText = 'Your Watermark';
  fonts = FONTS;
  fontStack = FONTS[0].stack;
  fontLabel = FONTS[0].label;
  fontOpen = false;
  colorPresets = COLOR_PRESETS;
  color = '#ffffff';
  opacityPct = 50;
  sizePct = 25;
  rotation = 0;

  // Position as fractions of the frame; tweened by the grid, set by drag.
  pos = { x: 0.75, y: 0.85 };
  private dragging = false;
  gridRows = [[0, 1, 2], [0, 1, 2], [0, 1, 2]];
  aspectOptions: AspectOption[] = ['original', '16:9', '9:16', '1:1'];
  aspect: AspectOption = 'original';
  showOriginal = false;

  // Logo mode
  logoUrl = '';
  private logoImg: HTMLImageElement | null = null;

  // Cached base image (avoids re-decoding on every keystroke)
  private baseImg: HTMLImageElement | null = null;
  private baseW = 0;
  private baseH = 0;
  // Monotonic render generation: async toBlob callbacks from older renders
  // are ignored so fast typing never serves a stale watermark.
  private renderSeq = 0;
  private uiTimers: number[] = [];

  // Video state
  videoDuration = 0;
  videoTime = 0;
  playing = false;
  videoReady = false;
  private rafId = 0;

  // Export state machine: idle → loading → rendering → done | error.
  exportState: 'idle' | 'loading' | 'rendering' | 'done' | 'error' = 'idle';
  exportProgress = 0;
  showProgressBar = false;
  exportError = '';
  private exportStartedAt = 0;
  private progressTimer: number | null = null;
  private engine: FfmpegEngine | null = null;
  private engineLoading: Promise<void> | null = null;
  /** True once the processing engine has been fetched and cached. */
  engineReady = false;
  private ffApi: FfmpegFactory | null = null;

  // Image export format + large-file gate + share panel.
  imgFormat: 'png' | 'jpeg' = 'png';
  sizeGate = false;
  sizeGateDismissed = false;
  showShare = false;
  copiedLink = false;
  readonly shareUrl = 'https://indifferent.fun/tools/add-watermark';
  readonly shareText = 'Watermark images and video in your browser, free and private. Indifferent Watermark Tool.';

  isDragOver = false;
  downloadUrl = '';

  ngAfterViewInit(): void {
    // Preview renders once media is loaded.
  }

  ngOnDestroy(): void {
    this.stopLoop();
    this.clearProgressTimer();
    this.clearUiTimers();
    if (this.fileUrl) URL.revokeObjectURL(this.fileUrl);
    if (this.logoUrl) URL.revokeObjectURL(this.logoUrl);
    if (this.downloadUrl) URL.revokeObjectURL(this.downloadUrl);
  }

  /** Tracked timeout: cleared on destroy so callbacks never touch a dead component. */
  private later(fn: () => void, ms: number): void {
    const id: number = window.setTimeout(() => {
      this.uiTimers = this.uiTimers.filter((t) => t !== id);
      fn();
    }, ms);
    this.uiTimers.push(id);
  }

  private clearUiTimers(): void {
    this.uiTimers.forEach((id) => clearTimeout(id));
    this.uiTimers = [];
  }

  // ------------------------------------------------------------------ files

  onDragOver(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver = true;
  }

  onDragLeave(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver = false;
  }

  onDrop(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver = false;
    const files = event.dataTransfer?.files;
    if (files && files.length > 0) this.handleFile(files[0]);
  }

  onFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    if (input.files && input.files.length > 0) this.handleFile(input.files[0]);
    input.value = '';
  }

  private handleFile(file: File): void {
    const imageTypes = ['image/jpeg', 'image/png', 'image/webp'];
    const videoTypes = ['video/mp4', 'video/webm', 'video/quicktime'];
    this.fileError = '';
    if (imageTypes.includes(file.type)) {
      this.setFile(file, 'image');
      this.loadBaseImage();
    } else if (videoTypes.includes(file.type)) {
      this.setFile(file, 'video');
      // Readiness confirmed on loadedmetadata; unsupported codecs surface
      // through the video element's error event below.
      setTimeout(() => this.updatePreview(), 100);
    } else {
      this.fileError = 'Unsupported file type. Please use JPG, PNG, WebP, MP4, WebM or MOV.';
    }
  }

  private setFile(file: File, kind: 'image' | 'video'): void {
    if (this.fileUrl) URL.revokeObjectURL(this.fileUrl);
    this.file = file;
    this.fileType = kind;
    this.fileUrl = URL.createObjectURL(file);
    this.showOriginal = false;
    this.videoReady = false;
    this.videoTime = 0;
    this.playing = false;
    this.sizeGate = file.size > MAX_CLIENT_BYTES;
    this.sizeGateDismissed = false;
    this.resetExportUi();
  }

  onLogoSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    const logo = input.files?.[0];
    input.value = '';
    if (!logo || !logo.type.startsWith('image/')) return;
    if (this.logoUrl) URL.revokeObjectURL(this.logoUrl);
    this.logoUrl = URL.createObjectURL(logo);
    const img = new Image();
    img.onload = () => {
      this.logoImg = img;
      this.updatePreview();
    };
    img.src = this.logoUrl;
  }

  setKind(kind: WatermarkKind): void {
    this.wmKind = kind;
    this.updatePreview();
  }

  // ------------------------------------------------------------------- draw

  private loadBaseImage(): void {
    const img = new Image();
    img.onload = () => {
      this.baseImg = img;
      this.baseW = img.naturalWidth;
      this.baseH = img.naturalHeight;
      this.updatePreview();
    };
    img.src = this.fileUrl;
  }

  /** Control change entry point: resets export UI, then redraws. */
  updatePreview(): void {
    this.resetExportUi();
    this.renderPreview(++this.renderSeq);
  }

  /** Pure redraw with no side effects (scrubber, playback loop, seeks). */
  renderPreview(seq: number = this.renderSeq): void {
    if (!this.file || !this.previewCanvas) return;
    const canvas = this.previewCanvas.nativeElement;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    if (this.fileType === 'image') {
      if (!this.baseImg) return;
      const scale = Math.min(1, 900 / Math.max(this.baseW, this.baseH));
      canvas.width = Math.max(1, Math.round(this.baseW * scale));
      canvas.height = Math.max(1, Math.round(this.baseH * scale));
      ctx.drawImage(this.baseImg, 0, 0, canvas.width, canvas.height);
    } else {
      const video = this.videoEl?.nativeElement;
      if (!video || !video.videoWidth) return;
      const scale = Math.min(1, 960 / Math.max(video.videoWidth, video.videoHeight));
      canvas.width = Math.max(1, Math.round(video.videoWidth * scale));
      canvas.height = Math.max(1, Math.round(video.videoHeight * scale));
      try {
        ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
      } catch {
        return;
      }
    }

    if (!this.showOriginal) this.drawWatermark(ctx, canvas.width, canvas.height);
    this.drawAspectGuide(ctx, canvas.width, canvas.height);
    if (this.fileType === 'image') this.generateDownload(seq);
  }

  private resetExportUi(): void {
    this.clearProgressTimer();
    if (this.exportState === 'done' || this.exportState === 'error') {
      this.exportState = 'idle';
    }
    this.exportError = '';
    this.showProgressBar = false;
    this.showShare = false;
  }

  private clearProgressTimer(): void {
    if (this.progressTimer) {
      clearTimeout(this.progressTimer);
      this.progressTimer = null;
    }
  }

  private drawWatermark(ctx: CanvasRenderingContext2D, w: number, h: number): void {
    if (this.wmKind === 'logo' && !this.logoImg) return;
    if (this.wmKind === 'text' && !this.watermarkText.trim()) return;
    ctx.save();
    ctx.globalAlpha = this.opacityPct / 100;
    ctx.translate(this.pos.x * w, this.pos.y * h);
    ctx.rotate((this.rotation * Math.PI) / 180);

    if (this.wmKind === 'logo' && this.logoImg) {
      const lw = (this.sizePct / 100) * w;
      const lh = lw * (this.logoImg.naturalHeight / Math.max(1, this.logoImg.naturalWidth));
      ctx.drawImage(this.logoImg, -lw / 2, -lh / 2, lw, lh);
    } else {
      const px = (this.sizePct / 100) * w;
      ctx.font = `${px}px ${this.fontStack}`;
      ctx.textBaseline = 'middle';
      ctx.textAlign = 'center';
      ctx.strokeStyle = 'rgba(0,0,0,0.5)';
      ctx.lineWidth = Math.max(1, px / 18);
      ctx.strokeText(this.watermarkText, 0, 0);
      ctx.fillStyle = this.color;
      ctx.fillText(this.watermarkText, 0, 0);
    }
    ctx.restore();
  }

  /** Dashed crop guide: largest centered rect of the chosen aspect. */
  private drawAspectGuide(ctx: CanvasRenderingContext2D, w: number, h: number): void {
    if (this.aspect === 'original') return;
    const [aw, ah] = this.aspect === '16:9' ? [16, 9] : this.aspect === '9:16' ? [9, 16] : [1, 1];
    const target = aw / ah;
    const frame = w / h;
    let gw = w;
    let gh = h;
    if (frame > target) gw = h * target;
    else gh = w / target;
    const gx = (w - gw) / 2;
    const gy = (h - gh) / 2;
    ctx.save();
    ctx.setLineDash([8, 6]);
    ctx.lineWidth = 2;
    ctx.strokeStyle = 'rgba(0,0,0,0.55)';
    ctx.strokeRect(gx, gy, gw, gh);
    ctx.strokeStyle = 'rgba(255,255,255,0.9)';
    ctx.strokeRect(gx, gy, gw, gh);
    ctx.restore();
  }

  // --------------------------------------------------------------- position

  isActiveSpot(ri: number, ci: number): boolean {
    return (
      Math.abs(this.pos.x - GRID_SPOTS[ci]) < 0.06 && Math.abs(this.pos.y - GRID_SPOTS[ri]) < 0.06
    );
  }

  spotName(ri: number, ci: number): string {
    const v = ['top', 'middle', 'bottom'][ri];
    const h = ['left', 'center', 'right'][ci];
    return `${v} ${h}`;
  }

  snapToSpot(ri: number, ci: number): void {
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const target = { x: GRID_SPOTS[ci], y: GRID_SPOTS[ri] };
    if (reduce) {
      this.pos.x = target.x;
      this.pos.y = target.y;
      this.updatePreview();
      return;
    }
    gsap.to(this.pos, {
      ...target,
      duration: 0.3,
      ease: 'power2.out',
      overwrite: true,
      onUpdate: () => this.updatePreview(),
    });
  }

  toggleAspect(a: AspectOption): void {
    this.aspect = this.aspect === a ? 'original' : a;
    this.updatePreview();
  }

  onPointerDown(event: PointerEvent): void {
    if (!this.previewCanvas) return;
    this.dragging = true;
    gsap.killTweensOf(this.pos);
    (event.target as HTMLElement).setPointerCapture?.(event.pointerId);
    this.updateDragPosition(event);
  }

  onPointerMove(event: PointerEvent): void {
    if (!this.dragging || !this.previewCanvas) return;
    this.updateDragPosition(event);
  }

  onPointerUp(): void {
    this.dragging = false;
  }

  private updateDragPosition(event: PointerEvent): void {
    const canvas = this.previewCanvas.nativeElement;
    const rect = canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;
    this.pos.x = Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width));
    this.pos.y = Math.min(1, Math.max(0, (event.clientY - rect.top) / rect.height));
    this.updatePreview();
  }

  // ------------------------------------------------------------------ video

  onVideoLoaded(): void {
    const video = this.videoEl?.nativeElement;
    if (!video) return;
    if (!video.videoWidth) {
      this.fileError = "This video can't be previewed in your browser. Try MP4 or WebM.";
      return;
    }
    this.videoDuration = video.duration || 0;
    this.videoReady = true;
    this.renderPreview();
  }

  onVideoError(): void {
    // Unsupported codec/container: loadedmetadata never fires, so surface
    // the dead end instead of leaving Export disabled with no feedback.
    this.videoReady = false;
    this.playing = false;
    this.fileError = "This video can't be played in your browser (unsupported codec). Try MP4 (H.264) or WebM.";
  }

  togglePlay(): void {
    const video = this.videoEl?.nativeElement;
    if (!video) return;
    if (video.paused) {
      void video.play().then(() => {
        this.playing = true;
        this.startLoop();
      }).catch(() => {});
    } else {
      video.pause();
      this.playing = false;
      this.stopLoop();
      this.renderPreview();
    }
  }

  scrubTo(t: number): void {
    const video = this.videoEl?.nativeElement;
    if (!video) return;
    this.videoTime = t;
    video.currentTime = Math.min(Math.max(0, t), this.videoDuration || 0);
    // The seeked event redraws; draw immediately too for responsiveness.
    this.renderPreview();
  }

  onVideoEnded(): void {
    this.playing = false;
    this.stopLoop();
    this.renderPreview();
  }

  formatTime(s: number): string {
    if (!s || s < 0) return '0:00';
    const m = Math.floor(s / 60);
    const sec = Math.floor(s % 60);
    return `${m}:${sec.toString().padStart(2, '0')}`;
  }

  private startLoop(): void {
    this.stopLoop();
    const tick = () => {
      const video = this.videoEl?.nativeElement;
      if (video && !video.paused) {
        this.videoTime = video.currentTime;
        this.renderPreview();
        this.rafId = requestAnimationFrame(tick);
      }
    };
    this.rafId = requestAnimationFrame(tick);
  }

  private stopLoop(): void {
    if (this.rafId) {
      cancelAnimationFrame(this.rafId);
      this.rafId = 0;
    }
  }

  /** Lazily boot FFmpeg.wasm (single-thread core, no special headers needed).
   * Never called on page load — only on first video export. The promise is
   * shared so concurrent clicks boot exactly once. */
  private ensureEngine(): Promise<void> {
    if (this.engine?.isLoaded()) return Promise.resolve();
    if (!this.engineLoading) {
      this.engineLoading = (async () => {
        // The 0.11 package is CJS: esbuild exposes it as a default-only
        // namespace, so normalize both shapes here.
        const ns = (await import('@ffmpeg/ffmpeg')) as unknown as Record<string, unknown>;
        const api =
          typeof ns['createFFmpeg'] === 'function'
            ? (ns as unknown as FfmpegFactory)
            : (ns['default'] as unknown as FfmpegFactory);
        const core = await loadVerifiedCore();
        const ff = api.createFFmpeg({ ...core, mainName: FF_MAIN_NAME, log: false });
        ff.setProgress(({ ratio }: { ratio: number }) => this.onEngineProgress(ratio));
        await ff.load();
        this.ffApi = api;
        this.engine = ff;
        this.engineReady = true;
      })().catch((err) => {
        this.engineLoading = null;
        throw err;
      });
    }
    return this.engineLoading;
  }

  private onEngineProgress(ratio: number): void {
    if (this.exportState !== 'rendering') this.exportState = 'rendering';
    if (typeof ratio === 'number' && isFinite(ratio)) {
      this.exportProgress = Math.min(99, Math.max(0, Math.round(ratio * 100)));
    }
    if (Date.now() - this.exportStartedAt > 5000) this.showProgressBar = true;
  }

  /** Render the watermark alone (transparent PNG) at full video resolution,
   * with opacity baked into the alpha channel for the ffmpeg overlay. */
  private renderWatermarkPng(vw: number, vh: number): HTMLCanvasElement | null {
    if (this.wmKind === 'logo' && !this.logoImg) return null;
    if (this.wmKind === 'text' && !this.watermarkText.trim()) return null;
    const canvas = document.createElement('canvas');
    canvas.width = Math.max(1, vw);
    canvas.height = Math.max(1, vh);
    const ctx = canvas.getContext('2d');
    if (!ctx) return null;
    const cx = this.pos.x * vw;
    const cy = this.pos.y * vh;
    ctx.save();
    ctx.translate(cx, cy);
    ctx.rotate((this.rotation * Math.PI) / 180);
    if (this.wmKind === 'logo' && this.logoImg) {
      const lw = (this.sizePct / 100) * vw;
      const lh = lw * (this.logoImg.naturalHeight / Math.max(1, this.logoImg.naturalWidth));
      ctx.drawImage(this.logoImg, -lw / 2, -lh / 2, lw, lh);
    } else {
      const px = (this.sizePct / 100) * vw;
      ctx.font = `${px}px ${this.fontStack}`;
      ctx.textBaseline = 'middle';
      ctx.textAlign = 'center';
      ctx.strokeStyle = 'rgba(0,0,0,0.5)';
      ctx.lineWidth = Math.max(1, px / 18);
      ctx.strokeText(this.watermarkText, 0, 0);
      ctx.fillStyle = this.color;
      ctx.fillText(this.watermarkText, 0, 0);
    }
    ctx.restore();
    // Multiply opacity into alpha so the overlay filter needs no extra stage.
    if (this.opacityPct < 100) {
      ctx.save();
      ctx.globalCompositeOperation = 'destination-in';
      ctx.globalAlpha = this.opacityPct / 100;
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.restore();
    }
    return canvas;
  }

  /** Export the watermarked video as MP4 (H.264 + AAC) via FFmpeg.wasm. */
  async exportVideo(): Promise<void> {
    const video = this.videoEl?.nativeElement;
    if (!video || !this.videoReady || this.exportState === 'loading' || this.exportState === 'rendering') {
      return;
    }
    this.exportError = '';
    this.showShare = false;
    this.exportProgress = 0;
    this.showProgressBar = false;
    this.exportState = 'loading';
    this.exportStartedAt = Date.now();
    let ff: FfmpegEngine | null = null;
    let unlinkInputs: string[] = [];
    this.progressTimer = window.setTimeout(() => {
      if (this.exportState === 'rendering') this.showProgressBar = true;
    }, 5000);

    try {
      await this.ensureEngine();
      ff = this.engine;
      if (!ff) throw new Error('engine unavailable');
      const vw = video.videoWidth;
      const vh = video.videoHeight;
      const wm = this.renderWatermarkPng(vw, vh);
      if (!wm) throw new Error('Add watermark text or a logo first.');
      const wmBytes = await new Promise<Uint8Array>((resolve, reject) => {
        wm.toBlob((blob) => {
          if (!blob) {
            reject(new Error('watermark raster failed'));
            return;
          }
          blob.arrayBuffer().then(
            (buf) => resolve(new Uint8Array(buf)),
            () => reject(new Error('watermark raster failed')),
          );
        }, 'image/png');
      });
      const ext = this.file?.type === 'video/webm' ? 'webm' : this.file?.type === 'video/quicktime' ? 'mov' : 'mp4';
      if (!this.ffApi || !this.file) throw new Error('engine unavailable');
      ff.FS('writeFile', `input.${ext}`, await this.ffApi.fetchFile(this.file));
      ff.FS('writeFile', 'wm.png', wmBytes);
      const rad = (this.rotation * Math.PI) / 180;
      const fx = this.pos.x.toFixed(4);
      const fy = this.pos.y.toFixed(4);
      // Note: no shell quoting here — args pass directly to ffmpeg, so
      // literal quotes would break filter parsing.
      const overlay =
        this.rotation !== 0
          ? `[1:v]format=rgba,rotate=${rad}:fillcolor=none@0:ow=rotw(iw):oh=roth(ih)[wm];[0:v][wm]overlay=x=(W-w)*${fx}:y=(H-h)*${fy}:format=yuv420`
          : `[0:v][1:v]overlay=x=(W-w)*${fx}:y=(H-h)*${fy}:format=yuv420`;
      this.exportState = 'rendering';
      await ff.run(
        '-i', `input.${ext}`,
        '-i', 'wm.png',
        '-filter_complex', overlay,
        '-c:v', 'libx264', '-preset', 'veryfast', '-crf', '23', '-pix_fmt', 'yuv420p',
        '-c:a', 'aac', '-b:a', '128k',
        '-movflags', '+faststart', '-shortest',
        'out.mp4',
      );
      const data = ff.FS('readFile', 'out.mp4') as unknown as Uint8Array;
      unlinkInputs = [`input.${ext}`, 'wm.png', 'out.mp4'];
      const buf = data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength) as ArrayBuffer;
      if (this.downloadUrl) URL.revokeObjectURL(this.downloadUrl);
      this.downloadUrl = URL.createObjectURL(new Blob([buf], { type: 'video/mp4' }));
      const link = document.createElement('a');
      link.href = this.downloadUrl;
      link.download = `watermarked-${this.file?.name.replace(/\.[^.]+$/, '') || 'video'}.mp4`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      this.exportState = 'done';
      this.exportProgress = 100;
      this.showShare = true;
      this.confettiBurst();
    } catch (err) {
      const detail = err instanceof Error ? err.message : String(err);
      this.exportState = 'error';
      this.exportError = detail
        ? `Processing failed (${detail.slice(0, 140)}). Try a smaller file.`
        : 'Processing failed. Try a smaller file.';
    } finally {
      // Always release WASM MEMFS files, even when the run throws — failed
      // exports of large videos would otherwise leak hundreds of MB.
      if (ff) {
        for (const name of unlinkInputs) {
          try {
            ff.FS('unlink', name);
          } catch {
            // Best effort cleanup of the in-memory FS.
          }
        }
      }
      this.clearProgressTimer();
    }
  }

  /** GSAP confetti burst (~1s particles) inside the preview panel. */
  private confettiBurst(): void {
    const layer = this.fxLayer?.nativeElement;
    if (!layer) return;
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (reduce) return;
    // Brand colours, read from the tokens so the burst matches the page.
    const css = getComputedStyle(layer);
    const colors = ['--color-accent', '--color-accent-text', '--color-ink', '--color-caution'].map((v) => css.getPropertyValue(v).trim() || 'currentColor');
    const rect = layer.getBoundingClientRect();
    for (let i = 0; i < 48; i++) {
      const p = document.createElement('span');
      const size = 5 + Math.random() * 6;
      p.style.cssText = `position:absolute;left:${rect.width / 2}px;top:${rect.height * 0.35}px;width:${size}px;height:${size * (Math.random() > 0.5 ? 1 : 0.5)}px;background:${colors[i % colors.length]};border-radius:2px;pointer-events:none;z-index:30;`;
      layer.appendChild(p);
      gsap.fromTo(
        p,
        { x: 0, y: 0, opacity: 1, scale: 1 },
        {
          x: gsap.utils.random(-170, 170),
          y: gsap.utils.random(-60, 190),
          rotation: gsap.utils.random(-270, 270),
          opacity: 0,
          duration: gsap.utils.random(0.7, 1.1),
          ease: 'power2.out',
          onComplete: () => p.remove(),
        },
      );
    }
  }

  // ------------------------------------------------------------------ share

  get xShareHref(): string {
    const params = new URLSearchParams({ text: this.shareText, url: this.shareUrl });
    return `https://twitter.com/intent/tweet?${params.toString()}`;
  }

  get facebookShareHref(): string {
    const params = new URLSearchParams({ u: this.shareUrl });
    return `https://www.facebook.com/sharer/sharer.php?${params.toString()}`;
  }

  get whatsappShareHref(): string {
    const params = new URLSearchParams({ text: `${this.shareText} ${this.shareUrl}` });
    return `https://wa.me/?${params.toString()}`;
  }

  copyShareLink(): void {
    const done = () => {
      this.copiedLink = true;
      this.later(() => (this.copiedLink = false), 2000);
    };
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(this.shareUrl).then(done, done);
    } else {
      done();
    }
  }

  dismissGate(): void {
    this.sizeGateDismissed = true;
  }

  // ----------------------------------------------------------------- export

  private generateDownload(seq: number): void {
    if (this.fileType !== 'image' || !this.baseImg) return;
    const canvas = document.createElement('canvas');
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    canvas.width = this.baseW;
    canvas.height = this.baseH;
    if (this.imgFormat === 'jpeg') {
      // JPEG has no alpha: composite over white instead of black.
      ctx.fillStyle = '#ffffff';
      ctx.fillRect(0, 0, canvas.width, canvas.height);
    }
    ctx.drawImage(this.baseImg, 0, 0);
    if (!this.showOriginal) this.drawWatermark(ctx, canvas.width, canvas.height);
    const mime = this.imgFormat === 'jpeg' ? 'image/jpeg' : 'image/png';
    canvas.toBlob(
      (blob) => {
        // Drop renders superseded by newer control changes (stale toBlob).
        if (seq !== this.renderSeq) return;
        if (blob) {
          if (this.downloadUrl) URL.revokeObjectURL(this.downloadUrl);
          this.downloadUrl = URL.createObjectURL(blob);
        }
      },
      mime,
      0.92,
    );
  }
  download(): void {
    if (!this.downloadUrl || this.fileType !== 'image') return;
    const ext = this.imgFormat === 'jpeg' ? 'jpg' : 'png';
    const link = document.createElement('a');
    link.href = this.downloadUrl;
    link.download = `watermarked-${this.file?.name.replace(/\.[^.]+$/, '') || 'image'}.${ext}`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    this.exportState = 'done';
    this.showShare = true;
    this.confettiBurst();
    this.later(() => {
      if (this.exportState === 'done') this.exportState = 'idle';
    }, 3000);
  }

  setImgFormat(format: 'png' | 'jpeg'): void {
    if (this.imgFormat === format) return;
    this.imgFormat = format;
    this.updatePreview();
  }

  exportBusy(): boolean {
    return this.exportState === 'loading' || this.exportState === 'rendering';
  }

  resetFile(): void {
    this.reset();
  }

  reset(): void {
    this.stopLoop();
    this.clearProgressTimer();
    if (this.fileUrl) URL.revokeObjectURL(this.fileUrl);
    if (this.logoUrl) URL.revokeObjectURL(this.logoUrl);
    if (this.downloadUrl) URL.revokeObjectURL(this.downloadUrl);
    this.file = null;
    this.fileType = null;
    this.fileUrl = '';
    this.fileError = '';
    this.wmKind = 'text';
    this.watermarkText = 'Your Watermark';
    this.fontStack = FONTS[0].stack;
    this.fontLabel = FONTS[0].label;
    this.fontOpen = false;
    this.color = '#ffffff';
    this.opacityPct = 50;
    this.sizePct = 25;
    this.rotation = 0;
    this.pos = { x: 0.75, y: 0.85 };
    this.aspect = 'original';
    this.showOriginal = false;
    this.logoUrl = '';
    this.logoImg = null;
    this.baseImg = null;
    this.videoDuration = 0;
    this.videoTime = 0;
    this.playing = false;
    this.videoReady = false;
    this.exportState = 'idle';
    this.exportProgress = 0;
    this.showProgressBar = false;
    this.exportError = '';
    this.clearProgressTimer();
    this.imgFormat = 'png';
    this.sizeGate = false;
    this.sizeGateDismissed = false;
    this.showShare = false;
    this.copiedLink = false;
    this.downloadUrl = '';
  }
}
