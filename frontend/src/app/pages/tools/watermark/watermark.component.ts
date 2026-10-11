import { Component, ElementRef, ViewChild, AfterViewInit, OnInit, OnDestroy, inject } from '@angular/core';
import { CommonModule, DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { NavMobileMenuComponent } from '../../../shared/components/nav-mobile-menu/nav-mobile-menu.component';
import { FormsModule } from '@angular/forms';
import gsap from 'gsap';
import { setPageSeo } from '../../../shared/seo';

// FFmpeg.wasm single-thread core (no COOP/COEP headers needed). Pinned CDN
// URLs, fetched lazily on first video export — never on page load.
const FF_CORE_JS = 'https://cdn.jsdelivr.net/npm/@ffmpeg/core-st@0.11.1/dist/ffmpeg-core.js';
const FF_CORE_WASM = 'https://cdn.jsdelivr.net/npm/@ffmpeg/core-st@0.11.1/dist/ffmpeg-core.wasm';
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
  createFFmpeg(options: { corePath: string; mainName?: string; log?: boolean }): FfmpegEngine;
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
  imports: [CommonModule, RouterLink, FormsModule, NavMobileMenuComponent],
  styles: [`
    .serif { font-family: 'Cormorant Garamond', 'Playfair Display', Georgia, serif; }
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .btn-primary { transition: transform .2s cubic-bezier(0.23,1,0.32,1), background-color .2s; }
    .btn-primary:hover { transform: translateY(-1px); }
    .btn-primary:active { transform: translateY(1px) scale(.98); }
    .grain::before {
      content: ''; position: fixed; inset: 0; z-index: 60; pointer-events: none; opacity: .05;
      background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.5'/%3E%3C/svg%3E");
    }
    input[type="range"] { accent-color: #BC5227; }
    .drag-surface { touch-action: none; }
    @media (prefers-reduced-motion: reduce) {
      .btn-primary { transition: none; }
    }
  `],
  template: `
    <div class="grain sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh] flex flex-col">
      <!-- Floating island nav -->
      <header class="fixed top-4 left-1/2 -translate-x-1/2 z-50 w-[min(1120px,calc(100%-2rem))]">
        <nav class="flex items-center justify-between h-14 pl-5 pr-2 rounded-full bg-[#0F0E0B]/90 backdrop-blur-xl border border-white/10 shadow-[0_8px_30px_rgba(20,15,10,0.25)]">
          <a routerLink="/" class="flex items-center gap-2 min-h-[44px] text-[#FAF7F2]">
            <span class="font-semibold tracking-tight text-[17px]">Indifferent<span class="text-[#D96C3D]">.</span></span>
          </a>
          <div class="hidden md:flex items-center gap-7 text-[13.5px] text-white/70">
            <a routerLink="/" fragment="process" class="hover:text-white transition-colors">How it works</a>
            <a routerLink="/blog" class="hover:text-white transition-colors">Journal</a>
            <a routerLink="/about" class="hover:text-white transition-colors">Studio</a>
          </div>
          <div class="flex items-center gap-1">
            <app-nav-mobile-menu>
              <a routerLink="/" fragment="process" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">How it works</a>
              <a routerLink="/blog" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">Journal</a>
              <a routerLink="/about" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">Studio</a>
            </app-nav-mobile-menu>
            <a routerLink="/login" class="btn-primary inline-flex items-center gap-2 h-11 pl-5 pr-5 sm:pr-1.5 rounded-full bg-[#FAF7F2] text-[#141310] text-[13.5px] font-semibold">
            Sign in
            <span class="hidden sm:flex w-7 h-7 rounded-full bg-[#1A1714] text-white items-center justify-center text-sm leading-none" aria-hidden="true">↗</span>
          </a>
          </div>
        </nav>
      </header>

      <!-- Compact light hero -->
      <section class="pt-28 pb-10 border-b border-black/[0.07]">
        <div class="w-[min(1240px,100%)] mx-auto px-5 md:px-10">
          <p class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-[10.5px] uppercase tracking-[0.22em] border border-black/15 text-[#6B6560]">
            <span class="w-1.5 h-1.5 rounded-full bg-[#D96C3D]"></span> Finishing · Tool
          </p>
          <h1 class="serif mt-4 font-medium leading-[0.95] text-[clamp(2.4rem,5vw,3.8rem)]">Brand it.</h1>
          <p class="mt-3 text-[#6B6560] max-w-[56ch] leading-relaxed">Add text or logo watermarks to images and videos. Free, private, processed in your browser. Nothing uploads anywhere.</p>
        </div>
      </section>

      <main class="flex-1 w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-12 md:py-16">
        <!-- Upload State -->
        <div *ngIf="!file" class="flex justify-center">
          <div
            class="rounded-[20px] border-2 border-dashed px-8 py-16 text-center cursor-pointer w-full max-w-[560px] transition-colors"
            [class]="isDragOver ? 'border-[#BC5227] bg-[#BC5227]/[.06]' : 'border-black/20 bg-white hover:border-black/40'"
            (click)="fileInput.click()"
            (dragover)="onDragOver($event)"
            (dragleave)="onDragLeave($event)"
            (drop)="onDrop($event)"
          >
            <svg xmlns="http://www.w3.org/2000/svg" width="44" height="44" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="mx-auto mb-5 text-[#6B6560]" aria-hidden="true">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
              <polyline points="17 8 12 3 7 8"/>
              <line x1="12" y1="3" x2="12" y2="15"/>
            </svg>
            <h2 class="serif text-[30px] leading-tight">Drop your file here</h2>
            <p class="mt-2 text-[14px] text-[#6B6560]">or click to browse · JPG, PNG, WebP, MP4</p>
            @if (fileError) {
              <p class="mt-3 text-[13.5px] font-medium text-red-600" role="alert">{{ fileError }}</p>
            }
          </div>
          <input
            #fileInput
            type="file"
            accept="image/jpeg,image/png,image/webp,video/mp4,video/webm,video/quicktime"
            (change)="onFileSelected($event)"
            style="display: none;"
          />
        </div>

        <!-- Editor: two panels — preview first on mobile, controls first on desktop -->
        <div *ngIf="file" class="grid grid-cols-1 lg:grid-cols-[320px_minmax(0,1fr)] gap-5 items-start">
          <!-- LIVE PREVIEW (70%) -->
          <div class="order-first lg:order-2 rounded-[20px] p-2 bg-black/[0.04] border border-black/10 min-w-0">
            <div class="rounded-[calc(20px-0.5rem)] bg-white border border-black/[0.06] p-3 sm:p-4">
              <div class="flex flex-wrap items-center justify-between gap-2 mb-3">
                <div class="flex items-center gap-1.5" role="group" aria-label="Aspect ratio">
                  @for (a of aspectOptions; track a) {
                    <button
                      type="button"
                      (click)="toggleAspect(a)"
                      [attr.aria-pressed]="aspect === a"
                      class="min-h-[36px] px-3 rounded-full text-[12.5px] font-semibold border transition-colors"
                      [class]="aspect === a ? 'bg-[#1A1714] text-white border-[#1A1714]' : 'border-black/15 text-[#3E3A33] hover:border-black/40'"
                    >{{ a === 'original' ? 'Original' : a }}</button>
                  }
                </div>
                <button
                  type="button"
                  (click)="showOriginal = !showOriginal; updatePreview()"
                  [attr.aria-pressed]="showOriginal"
                  class="min-h-[36px] px-4 rounded-full text-[12.5px] font-semibold border transition-colors"
                  [class]="showOriginal ? 'bg-[#BC5227] text-white border-[#BC5227]' : 'border-black/15 text-[#3E3A33] hover:border-black/40'"
                >{{ showOriginal ? 'Show watermarked' : 'Before / After' }}</button>
              </div>

              <div
                #fxLayer
                class="drag-surface relative flex items-center justify-center overflow-hidden rounded-xl bg-[#1A1714]/[.03] min-h-[280px] cursor-crosshair"
                (pointerdown)="onPointerDown($event)"
                (pointermove)="onPointerMove($event)"
                (pointerup)="onPointerUp()"
                (pointercancel)="onPointerUp()"
                (pointerleave)="onPointerUp()"
              >
                <canvas #previewCanvas class="max-w-full max-h-[62vh] rounded-lg shadow-[0_4px_20px_rgba(0,0,0,0.08)]"></canvas>
                <video #videoEl class="hidden" [src]="fileUrl" muted playsInline preload="auto" (loadedmetadata)="onVideoLoaded()" (seeked)="renderPreview()" (ended)="onVideoEnded()"></video>
                <span class="absolute bottom-3 left-1/2 -translate-x-1/2 rounded-full bg-[#1A1714]/[.06] border border-black/10 text-[#3E3A33] text-[11.5px] font-medium px-3 py-1 pointer-events-none whitespace-nowrap">Drag to move watermark</span>
              </div>

              @if (fileType === 'video') {
                <div class="mt-3 flex items-center gap-3">
                  <button
                    type="button"
                    (click)="togglePlay()"
                    class="shrink-0 w-11 h-11 rounded-full bg-[#1A1714] text-white inline-flex items-center justify-center text-[15px]"
                    [attr.aria-label]="playing ? 'Pause video' : 'Play video'"
                  >{{ playing ? '❚❚' : '▶' }}</button>
                  <input
                    type="range" min="0" [max]="videoDuration || 0" step="0.1"
                    [ngModel]="videoTime" (ngModelChange)="scrubTo($event)"
                    class="flex-1" aria-label="Video timeline scrubber"
                  />
                  <span class="text-[12px] font-mono text-[#6B6560] tabular-nums shrink-0">{{ formatTime(videoTime) }} / {{ formatTime(videoDuration) }}</span>
                </div>
              }

              @if (showShare) {
                <div class="mt-4 rounded-2xl border border-black/10 bg-[#1E3A2A] text-[#EDE8DB] p-5">
                  <p class="serif text-[20px]">Share your work</p>
                  <p class="mt-1 text-[13px] text-white/70">Your watermarked file is ready. Show it off.</p>
                  <div class="mt-3 flex flex-wrap gap-2">
                    <a [href]="xShareHref" target="_blank" rel="noopener"
                      class="inline-flex items-center min-h-[40px] rounded-full bg-white/10 border border-white/15 px-4 text-[13px] font-semibold hover:bg-white/20 transition-colors">Post to X</a>
                    <a [href]="facebookShareHref" target="_blank" rel="noopener"
                      class="inline-flex items-center min-h-[40px] rounded-full bg-white/10 border border-white/15 px-4 text-[13px] font-semibold hover:bg-white/20 transition-colors">Facebook</a>
                    <a [href]="whatsappShareHref" target="_blank" rel="noopener"
                      class="inline-flex items-center min-h-[40px] rounded-full bg-white/10 border border-white/15 px-4 text-[13px] font-semibold hover:bg-white/20 transition-colors">WhatsApp</a>
                    <button type="button" (click)="copyShareLink()"
                      class="inline-flex items-center min-h-[40px] rounded-full bg-white/10 border border-white/15 px-4 text-[13px] font-semibold hover:bg-white/20 transition-colors">
                      {{ copiedLink ? 'Copied!' : 'Copy link' }}
                    </button>
                  </div>
                </div>
              }
            </div>
          </div>

          <!-- CONTROLS (30%) -->
          <div class="order-last lg:order-1 rounded-[20px] bg-white border border-black/[0.07] p-5 lg:sticky lg:top-24">
            <div class="flex items-center justify-between mb-4">
              <h2 class="serif text-[20px]">Settings</h2>
              <button type="button" (click)="resetFile()" class="text-[12.5px] font-semibold text-[#BC5227] hover:underline min-h-[32px]">New file</button>
            </div>

            <div class="mb-4">
              <span class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5" id="wm-type-label">Type</span>
              <div class="grid grid-cols-2 gap-2" role="group" aria-labelledby="wm-type-label">
                <button type="button" (click)="setKind('text')" [attr.aria-pressed]="wmKind === 'text'"
                  class="min-h-[44px] rounded-[12px] border text-[14px] font-semibold transition-colors"
                  [class]="wmKind === 'text' ? 'bg-[#1A1714] text-white border-[#1A1714]' : 'border-black/15 hover:border-black/40'">Text</button>
                <button type="button" (click)="setKind('logo')" [attr.aria-pressed]="wmKind === 'logo'"
                  class="min-h-[44px] rounded-[12px] border text-[14px] font-semibold transition-colors"
                  [class]="wmKind === 'logo' ? 'bg-[#1A1714] text-white border-[#1A1714]' : 'border-black/15 hover:border-black/40'">Logo</button>
              </div>
            </div>

            @if (wmKind === 'text') {
              <div class="mb-4">
                <label class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5" for="wm-text">Watermark text</label>
                <input id="wm-text" type="text" [(ngModel)]="watermarkText" (ngModelChange)="updatePreview()" maxlength="60"
                  class="w-full rounded-[12px] border border-black/15 bg-transparent px-4 h-11 text-[14.5px] outline-none focus:border-[#BC5227]" />
              </div>
              <div class="mb-4">
                <label class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5" for="wm-font">Font</label>
                <div class="relative">
                  <button type="button" id="wm-font" (click)="fontOpen = !fontOpen" [attr.aria-expanded]="fontOpen"
                    class="w-full min-h-[44px] rounded-[12px] border border-black/15 bg-transparent px-4 text-left text-[14.5px] flex items-center justify-between gap-2"
                    [style.font-family]="fontStack">
                    <span class="truncate">{{ fontLabel }}</span><span aria-hidden="true">▾</span>
                  </button>
                  @if (fontOpen) {
                    <ul class="absolute z-20 mt-1.5 w-full max-h-56 overflow-y-auto rounded-[12px] border border-black/15 bg-white shadow-xl py-1" role="listbox" aria-label="Font choices">
                      @for (f of fonts; track f.label) {
                        <li>
                          <button type="button" role="option" [attr.aria-selected]="fontStack === f.stack"
                            (click)="fontStack = f.stack; fontLabel = f.label; fontOpen = false; updatePreview()"
                            class="w-full text-left px-4 py-2.5 text-[15px] hover:bg-black/[0.04] min-h-[44px]"
                            [style.font-family]="f.stack">{{ f.label }}</button>
                        </li>
                      }
                    </ul>
                  }
                </div>
              </div>
            } @else {
              <div class="mb-4">
                <span class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5">Logo image</span>
                <button type="button" (click)="logoInput.click()"
                  class="w-full min-h-[44px] rounded-[12px] border border-dashed border-black/25 px-4 text-[13.5px] font-semibold text-[#3E3A33] hover:border-black/50 transition-colors">
                  {{ logoUrl ? 'Change logo (PNG with transparency works best)' : 'Upload logo (PNG/JPG/WebP)' }}
                </button>
                <input #logoInput type="file" accept="image/png,image/jpeg,image/webp" (change)="onLogoSelected($event)" style="display: none;" />
              </div>
            }

            <div class="mb-4">
              <label class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5" for="wm-opacity">Opacity</label>
              <div class="flex items-center gap-3">
                <input id="wm-opacity" type="range" min="0" max="100" step="1" [(ngModel)]="opacityPct" (ngModelChange)="updatePreview()" class="flex-1" />
                <span class="text-[12.5px] font-semibold text-[#BC5227] min-w-11 text-right tabular-nums">{{ opacityPct }}%</span>
              </div>
            </div>

            <div class="mb-4">
              <label class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5" for="wm-size">Size · % of frame width</label>
              <div class="flex items-center gap-3">
                <input id="wm-size" type="range" min="10" max="50" step="1" [(ngModel)]="sizePct" (ngModelChange)="updatePreview()" class="flex-1" />
                <span class="text-[12.5px] font-semibold text-[#BC5227] min-w-11 text-right tabular-nums">{{ sizePct }}%</span>
              </div>
            </div>

            <div class="mb-4">
              <label class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5" for="wm-color">Color</label>
              <div class="flex items-center gap-2 flex-wrap">
                <input id="wm-color" type="color" [(ngModel)]="color" (ngModelChange)="updatePreview()" aria-label="Custom watermark color"
                  class="w-11 h-11 rounded-[12px] border border-black/15 cursor-pointer p-1 bg-transparent" />
                @for (preset of colorPresets; track preset) {
                  <button type="button" (click)="color = preset; updatePreview()"
                    [attr.aria-label]="'Use color ' + preset" [attr.aria-pressed]="color === preset"
                    class="w-11 h-11 rounded-[12px] border-2 transition-transform hover:scale-105 min-w-[44px] min-h-[44px]"
                    [style.background]="preset"
                    [class]="color === preset ? 'border-[#BC5227]' : 'border-black/15'"></button>
                }
              </div>
            </div>

            <div class="mb-4">
              <label class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5" for="wm-rotation">Rotation</label>
              <div class="flex items-center gap-3">
                <input id="wm-rotation" type="range" min="-180" max="180" step="1" [(ngModel)]="rotation" (ngModelChange)="updatePreview()" class="flex-1" />
                <span class="text-[12.5px] font-semibold text-[#BC5227] min-w-11 text-right tabular-nums">{{ rotation }}&deg;</span>
              </div>
            </div>

            <div class="mb-2">
              <span class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5" id="wm-pos-label">Position</span>
              <div class="grid grid-cols-3 gap-1.5 w-fit" role="group" aria-labelledby="wm-pos-label">
                @for (row of gridRows; track row; let ri = $index) {
                  @for (col of row; track col; let ci = $index) {
                    <button type="button"
                      (click)="snapToSpot(ri, ci)"
                      [attr.aria-label]="'Move watermark to ' + spotName(ri, ci)"
                      [attr.aria-pressed]="isActiveSpot(ri, ci)"
                      class="w-11 h-11 rounded-[10px] border transition-colors min-w-[44px] min-h-[44px]"
                      [class]="isActiveSpot(ri, ci) ? 'bg-[#BC5227] border-[#BC5227]' : 'border-black/15 hover:border-black/40 bg-transparent'"></button>
                  }
                }
              </div>
              <p class="mt-1.5 text-[12px] text-[#6B6560]">Tip: drag directly on the preview, too.</p>
            </div>

            @if (sizeGate && !sizeGateDismissed) {
              <div class="rounded-2xl border border-amber-500/40 bg-amber-500/[0.07] p-4" role="alert">
                <p class="text-[13.5px] font-semibold">Large file (over 500&nbsp;MB)</p>
                <p class="mt-1 text-[12.5px] text-[#6B6560] leading-relaxed">
                  Server-side processing isn't available in this version, so exports run
                  entirely in your browser and need roughly 2&nbsp;GB of free memory.
                  You can try anyway, or pick a smaller file.
                </p>
                <div class="mt-3 flex flex-col gap-2">
                  <button type="button" (click)="dismissGate()"
                    class="w-full rounded-full bg-[#1A1714] text-white py-2.5 text-[13px] font-semibold">Process anyway</button>
                  <button type="button" (click)="reset()"
                    class="w-full rounded-full border border-black/15 py-2.5 text-[13px] font-semibold hover:border-black/40 transition-colors">Choose smaller file</button>
                </div>
              </div>
            }

            @if (fileError) {
              <p class="mt-3 text-[13px] font-medium text-red-600" role="alert">{{ fileError }}</p>
            }

            @if (exportError) {
              <p class="mt-3 rounded-xl border border-red-500/30 bg-red-500/[0.06] px-4 py-3 text-[13px] font-medium text-red-600" role="alert">{{ exportError }}</p>
            }

            <div class="flex flex-col gap-2.5 mt-5 pt-5 border-t border-black/10">
              @if (fileType === 'image') {
                <div>
                  <span class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5">Format</span>
                  <div class="grid grid-cols-2 gap-2" role="group" aria-label="Image export format">
                    <button type="button" (click)="setImgFormat('png')" [attr.aria-pressed]="imgFormat === 'png'"
                      class="min-h-[44px] rounded-[12px] border text-[14px] font-semibold transition-colors"
                      [class]="imgFormat === 'png' ? 'bg-[#1A1714] text-white border-[#1A1714]' : 'border-black/15 hover:border-black/40'">PNG</button>
                    <button type="button" (click)="setImgFormat('jpeg')" [attr.aria-pressed]="imgFormat === 'jpeg'"
                      class="min-h-[44px] rounded-[12px] border text-[14px] font-semibold transition-colors"
                      [class]="imgFormat === 'jpeg' ? 'bg-[#1A1714] text-white border-[#1A1714]' : 'border-black/15 hover:border-black/40'">JPEG</button>
                  </div>
                  <p class="mt-1.5 text-[12px] text-[#6B6560]">JPEG flattens transparency onto white.</p>
                </div>
                <button class="btn-primary w-full rounded-full bg-[#1A1714] text-white py-3.5 font-semibold text-[14.5px] disabled:opacity-40 disabled:cursor-not-allowed"
                  (click)="download()" [disabled]="!downloadUrl || exportBusy()">
                  {{ exportState === 'done' ? 'Downloaded ✓' : 'Download ' + (imgFormat === 'jpeg' ? 'JPEG' : 'PNG') }}
                </button>
              } @else {
                <div>
                  <span class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5">Format</span>
                  <div class="rounded-[12px] border border-black/15 px-4 min-h-[44px] flex items-center text-[14px] font-semibold">MP4 · H.264 + AAC</div>
                </div>
                <button class="btn-primary w-full rounded-full bg-[#1A1714] text-white py-3.5 font-semibold text-[14.5px] disabled:opacity-40 disabled:cursor-not-allowed"
                  (click)="exportVideo()" [disabled]="!videoReady || exportBusy() || (sizeGate && !sizeGateDismissed)">
                  @if (exportState === 'loading') {
                    <span class="inline-flex items-center justify-center gap-2"><span class="w-4 h-4 rounded-full border-2 border-white/40 border-t-white animate-spin" aria-hidden="true"></span>Loading engine…</span>
                  } @else if (exportState === 'rendering') {
                    <span class="inline-flex items-center justify-center gap-2"><span class="w-4 h-4 rounded-full border-2 border-white/40 border-t-white animate-spin" aria-hidden="true"></span>Rendering…{{ showProgressBar ? ' ' + exportProgress + '%' : '' }}</span>
                  } @else if (exportState === 'done') {
                    Downloaded ✓
                  } @else {
                    Export watermarked video
                  }
                </button>
                @if (showProgressBar && exportState === 'rendering') {
                  <div class="h-2 rounded-full bg-black/10 overflow-hidden" role="progressbar"
                    [attr.aria-valuenow]="exportProgress" aria-valuemin="0" aria-valuemax="100" aria-label="Render progress">
                    <div class="h-full bg-[#BC5227] rounded-full transition-all duration-300" [style.width.%]="exportProgress"></div>
                  </div>
                }
                <p class="text-[12px] text-[#6B6560] leading-relaxed">Rendered fully in your browser. Nothing uploads anywhere.</p>
              }
              <button class="w-full rounded-full border border-black/15 py-3 text-[13.5px] font-semibold text-[#3E3A33] hover:border-black/40 transition-colors" (click)="reset()">Reset</button>
            </div>
          </div>
        </div>

        <!-- Hidden file input for editor view -->
        <input
          *ngIf="file"
          #fileInput
          type="file"
          accept="image/jpeg,image/png,image/webp,video/mp4,video/webm,video/quicktime"
          (change)="onFileSelected($event)"
          style="display: none;"
        />
      </main>

      <section class="border-t border-black/[0.07]">
        <div class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-10 flex flex-col sm:flex-row justify-between gap-3 text-[12.5px] text-[#6B6560]">
          <span>© {{ currentYear }} Indifferent. All rights reserved.</span>
          <span class="flex flex-wrap">
            <a routerLink="/" class="inline-flex items-center min-h-[44px] px-3 hover:text-[#1A1714]">Home</a>
            <a routerLink="/about" class="inline-flex items-center min-h-[44px] px-3 hover:text-[#1A1714]">Studio</a>
            <a routerLink="/privacy" class="inline-flex items-center min-h-[44px] px-3 hover:text-[#1A1714]">Privacy</a>
            <a routerLink="/terms" class="inline-flex items-center min-h-[44px] px-3 hover:text-[#1A1714]">Terms</a>
          </span>
        </div>
      </section>
    </div>
  `,
})
export class WatermarkComponent implements OnInit, AfterViewInit, OnDestroy {
  private readonly document = inject(DOCUMENT);

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Free Watermark Tool: Add Text Watermarks to Images | Indifferent',
      description:
        'Free in-browser watermark tool: draggable text watermarks on JPG, PNG, WebP with size, opacity, color and rotation. Private, full-resolution PNG export.',
      canonical: 'https://indifferent.fun/tools/add-watermark',
    });
  }

  @ViewChild('previewCanvas') previewCanvas!: ElementRef<HTMLCanvasElement>;
  @ViewChild('videoEl') videoEl?: ElementRef<HTMLVideoElement>;
  @ViewChild('fxLayer') fxLayer?: ElementRef<HTMLDivElement>;

  currentYear = new Date().getFullYear();

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
  private ffApi: FfmpegFactory | null = null;

  // Image export format + large-file gate + share panel.
  imgFormat: 'png' | 'jpeg' = 'png';
  sizeGate = false;
  sizeGateDismissed = false;
  showShare = false;
  copiedLink = false;
  readonly shareUrl = 'https://indifferent.fun/tools/add-watermark';
  readonly shareText = 'I just branded my content with the Indifferent Watermark Tool — free, in-browser, private.';

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
      this.fileError = 'Unsupported file type. Please use JPG, PNG, WebP or MP4.';
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
        const ff = api.createFFmpeg({ corePath: FF_CORE_JS, mainName: FF_MAIN_NAME, log: false });
        ff.setProgress(({ ratio }: { ratio: number }) => this.onEngineProgress(ratio));
        await ff.load();
        this.ffApi = api;
        this.engine = ff;
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
        ? `Processing failed (${detail.slice(0, 140)}). Try a smaller file or use server mode.`
        : 'Processing failed. Try a smaller file or use server mode.';
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
    const colors = ['#BC5227', '#D96C3D', '#1A1714', '#6366f1', '#22c55e', '#eab308'];
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
