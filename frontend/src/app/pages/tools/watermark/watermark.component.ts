import { Component, ElementRef, ViewChild, AfterViewInit, OnInit, OnDestroy, inject } from '@angular/core';
import { CommonModule, DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { NavMobileMenuComponent } from '../../../shared/components/nav-mobile-menu/nav-mobile-menu.component';
import { FormsModule } from '@angular/forms';
import gsap from 'gsap';
import { setPageSeo } from '../../../shared/seo';

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
                class="drag-surface relative flex items-center justify-center overflow-hidden rounded-xl bg-[#1A1714]/[.03] min-h-[280px] cursor-crosshair"
                (pointerdown)="onPointerDown($event)"
                (pointermove)="onPointerMove($event)"
                (pointerup)="onPointerUp()"
                (pointercancel)="onPointerUp()"
                (pointerleave)="onPointerUp()"
              >
                <canvas #previewCanvas class="max-w-full max-h-[62vh] rounded-lg shadow-[0_4px_20px_rgba(0,0,0,0.08)]"></canvas>
                <video #videoEl class="hidden" [src]="fileUrl" muted playsInline preload="auto" (loadedmetadata)="onVideoLoaded()" (seeked)="updatePreview()" (ended)="onVideoEnded()"></video>
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

            @if (fileError) {
              <p class="mt-3 text-[13px] font-medium text-red-600" role="alert">{{ fileError }}</p>
            }

            <div class="flex flex-col gap-2.5 mt-5 pt-5 border-t border-black/10">
              @if (fileType === 'image') {
                <button class="btn-primary w-full rounded-full bg-[#1A1714] text-white py-3.5 font-semibold text-[14.5px] disabled:opacity-40 disabled:cursor-not-allowed" (click)="download()" [disabled]="!downloadUrl">Download PNG</button>
              } @else {
                <button class="btn-primary w-full rounded-full bg-[#1A1714] text-white py-3.5 font-semibold text-[14.5px] disabled:opacity-40 disabled:cursor-not-allowed"
                  (click)="recording ? stopRecording() : exportVideo()" [disabled]="!videoReady">
                  {{ recording ? 'Stop & save video (' + formatTime(videoTime) + ')' : 'Export watermarked video' }}
                </button>
                <p class="text-[12px] text-[#6B6560] leading-relaxed">Exports WebM with original audio by re-recording playback in your browser.</p>
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

  // Video state
  videoDuration = 0;
  videoTime = 0;
  playing = false;
  videoReady = false;
  recording = false;
  private rafId = 0;
  private recorder: MediaRecorder | null = null;
  private recordChunks: Blob[] = [];
  private recordTracks: MediaStreamTrack[] = [];

  isDragOver = false;
  downloadUrl = '';

  ngAfterViewInit(): void {
    // Preview renders once media is loaded.
  }

  ngOnDestroy(): void {
    this.stopLoop();
    this.stopRecordingTracks();
    if (this.fileUrl) URL.revokeObjectURL(this.fileUrl);
    if (this.logoUrl) URL.revokeObjectURL(this.logoUrl);
    if (this.downloadUrl) URL.revokeObjectURL(this.downloadUrl);
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

  /** Redraw the preview canvas synchronously (no re-decode: ~16ms budget). */
  updatePreview(): void {
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
    if (this.fileType === 'image') this.generateDownload();
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
    this.updatePreview();
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
      this.updatePreview();
    }
  }

  scrubTo(t: number): void {
    const video = this.videoEl?.nativeElement;
    if (!video) return;
    this.videoTime = t;
    video.currentTime = Math.min(Math.max(0, t), this.videoDuration || 0);
    // The seeked event redraws; draw immediately too for responsiveness.
    this.updatePreview();
  }

  onVideoEnded(): void {
    this.playing = false;
    this.stopLoop();
    if (this.recording) this.stopRecording();
    else this.updatePreview();
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
        this.updatePreview();
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

  /** Export the watermarked video by re-recording playback (WebM + audio). */
  exportVideo(): void {
    const video = this.videoEl?.nativeElement;
    const canvas = this.previewCanvas?.nativeElement;
    if (!video || !canvas || !this.videoReady || this.recording) return;
    const capture = canvas.captureStream(30);
    try {
      const withMedia = (video as HTMLVideoElement & { captureStream?: () => MediaStream; mozCaptureStream?: () => MediaStream });
      const av = withMedia.captureStream?.() ?? withMedia.mozCaptureStream?.();
      av?.getAudioTracks().forEach((t) => {
        capture.addTrack(t);
        this.recordTracks.push(t);
      });
    } catch {
      // Video-only export when audio capture is unavailable.
    }
    this.recordChunks = [];
    const rec = new MediaRecorder(capture, { mimeType: 'video/webm', videoBitsPerSecond: 8_000_000 });
    rec.ondataavailable = (e) => {
      if (e.data.size > 0) this.recordChunks.push(e.data);
    };
    rec.onstop = () => {
      const blob = new Blob(this.recordChunks, { type: 'video/webm' });
      if (this.downloadUrl) URL.revokeObjectURL(this.downloadUrl);
      this.downloadUrl = URL.createObjectURL(blob);
      this.recording = false;
      this.stopRecordingTracks();
      // Auto-download the finished export.
      const link = document.createElement('a');
      link.href = this.downloadUrl;
      link.download = `watermarked-${this.file?.name.replace(/\.[^.]+$/, '') || 'video'}.webm`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
    };
    this.recorder = rec;
    this.recording = true;
    video.currentTime = 0;
    rec.start(250);
    void video.play().then(() => {
      this.playing = true;
      this.startLoop();
    }).catch(() => {
      this.recording = false;
    });
  }

  stopRecording(): void {
    if (this.recorder && this.recorder.state !== 'inactive') {
      this.recorder.stop();
    } else {
      this.recording = false;
    }
    this.videoEl?.nativeElement.pause();
    this.playing = false;
    this.stopLoop();
    this.stopRecordingTracks();
  }

  private stopRecordingTracks(): void {
    this.recordTracks.forEach((t) => {
      try {
        t.stop();
      } catch {
        // Already stopped.
      }
    });
    this.recordTracks = [];
  }

  // ----------------------------------------------------------------- export

  private generateDownload(): void {
    if (this.fileType !== 'image' || !this.baseImg) return;
    const canvas = document.createElement('canvas');
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    canvas.width = this.baseW;
    canvas.height = this.baseH;
    ctx.drawImage(this.baseImg, 0, 0);
    if (!this.showOriginal) this.drawWatermark(ctx, canvas.width, canvas.height);
    canvas.toBlob((blob) => {
      if (blob) {
        if (this.downloadUrl) URL.revokeObjectURL(this.downloadUrl);
        this.downloadUrl = URL.createObjectURL(blob);
      }
    }, 'image/png');
  }

  download(): void {
    if (!this.downloadUrl) return;
    const link = document.createElement('a');
    link.href = this.downloadUrl;
    link.download = `watermarked-${this.file?.name || 'image.png'}`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }

  resetFile(): void {
    this.reset();
  }

  reset(): void {
    this.stopLoop();
    this.stopRecordingTracks();
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
    this.recording = false;
    this.downloadUrl = '';
  }
}
