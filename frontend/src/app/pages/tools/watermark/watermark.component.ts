import { Component, ElementRef, ViewChild, AfterViewInit } from '@angular/core';
import { RouterLink } from '@angular/router';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-watermark',
  standalone: true,
  imports: [RouterLink, FormsModule],
  styles: [`
    :host {
      display: block;
      background-color: #F8FAFC;
    }
    .about-card {
      background-color: #FFFFFF;
      border: 1px solid #E2E8F0;
      border-radius: 0.75rem;
    }
    .about-heading {
      color: #0F172A;
    }
    .about-muted {
      color: #475569;
    }
    .about-accent {
      color: #0D9488;
    }
    .about-accent-bg {
      background-color: #CCFBF1;
    }
    .about-nav {
      background-color: rgba(255, 255, 255, 0.85);
      border-bottom: 1px solid #E2E8F0;
      backdrop-filter: blur(12px);
    }
    .about-signin-btn {
      background-color: #0D9488;
      color: #FFFFFF;
      padding: 0.5rem 1.25rem;
      border-radius: 0.5rem;
      font-weight: 600;
      font-size: 0.875rem;
      text-decoration: none;
      transition: background-color 0.2s;
      cursor: pointer;
      border: none;
    }
    .about-signin-btn:hover {
      background-color: #0F766E;
    }
    .about-signin-btn:disabled {
      background-color: #94A3B8;
      cursor: not-allowed;
    }
    .drop-zone {
      border: 2px dashed #CBD5E1;
      border-radius: 0.75rem;
      padding: 3rem 2rem;
      text-align: center;
      cursor: pointer;
      transition: border-color 0.2s, background-color 0.2s;
    }
    .drop-zone:hover,
    .drop-zone.drag-over {
      border-color: #0D9488;
      background-color: #F0FDFA;
    }
    .preview-area {
      background-color: #1E293B;
      border-radius: 0.75rem;
      padding: 1rem;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 300px;
    }
    .preview-area canvas {
      max-width: 100%;
      max-height: 500px;
      border-radius: 0.5rem;
    }
    .control-group {
      margin-bottom: 1.25rem;
    }
    .control-label {
      display: block;
      font-size: 0.875rem;
      font-weight: 600;
      color: #334155;
      margin-bottom: 0.5rem;
    }
    .control-input {
      width: 100%;
      padding: 0.5rem 0.75rem;
      border: 1px solid #E2E8F0;
      border-radius: 0.5rem;
      font-size: 0.875rem;
      outline: none;
      transition: border-color 0.2s;
    }
    .control-input:focus {
      border-color: #0D9488;
    }
    .control-select {
      width: 100%;
      padding: 0.5rem 0.75rem;
      border: 1px solid #E2E8F0;
      border-radius: 0.5rem;
      font-size: 0.875rem;
      outline: none;
      background-color: #FFFFFF;
      cursor: pointer;
    }
    .control-select:focus {
      border-color: #0D9488;
    }
    .slider-container {
      display: flex;
      align-items: center;
      gap: 0.75rem;
    }
    .slider-container input[type="range"] {
      flex: 1;
      height: 6px;
      -webkit-appearance: none;
      appearance: none;
      background: #E2E8F0;
      border-radius: 3px;
      outline: none;
    }
    .slider-container input[type="range"]::-webkit-slider-thumb {
      -webkit-appearance: none;
      appearance: none;
      width: 18px;
      height: 18px;
      border-radius: 50%;
      background: #0D9488;
      cursor: pointer;
    }
    .slider-container input[type="range"]::-moz-range-thumb {
      width: 18px;
      height: 18px;
      border-radius: 50%;
      background: #0D9488;
      cursor: pointer;
      border: none;
    }
    .slider-value {
      font-size: 0.875rem;
      font-weight: 600;
      color: #0D9488;
      min-width: 3rem;
      text-align: right;
    }
    .color-picker-wrapper {
      display: flex;
      align-items: center;
      gap: 0.75rem;
    }
    .color-picker-wrapper input[type="color"] {
      width: 40px;
      height: 40px;
      border: 2px solid #E2E8F0;
      border-radius: 0.5rem;
      cursor: pointer;
      padding: 2px;
    }
    .badge-coming-soon {
      display: inline-block;
      background-color: #FEF3C7;
      color: #92400E;
      font-size: 0.75rem;
      font-weight: 600;
      padding: 0.25rem 0.5rem;
      border-radius: 0.25rem;
    }
    .btn-reset {
      background-color: transparent;
      border: 1px solid #E2E8F0;
      color: #475569;
      padding: 0.5rem 1.25rem;
      border-radius: 0.5rem;
      font-weight: 600;
      font-size: 0.875rem;
      cursor: pointer;
      transition: border-color 0.2s, color 0.2s;
    }
    .btn-reset:hover {
      border-color: #0D9488;
      color: #0D9488;
    }
  `],
  template: `
    <!-- Navbar -->
    <nav class="fixed top-0 left-0 right-0 z-50 flex items-center justify-between px-6 py-4 about-nav">
      <a routerLink="/" class="flex items-center gap-2 text-xl font-bold">
        <img src="logo.svg" alt="Indifferent" class="h-8">
      </a>
      <a routerLink="/login" class="about-signin-btn">Sign In</a>
    </nav>

    <main class="max-w-4xl mx-auto px-6 py-12 pt-24">
      <header class="text-center mb-12">
        <h1 class="text-4xl font-bold mb-4 about-heading">Add Watermark</h1>
        <p class="text-lg about-muted">
          Add text watermarks to your images and videos &mdash; processed entirely in your browser
        </p>
      </header>

      <!-- Upload Area -->
      @if (!file) {
        <section class="about-card p-8 mb-8">
          <div
            class="drop-zone"
            [class.drag-over]="isDragOver"
            (click)="fileInput.click()"
            (dragover)="onDragOver($event)"
            (dragleave)="onDragLeave($event)"
            (drop)="onDrop($event)"
          >
            <div class="mb-3">
              <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="#94A3B8" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="mx-auto">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                <polyline points="17 8 12 3 7 8"/>
                <line x1="12" y1="3" x2="12" y2="15"/>
              </svg>
            </div>
            <p class="font-semibold about-heading mb-1">Drop your file here or click to browse</p>
            <p class="text-sm about-muted">Supports JPG, PNG, WebP, MP4, WebM</p>
          </div>
          <input
            #fileInput
            type="file"
            accept="image/jpeg,image/png,image/webp,video/mp4,video/webm"
            (change)="onFileSelected($event)"
            class="hidden"
          />
        </section>
      }

      <!-- Video Coming Soon -->
      @if (file && fileType === 'video') {
        <section class="about-card p-8 mb-8 text-center">
          <div class="mb-4">
            <span class="badge-coming-soon">Coming Soon</span>
          </div>
          <h2 class="text-xl font-semibold about-heading mb-2">Video Watermark</h2>
          <p class="about-muted mb-6">
            Video watermarking is coming in a future update. For now, please use an image file (JPG, PNG, or WebP).
          </p>
          <button class="btn-reset" (click)="reset()">Upload a Different File</button>
        </section>
      }

      <!-- Image Watermark Editor -->
      @if (file && fileType === 'image') {
        <!-- Controls -->
        <section class="about-card p-8 mb-8">
          <h2 class="text-xl font-semibold about-heading mb-6">Watermark Settings</h2>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <!-- Left column -->
            <div>
              <div class="control-group">
                <label for="watermarkText" class="control-label">Watermark Text</label>
                <input
                  id="watermarkText"
                  type="text"
                  class="control-input"
                  [(ngModel)]="watermarkText"
                  (ngModelChange)="updatePreview()"
                  placeholder="Enter watermark text"
                />
              </div>

              <div class="control-group">
                <label for="position" class="control-label">Position</label>
                <select
                  id="position"
                  class="control-select"
                  [(ngModel)]="position"
                  (ngModelChange)="updatePreview()"
                >
                  @for (pos of positions; track pos.value) {
                    <option [value]="pos.value">{{ pos.label }}</option>
                  }
                </select>
              </div>

              <div class="control-group">
                <label class="control-label">Color</label>
                <div class="color-picker-wrapper">
                  <input
                    type="color"
                    [(ngModel)]="color"
                    (ngModelChange)="updatePreview()"
                  />
                  <span class="text-sm about-muted">{{ color }}</span>
                </div>
              </div>
            </div>

            <!-- Right column -->
            <div>
              <div class="control-group">
                <label class="control-label">Font Size: {{ fontSize }}px</label>
                <div class="slider-container">
                  <input
                    type="range"
                    min="12"
                    max="72"
                    step="1"
                    [(ngModel)]="fontSize"
                    (ngModelChange)="updatePreview()"
                  />
                  <span class="slider-value">{{ fontSize }}px</span>
                </div>
              </div>

              <div class="control-group">
                <label class="control-label">Opacity: {{ (opacity * 100).toFixed(0) }}%</label>
                <div class="slider-container">
                  <input
                    type="range"
                    min="0.1"
                    max="1"
                    step="0.05"
                    [(ngModel)]="opacity"
                    (ngModelChange)="updatePreview()"
                  />
                  <span class="slider-value">{{ (opacity * 100).toFixed(0) }}%</span>
                </div>
              </div>
            </div>
          </div>
        </section>

        <!-- Preview -->
        <section class="about-card p-8 mb-8">
          <h2 class="text-xl font-semibold about-heading mb-4">Preview</h2>
          <div class="preview-area">
            <canvas #previewCanvas></canvas>
          </div>
        </section>

        <!-- Actions -->
        <section class="flex items-center justify-center gap-4 mb-8">
          <button class="btn-reset" (click)="reset()">Start Over</button>
          <button
            class="about-signin-btn"
            (click)="download()"
            [disabled]="!downloadUrl"
          >
            Download Watermarked Image
          </button>
        </section>
      }

      <!-- Footer -->
      <footer class="mt-12 pt-8 border-t border-gray-200">
        <div class="flex flex-wrap justify-center gap-6 text-sm about-muted mb-4">
          <a routerLink="/" class="hover:underline about-accent">Home</a>
          <a routerLink="/about" class="hover:underline about-accent">About</a>
          <a routerLink="/contact" class="hover:underline about-accent">Contact</a>
          <a routerLink="/privacy" class="hover:underline about-accent">Privacy Policy</a>
          <a routerLink="/terms" class="hover:underline about-accent">Terms of Service</a>
        </div>
        <p class="text-center text-xs about-muted">&copy; 2025 Indifferent. All rights reserved.</p>
      </footer>
    </main>
  `,
})
export class WatermarkComponent implements AfterViewInit {
  @ViewChild('previewCanvas') previewCanvas!: ElementRef<HTMLCanvasElement>;

  file: File | null = null;
  fileType: 'image' | 'video' | null = null;
  fileUrl = '';

  watermarkText = 'Your Watermark';
  position = 'bottom-right';
  fontSize = 24;
  opacity = 0.5;
  color = '#ffffff';

  isDragOver = false;
  downloadUrl = '';

  positions = [
    { value: 'top-left', label: 'Top Left' },
    { value: 'top-center', label: 'Top Center' },
    { value: 'top-right', label: 'Top Right' },
    { value: 'middle-left', label: 'Middle Left' },
    { value: 'center', label: 'Center' },
    { value: 'middle-right', label: 'Middle Right' },
    { value: 'bottom-left', label: 'Bottom Left' },
    { value: 'bottom-center', label: 'Bottom Center' },
    { value: 'bottom-right', label: 'Bottom Right' },
  ];

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
      // Delay preview rendering to allow the canvas to mount
      setTimeout(() => this.updatePreview(), 100);
    } else if (videoTypes.includes(file.type)) {
      this.file = file;
      this.fileType = 'video';
      this.fileUrl = URL.createObjectURL(file);
    }
  }

  updatePreview(): void {
    if (this.fileType !== 'image' || !this.previewCanvas) return;

    const canvas = this.previewCanvas.nativeElement;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const img = new Image();
    img.onload = () => {
      // Scale down for preview if image is very large
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

    const { x, y } = this.getTextPosition(width, height, ctx);

    // Text shadow for visibility
    ctx.strokeStyle = 'rgba(0,0,0,0.5)';
    ctx.lineWidth = 2;
    ctx.strokeText(this.watermarkText, x, y);

    // Main text
    ctx.fillText(this.watermarkText, x, y);
    ctx.restore();
  }

  private getTextPosition(canvasWidth: number, canvasHeight: number, ctx: CanvasRenderingContext2D): { x: number; y: number } {
    const metrics = ctx.measureText(this.watermarkText);
    const textWidth = metrics.width;
    const padding = 20;

    let x = padding;
    let y = padding + this.fontSize / 2;

    switch (this.position) {
      case 'top-left':
        x = padding;
        y = padding + this.fontSize / 2;
        break;
      case 'top-center':
        x = (canvasWidth - textWidth) / 2;
        y = padding + this.fontSize / 2;
        break;
      case 'top-right':
        x = canvasWidth - textWidth - padding;
        y = padding + this.fontSize / 2;
        break;
      case 'middle-left':
        x = padding;
        y = canvasHeight / 2;
        break;
      case 'center':
        x = (canvasWidth - textWidth) / 2;
        y = canvasHeight / 2;
        break;
      case 'middle-right':
        x = canvasWidth - textWidth - padding;
        y = canvasHeight / 2;
        break;
      case 'bottom-left':
        x = padding;
        y = canvasHeight - padding - this.fontSize / 2;
        break;
      case 'bottom-center':
        x = (canvasWidth - textWidth) / 2;
        y = canvasHeight - padding - this.fontSize / 2;
        break;
      case 'bottom-right':
        x = canvasWidth - textWidth - padding;
        y = canvasHeight - padding - this.fontSize / 2;
        break;
    }

    return { x, y };
  }

  private generateDownload(): void {
    if (this.fileType !== 'image') return;

    // Generate full-resolution watermarked image for download
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

      const metrics = ctx.measureText(this.watermarkText);
      const textWidth = metrics.width;
      const padding = 20 / (previewScale || 1);

      let x = padding;
      let y = padding + fullFontSize / 2;

      switch (this.position) {
        case 'top-left':
          x = padding;
          y = padding + fullFontSize / 2;
          break;
        case 'top-center':
          x = (canvas.width - textWidth) / 2;
          y = padding + fullFontSize / 2;
          break;
        case 'top-right':
          x = canvas.width - textWidth - padding;
          y = padding + fullFontSize / 2;
          break;
        case 'middle-left':
          x = padding;
          y = canvas.height / 2;
          break;
        case 'center':
          x = (canvas.width - textWidth) / 2;
          y = canvas.height / 2;
          break;
        case 'middle-right':
          x = canvas.width - textWidth - padding;
          y = canvas.height / 2;
          break;
        case 'bottom-left':
          x = padding;
          y = canvas.height - padding - fullFontSize / 2;
          break;
        case 'bottom-center':
          x = (canvas.width - textWidth) / 2;
          y = canvas.height - padding - fullFontSize / 2;
          break;
        case 'bottom-right':
          x = canvas.width - textWidth - padding;
          y = canvas.height - padding - fullFontSize / 2;
          break;
      }

      ctx.strokeStyle = 'rgba(0,0,0,0.5)';
      ctx.lineWidth = 2;
      ctx.strokeText(this.watermarkText, x, y);
      ctx.fillText(this.watermarkText, x, y);
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
    this.position = 'bottom-right';
    this.fontSize = 24;
    this.opacity = 0.5;
    this.color = '#ffffff';
    this.downloadUrl = '';
  }
}
