import { Component, ElementRef, ViewChild, AfterViewInit, OnInit, inject } from '@angular/core';
import { CommonModule, DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { NavMobileMenuComponent } from '../../../shared/components/nav-mobile-menu/nav-mobile-menu.component';
import { FormsModule } from '@angular/forms';
import { setPageSeo } from '../../../shared/seo';

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
            <a routerLink="/login" class="hover:text-white transition-colors">Templates</a>
            <a routerLink="/blog" class="hover:text-white transition-colors">Journal</a>
            <a routerLink="/about" class="hover:text-white transition-colors">Studio</a>
          </div>
          <div class="flex items-center gap-1">
            <app-nav-mobile-menu>
              <a routerLink="/login" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">Templates</a>
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
          <p class="mt-3 text-[#6B6560] max-w-[56ch] leading-relaxed">Add text watermarks to your images. Free, private, processed in your browser. Nothing uploads anywhere.</p>
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
            <h2 class="serif text-[30px] leading-tight">Drop your image here</h2>
            <p class="mt-2 text-[14px] text-[#6B6560]">or click to browse · JPG, PNG, WebP up to 20MB</p>
          </div>
          <input
            #fileInput
            type="file"
            accept="image/jpeg,image/png,image/webp,video/mp4,video/webm"
            (change)="onFileSelected($event)"
            style="display: none;"
          />
        </div>

        <!-- Editor State -->
        <div *ngIf="file && fileType === 'image'" class="grid grid-cols-1 lg:grid-cols-[1fr_320px] gap-5 items-start">
          <div class="rounded-[20px] p-2 bg-black/[0.04] border border-black/10">
            <div
              class="rounded-[calc(20px-0.5rem)] bg-white border border-black/[0.06] flex items-center justify-center relative min-h-[380px] cursor-crosshair overflow-hidden p-4"
              (mousedown)="onCanvasMouseDown($event)"
              (mousemove)="onCanvasMouseMove($event)"
              (mouseup)="onCanvasMouseUp()"
              (mouseleave)="onCanvasMouseUp()"
            >
              <canvas #previewCanvas class="max-w-full max-h-[62vh] rounded-lg shadow-[0_4px_20px_rgba(0,0,0,0.08)]"></canvas>
              <span class="absolute bottom-3 left-1/2 -translate-x-1/2 rounded-full bg-[#1A1714]/[.06] border border-black/10 text-[#3E3A33] text-[11.5px] font-medium px-3 py-1 pointer-events-none">Drag to move watermark</span>
            </div>
          </div>
          <div class="rounded-[20px] bg-white border border-black/[0.07] p-5 lg:sticky lg:top-24">
            <div class="flex items-center justify-between mb-4">
              <h2 class="serif text-[20px]">Settings</h2>
            </div>
            <div class="mb-4">
              <label class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5">Watermark text</label>
              <input type="text" [(ngModel)]="watermarkText" (ngModelChange)="updatePreview()"
                class="w-full rounded-[12px] border border-black/15 bg-transparent px-4 h-11 text-[14.5px] outline-none focus:border-[#BC5227]" />
            </div>
            <div class="mb-4">
              <label class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5">Font size</label>
              <div class="flex items-center gap-3">
                <input type="range" min="12" max="72" step="1" [(ngModel)]="fontSize" (ngModelChange)="updatePreview()" class="flex-1" />
                <span class="text-[12.5px] font-semibold text-[#BC5227] min-w-10 text-right">{{ fontSize }}px</span>
              </div>
            </div>
            <div class="mb-4">
              <label class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5">Opacity</label>
              <div class="flex items-center gap-3">
                <input type="range" min="0.1" max="1" step="0.05" [(ngModel)]="opacity" (ngModelChange)="updatePreview()" class="flex-1" />
                <span class="text-[12.5px] font-semibold text-[#BC5227] min-w-10 text-right">{{ (opacity * 100).toFixed(0) }}%</span>
              </div>
            </div>
            <div class="mb-4">
              <label class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5">Color</label>
              <div class="flex items-center gap-3">
                <input type="color" [(ngModel)]="color" (ngModelChange)="updatePreview()" class="w-9 h-9 rounded-lg border border-black/15 cursor-pointer p-0.5 bg-transparent" />
                <span class="text-[12px] text-[#6B6560] font-mono">{{ color }}</span>
              </div>
            </div>
            <div class="mb-2">
              <label class="block text-[11px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-1.5">Rotation</label>
              <div class="flex items-center gap-3">
                <input type="range" min="-180" max="180" step="1" [(ngModel)]="rotation" (ngModelChange)="updatePreview()" class="flex-1" />
                <span class="text-[12.5px] font-semibold text-[#BC5227] min-w-10 text-right">{{ rotation }}&deg;</span>
              </div>
            </div>
            <div class="flex flex-col gap-2.5 mt-5 pt-5 border-t border-black/10">
              <button class="btn-primary w-full rounded-full bg-[#1A1714] text-white py-3.5 font-semibold text-[14.5px] disabled:opacity-40 disabled:cursor-not-allowed" (click)="download()" [disabled]="!downloadUrl">Download</button>
              <button class="w-full rounded-full border border-black/15 py-3 text-[13.5px] font-semibold text-[#3E3A33] hover:border-black/40 transition-colors" (click)="reset()">Reset</button>
            </div>
          </div>
        </div>

        <!-- Video Coming Soon -->
        <div *ngIf="file && fileType === 'video'" class="mx-auto max-w-[560px] text-center rounded-[20px] bg-white border border-black/[0.07] px-8 py-14">
          <span class="inline-block rounded-full bg-[#BC5227]/10 text-[#BC5227] text-[12px] font-semibold px-3 py-1 mb-4">Coming soon</span>
          <h2 class="serif text-[32px]">Video watermarking</h2>
          <p class="mt-2 text-[#6B6560] text-[14.5px]">Video support is arriving soon. Please upload an image for now.</p>
          <button class="mt-6 rounded-full border border-black/15 px-6 py-3 text-[14px] font-semibold hover:border-black/40 transition-colors" (click)="reset()">Try another file</button>
        </div>

        <!-- Hidden file input for editor view -->
        <input
          *ngIf="file"
          #fileInput
          type="file"
          accept="image/jpeg,image/png,image/webp,video/mp4,video/webm"
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
export class WatermarkComponent implements OnInit, AfterViewInit {
  private readonly document = inject(DOCUMENT);

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Free Watermark Tool: Add Text Watermarks to Images | Indifferent',
      description:
        'Free in-browser watermark tool: draggable text watermarks on JPG, PNG and WebP with size, opacity, color and rotation. Private, full-resolution PNG export.',
      canonical: 'https://indifferent.fun/tools/add-watermark',
    });
  }

  @ViewChild('previewCanvas') previewCanvas!: ElementRef<HTMLCanvasElement>;

  currentYear = new Date().getFullYear();

  file: File | null = null;
  fileType: 'image' | 'video' | null = null;
  fileUrl = '';

  watermarkText = 'Your Watermark';
  fontSize = 24;
  opacity = 0.5;
  color = '#ffffff';
  rotation = 0;

  // Draggable position (percentage 0-1)
  dragX = 0.75;
  dragY = 0.85;
  isDragging = false;

  isDragOver = false;
  downloadUrl = '';

  ngAfterViewInit(): void {
    // Preview will render once an image is loaded
  }

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
    if (files && files.length > 0) {
      this.handleFile(files[0]);
    }
  }

  onFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    if (input.files && input.files.length > 0) {
      this.handleFile(input.files[0]);
    }
  }

  private handleFile(file: File): void {
    const imageTypes = ['image/jpeg', 'image/png', 'image/webp'];
    const videoTypes = ['video/mp4', 'video/webm'];

    if (imageTypes.includes(file.type)) {
      this.file = file;
      this.fileType = 'image';
      this.fileUrl = URL.createObjectURL(file);
      setTimeout(() => this.updatePreview(), 100);
    } else if (videoTypes.includes(file.type)) {
      this.file = file;
      this.fileType = 'video';
      this.fileUrl = URL.createObjectURL(file);
    }
  }

  // --- Draggable watermark handlers ---

  onCanvasMouseDown(event: MouseEvent): void {
    if (!this.previewCanvas) return;
    this.isDragging = true;
    this.updateDragPosition(event);
  }

  onCanvasMouseMove(event: MouseEvent): void {
    if (!this.isDragging || !this.previewCanvas) return;
    this.updateDragPosition(event);
  }

  onCanvasMouseUp(): void {
    this.isDragging = false;
  }

  private updateDragPosition(event: MouseEvent): void {
    const canvas = this.previewCanvas.nativeElement;
    const rect = canvas.getBoundingClientRect();

    const x = (event.clientX - rect.left) / rect.width;
    const y = (event.clientY - rect.top) / rect.height;

    this.dragX = Math.max(0, Math.min(1, x));
    this.dragY = Math.max(0, Math.min(1, y));

    this.updatePreview();
  }

  // --- Preview and watermark rendering ---

  updatePreview(): void {
    if (this.fileType !== 'image' || !this.previewCanvas) return;

    const canvas = this.previewCanvas.nativeElement;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const img = new Image();
    img.onload = () => {
      const maxPreviewWidth = 800;
      const scale = img.width > maxPreviewWidth ? maxPreviewWidth / img.width : 1;
      canvas.width = img.width * scale;
      canvas.height = img.height * scale;

      ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
      this.drawWatermark(ctx, canvas.width, canvas.height);
      this.generateDownload();
    };
    img.src = this.fileUrl;
  }

  private drawWatermark(ctx: CanvasRenderingContext2D, width: number, height: number): void {
    ctx.save();
    ctx.globalAlpha = this.opacity;
    ctx.font = `${this.fontSize}px Arial`;
    ctx.fillStyle = this.color;
    ctx.textBaseline = 'middle';
    ctx.textAlign = 'center';

    const x = this.dragX * width;
    const y = this.dragY * height;

    // Translate to position and apply rotation
    ctx.translate(x, y);
    ctx.rotate(this.rotation * Math.PI / 180);

    // Text shadow for visibility
    ctx.strokeStyle = 'rgba(0,0,0,0.5)';
    ctx.lineWidth = 2;
    ctx.strokeText(this.watermarkText, 0, 0);

    // Main text
    ctx.fillText(this.watermarkText, 0, 0);
    ctx.restore();
  }

  private generateDownload(): void {
    if (this.fileType !== 'image') return;

    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement('canvas');
      const ctx = canvas.getContext('2d');
      if (!ctx) return;

      canvas.width = img.width;
      canvas.height = img.height;
      ctx.drawImage(img, 0, 0);

      // Scale font size proportionally for full-res output
      const previewScale = this.previewCanvas
        ? this.previewCanvas.nativeElement.width / img.width
        : 1;
      const fullFontSize = this.fontSize / (previewScale || 1);

      ctx.save();
      ctx.globalAlpha = this.opacity;
      ctx.font = `${fullFontSize}px Arial`;
      ctx.fillStyle = this.color;
      ctx.textBaseline = 'middle';
      ctx.textAlign = 'center';

      const x = this.dragX * canvas.width;
      const y = this.dragY * canvas.height;

      ctx.translate(x, y);
      ctx.rotate(this.rotation * Math.PI / 180);

      ctx.strokeStyle = 'rgba(0,0,0,0.5)';
      ctx.lineWidth = 2;
      ctx.strokeText(this.watermarkText, 0, 0);
      ctx.fillText(this.watermarkText, 0, 0);
      ctx.restore();

      canvas.toBlob((blob) => {
        if (blob) {
          if (this.downloadUrl) {
            URL.revokeObjectURL(this.downloadUrl);
          }
          this.downloadUrl = URL.createObjectURL(blob);
        }
      }, 'image/png');
    };
    img.src = this.fileUrl;
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

  reset(): void {
    if (this.fileUrl) {
      URL.revokeObjectURL(this.fileUrl);
    }
    if (this.downloadUrl) {
      URL.revokeObjectURL(this.downloadUrl);
    }
    this.file = null;
    this.fileType = null;
    this.fileUrl = '';
    this.watermarkText = 'Your Watermark';
    this.fontSize = 24;
    this.opacity = 0.5;
    this.color = '#ffffff';
    this.rotation = 0;
    this.dragX = 0.75;
    this.dragY = 0.85;
    this.downloadUrl = '';
  }
}
