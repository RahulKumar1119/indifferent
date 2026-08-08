import { Component, ElementRef, ViewChild, AfterViewInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-watermark',
  standalone: true,
  imports: [CommonModule, RouterLink, FormsModule],
  styles: [`
    :host {
      display: block;
      background: #f8fafc;
      min-height: 100vh;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    }
    .wm-nav {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 1rem 2rem;
      background: rgba(255,255,255,0.9);
      border-bottom: 1px solid #e2e8f0;
      backdrop-filter: blur(12px);
      position: sticky;
      top: 0;
      z-index: 100;
    }
    .wm-logo img {
      height: 2rem;
    }
    .wm-signin {
      background: #0d9488;
      color: #fff;
      padding: 0.5rem 1.25rem;
      border-radius: 0.5rem;
      font-weight: 600;
      font-size: 0.875rem;
      text-decoration: none;
    }
    .wm-signin:hover { background: #0f766e; }
    .wm-header {
      text-align: center;
      padding: 3rem 2rem 2rem;
    }
    .wm-header h1 {
      font-size: 2.25rem;
      font-weight: 800;
      color: #0f172a;
      margin: 0 0 0.75rem;
    }
    .wm-header p {
      color: #64748b;
      font-size: 1.1rem;
      margin: 0;
    }
    .wm-upload-wrapper {
      display: flex;
      justify-content: center;
      padding: 2rem;
    }
    .wm-dropzone {
      border: 2px dashed #cbd5e1;
      border-radius: 1rem;
      padding: 4rem 3rem;
      text-align: center;
      cursor: pointer;
      max-width: 500px;
      width: 100%;
      transition: border-color 0.2s, background 0.2s;
    }
    .wm-dropzone:hover, .wm-dropzone.active {
      border-color: #0d9488;
      background: #f0fdfa;
    }
    .wm-dropzone svg {
      display: block;
      margin: 0 auto 1.25rem;
    }
    .wm-dropzone h3 {
      color: #0f172a;
      font-size: 1.1rem;
      font-weight: 600;
      margin: 0 0 0.5rem;
    }
    .wm-dropzone p {
      color: #94a3b8;
      font-size: 0.875rem;
      margin: 0;
    }
    .wm-editor {
      display: grid;
      grid-template-columns: 1fr 300px;
      gap: 1.5rem;
      max-width: 1200px;
      margin: 0 auto;
      padding: 0 2rem 2rem;
    }
    @media (max-width: 768px) {
      .wm-editor { grid-template-columns: 1fr; }
    }
    .wm-canvas-wrapper {
      background: #f1f5f9;
      border: 1px solid #e2e8f0;
      border-radius: 0.75rem;
      display: flex;
      align-items: center;
      justify-content: center;
      position: relative;
      min-height: 400px;
      cursor: crosshair;
      overflow: hidden;
      padding: 1rem;
    }
    .wm-canvas-wrapper canvas {
      max-width: 100%;
      max-height: 60vh;
      border-radius: 0.5rem;
      box-shadow: 0 4px 20px rgba(0,0,0,0.08);
    }
    .wm-drag-label {
      position: absolute;
      bottom: 0.75rem;
      left: 50%;
      transform: translateX(-50%);
      background: rgba(13,148,136,0.1);
      color: #0d9488;
      font-size: 0.7rem;
      font-weight: 600;
      padding: 0.3rem 0.75rem;
      border-radius: 1rem;
      pointer-events: none;
    }
    .wm-controls {
      background: #ffffff;
      border: 1px solid #e2e8f0;
      border-radius: 0.75rem;
      padding: 1.5rem;
      height: fit-content;
      position: sticky;
      top: 5rem;
    }
    .wm-controls h3 {
      font-size: 1rem;
      font-weight: 700;
      color: #0f172a;
      margin: 0 0 1.5rem;
    }
    .wm-field {
      margin-bottom: 1.25rem;
    }
    .wm-field label {
      display: block;
      font-size: 0.75rem;
      font-weight: 600;
      color: #64748b;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      margin-bottom: 0.4rem;
    }
    .wm-field input[type="text"] {
      width: 100%;
      padding: 0.6rem 0.75rem;
      border: 1px solid #e2e8f0;
      border-radius: 0.5rem;
      font-size: 0.875rem;
      color: #0f172a;
      outline: none;
      box-sizing: border-box;
    }
    .wm-field input[type="text"]:focus {
      border-color: #0d9488;
    }
    .wm-slider-row {
      display: flex;
      align-items: center;
      gap: 0.75rem;
    }
    .wm-slider-row input[type="range"] {
      flex: 1;
      height: 5px;
      -webkit-appearance: none;
      appearance: none;
      background: #e2e8f0;
      border-radius: 3px;
      outline: none;
    }
    .wm-slider-row input[type="range"]::-webkit-slider-thumb {
      -webkit-appearance: none;
      width: 16px;
      height: 16px;
      border-radius: 50%;
      background: #0d9488;
      cursor: pointer;
    }
    .wm-slider-row input[type="range"]::-moz-range-thumb {
      width: 16px;
      height: 16px;
      border-radius: 50%;
      background: #0d9488;
      cursor: pointer;
      border: none;
    }
    .wm-slider-row span {
      font-size: 0.75rem;
      font-weight: 600;
      color: #0d9488;
      min-width: 2.5rem;
      text-align: right;
    }
    .wm-color-row {
      display: flex;
      align-items: center;
      gap: 0.75rem;
    }
    .wm-color-row input[type="color"] {
      width: 36px;
      height: 36px;
      border: 2px solid #e2e8f0;
      border-radius: 0.375rem;
      cursor: pointer;
      padding: 2px;
    }
    .wm-color-row span {
      font-size: 0.75rem;
      color: #94a3b8;
      font-family: monospace;
    }
    .wm-actions {
      display: flex;
      flex-direction: column;
      gap: 0.75rem;
      margin-top: 1.5rem;
      padding-top: 1.5rem;
      border-top: 1px solid #e2e8f0;
    }
    .wm-btn-primary {
      width: 100%;
      padding: 0.75rem;
      background: #0d9488;
      color: #fff;
      border: none;
      border-radius: 0.5rem;
      font-weight: 600;
      font-size: 0.875rem;
      cursor: pointer;
    }
    .wm-btn-primary:hover { background: #0f766e; }
    .wm-btn-primary:disabled { background: #cbd5e1; cursor: not-allowed; }
    .wm-btn-secondary {
      width: 100%;
      padding: 0.625rem;
      background: transparent;
      color: #64748b;
      border: 1px solid #e2e8f0;
      border-radius: 0.5rem;
      font-weight: 600;
      font-size: 0.8rem;
      cursor: pointer;
    }
    .wm-btn-secondary:hover { border-color: #0d9488; color: #0d9488; }
    .wm-coming-soon {
      text-align: center;
      padding: 4rem 2rem;
    }
    .wm-badge {
      display: inline-block;
      background: #fef3c7;
      color: #92400e;
      font-size: 0.75rem;
      font-weight: 600;
      padding: 0.25rem 0.75rem;
      border-radius: 1rem;
      margin-bottom: 1rem;
    }
    .wm-coming-soon h2 {
      color: #0f172a;
      font-size: 1.5rem;
      margin: 0 0 0.5rem;
    }
    .wm-coming-soon p {
      color: #64748b;
      margin: 0 0 1.5rem;
    }
    .wm-footer {
      text-align: center;
      padding: 2rem;
      border-top: 1px solid #e2e8f0;
      margin-top: 2rem;
    }
    .wm-footer a {
      color: #64748b;
      font-size: 0.8rem;
      text-decoration: none;
      margin: 0 0.5rem;
    }
    .wm-footer a:hover { color: #0d9488; }
    .wm-footer p {
      color: #94a3b8;
      font-size: 0.7rem;
      margin-top: 0.5rem;
    }
  `],
  template: `
    <!-- Navbar -->
    <nav class="wm-nav">
      <a routerLink="/" class="wm-logo"><img src="logo.svg" alt="Indifferent" /></a>
      <a routerLink="/login" class="wm-signin">Sign In</a>
    </nav>

    <!-- Page Header -->
    <div class="wm-header">
      <h1>Add Watermark</h1>
      <p>Add text watermarks to your images — free, private, processed in your browser</p>
    </div>

    <!-- Upload State -->
    <div *ngIf="!file" class="wm-upload-wrapper">
      <div
        class="wm-dropzone"
        [class.active]="isDragOver"
        (click)="fileInput.click()"
        (dragover)="onDragOver($event)"
        (dragleave)="onDragLeave($event)"
        (drop)="onDrop($event)"
      >
        <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="#94a3b8" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
          <polyline points="17 8 12 3 7 8"/>
          <line x1="12" y1="3" x2="12" y2="15"/>
        </svg>
        <h3>Drop your image here or click to upload</h3>
        <p>JPG, PNG, WebP — up to 20MB</p>
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
    <div *ngIf="file && fileType === 'image'" class="wm-editor">
      <div
        class="wm-canvas-wrapper"
        (mousedown)="onCanvasMouseDown($event)"
        (mousemove)="onCanvasMouseMove($event)"
        (mouseup)="onCanvasMouseUp()"
        (mouseleave)="onCanvasMouseUp()"
      >
        <canvas #previewCanvas></canvas>
        <span class="wm-drag-label">Drag to move watermark</span>
      </div>
      <div class="wm-controls">
        <h3>Settings</h3>
        <div class="wm-field">
          <label>Watermark Text</label>
          <input type="text" [(ngModel)]="watermarkText" (ngModelChange)="updatePreview()" />
        </div>
        <div class="wm-field">
          <label>Font Size</label>
          <div class="wm-slider-row">
            <input type="range" min="12" max="72" step="1" [(ngModel)]="fontSize" (ngModelChange)="updatePreview()" />
            <span>{{ fontSize }}px</span>
          </div>
        </div>
        <div class="wm-field">
          <label>Opacity</label>
          <div class="wm-slider-row">
            <input type="range" min="0.1" max="1" step="0.05" [(ngModel)]="opacity" (ngModelChange)="updatePreview()" />
            <span>{{ (opacity * 100).toFixed(0) }}%</span>
          </div>
        </div>
        <div class="wm-field">
          <label>Color</label>
          <div class="wm-color-row">
            <input type="color" [(ngModel)]="color" (ngModelChange)="updatePreview()" />
            <span>{{ color }}</span>
          </div>
        </div>
        <div class="wm-field">
          <label>Rotation</label>
          <div class="wm-slider-row">
            <input type="range" min="-180" max="180" step="1" [(ngModel)]="rotation" (ngModelChange)="updatePreview()" />
            <span>{{ rotation }}&deg;</span>
          </div>
        </div>
        <div class="wm-actions">
          <button class="wm-btn-primary" (click)="download()" [disabled]="!downloadUrl">Download</button>
          <button class="wm-btn-secondary" (click)="reset()">Reset</button>
        </div>
      </div>
    </div>

    <!-- Video Coming Soon -->
    <div *ngIf="file && fileType === 'video'" class="wm-coming-soon">
      <span class="wm-badge">Coming Soon</span>
      <h2>Video Watermark</h2>
      <p>Video support arriving soon. Please upload an image.</p>
      <button class="wm-btn-secondary" style="width: auto; display: inline-block; padding: 0.5rem 1.5rem;" (click)="reset()">Try Another File</button>
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

    <!-- Footer -->
    <footer class="wm-footer">
      <div>
        <a routerLink="/">Home</a> &middot;
        <a routerLink="/about">About</a> &middot;
        <a routerLink="/contact">Contact</a> &middot;
        <a routerLink="/privacy">Privacy</a> &middot;
        <a routerLink="/terms">Terms</a>
      </div>
      <p>&copy; 2025 Indifferent</p>
    </footer>
  `,
})
export class WatermarkComponent implements AfterViewInit {
  @ViewChild('previewCanvas') previewCanvas!: ElementRef<HTMLCanvasElement>;

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
