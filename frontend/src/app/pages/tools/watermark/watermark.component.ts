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
      min-height: 100vh;
      background: linear-gradient(135deg, #0f172a 0%, #1e293b 50%, #0f172a 100%);
    }
    .editor-nav {
      background: rgba(15, 23, 42, 0.8);
      border-bottom: 1px solid rgba(255, 255, 255, 0.08);
      backdrop-filter: blur(20px);
    }
    .editor-container {
      display: grid;
      grid-template-columns: 1fr 320px;
      gap: 1.5rem;
      max-width: 1400px;
      margin: 0 auto;
      padding: 1.5rem;
      min-height: calc(100vh - 140px);
    }
    @media (max-width: 768px) {
      .editor-container {
        grid-template-columns: 1fr;
      }
    }
    .canvas-area {
      background: rgba(30, 41, 59, 0.5);
      border: 1px solid rgba(255, 255, 255, 0.06);
      border-radius: 1rem;
      display: flex;
      align-items: center;
      justify-content: center;
      position: relative;
      overflow: hidden;
      cursor: crosshair;
      min-height: 400px;
    }
    .canvas-area canvas {
      max-width: 100%;
      max-height: 70vh;
      border-radius: 0.5rem;
    }
    .controls-panel {
      background: rgba(30, 41, 59, 0.6);
      border: 1px solid rgba(255, 255, 255, 0.08);
      border-radius: 1rem;
      backdrop-filter: blur(20px);
      padding: 1.5rem;
      height: fit-content;
      position: sticky;
      top: 5rem;
    }
    .panel-title {
      color: #f1f5f9;
      font-size: 1.125rem;
      font-weight: 700;
      margin-bottom: 1.5rem;
    }
    .control-group {
      margin-bottom: 1.25rem;
    }
    .control-label {
      display: block;
      font-size: 0.75rem;
      font-weight: 600;
      color: #94a3b8;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      margin-bottom: 0.5rem;
    }
    .control-input {
      width: 100%;
      padding: 0.625rem 0.875rem;
      background: rgba(15, 23, 42, 0.6);
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 0.5rem;
      font-size: 0.875rem;
      color: #f1f5f9;
      outline: none;
      transition: border-color 0.2s;
    }
    .control-input:focus {
      border-color: #0d9488;
    }
    .control-input::placeholder {
      color: #64748b;
    }
    .slider-row {
      display: flex;
      align-items: center;
      gap: 0.75rem;
    }
    .slider-row input[type="range"] {
      flex: 1;
      height: 4px;
      -webkit-appearance: none;
      appearance: none;
      background: rgba(255, 255, 255, 0.1);
      border-radius: 2px;
      outline: none;
    }
    .slider-row input[type="range"]::-webkit-slider-thumb {
      -webkit-appearance: none;
      appearance: none;
      width: 16px;
      height: 16px;
      border-radius: 50%;
      background: #0d9488;
      cursor: pointer;
      box-shadow: 0 0 8px rgba(13, 148, 136, 0.4);
    }
    .slider-row input[type="range"]::-moz-range-thumb {
      width: 16px;
      height: 16px;
      border-radius: 50%;
      background: #0d9488;
      cursor: pointer;
      border: none;
      box-shadow: 0 0 8px rgba(13, 148, 136, 0.4);
    }
    .slider-value {
      font-size: 0.75rem;
      font-weight: 600;
      color: #0d9488;
      min-width: 2.5rem;
      text-align: right;
    }
    .color-row {
      display: flex;
      align-items: center;
      gap: 0.75rem;
    }
    .color-row input[type="color"] {
      width: 36px;
      height: 36px;
      border: 2px solid rgba(255, 255, 255, 0.1);
      border-radius: 0.5rem;
      cursor: pointer;
      padding: 2px;
      background: transparent;
    }
    .color-hex {
      font-size: 0.75rem;
      color: #64748b;
      font-family: monospace;
    }
    .btn-primary {
      width: 100%;
      padding: 0.75rem;
      background: linear-gradient(135deg, #0d9488, #0f766e);
      color: #ffffff;
      border: none;
      border-radius: 0.5rem;
      font-weight: 600;
      font-size: 0.875rem;
      cursor: pointer;
      transition: opacity 0.2s, transform 0.1s;
    }
    .btn-primary:hover {
      opacity: 0.9;
    }
    .btn-primary:active {
      transform: scale(0.98);
    }
    .btn-primary:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }
    .btn-secondary {
      width: 100%;
      padding: 0.625rem;
      background: transparent;
      color: #94a3b8;
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 0.5rem;
      font-weight: 600;
      font-size: 0.8rem;
      cursor: pointer;
      transition: border-color 0.2s, color 0.2s;
    }
    .btn-secondary:hover {
      border-color: #0d9488;
      color: #0d9488;
    }
    .drop-zone {
      border: 2px dashed rgba(255, 255, 255, 0.15);
      border-radius: 1rem;
      padding: 4rem 2rem;
      text-align: center;
      cursor: pointer;
      transition: border-color 0.2s, background-color 0.2s;
      width: 100%;
      max-width: 500px;
    }
    .drop-zone:hover,
    .drop-zone.drag-over {
      border-color: #0d9488;
      background: rgba(13, 148, 136, 0.05);
    }
    .drop-title {
      color: #e2e8f0;
      font-weight: 600;
      font-size: 1rem;
      margin-bottom: 0.5rem;
    }
    .drop-subtitle {
      color: #64748b;
      font-size: 0.875rem;
    }
    .drag-hint {
      position: absolute;
      bottom: 1rem;
      left: 50%;
      transform: translateX(-50%);
      background: rgba(13, 148, 136, 0.15);
      color: #5eead4;
      font-size: 0.7rem;
      padding: 0.35rem 0.75rem;
      border-radius: 1rem;
      pointer-events: none;
    }
    .divider {
      height: 1px;
      background: rgba(255, 255, 255, 0.06);
      margin: 1.25rem 0;
    }
    .footer-dark {
      border-top: 1px solid rgba(255, 255, 255, 0.06);
      padding: 1.5rem;
      text-align: center;
    }
    .footer-dark a {
      color: #64748b;
      font-size: 0.8rem;
      text-decoration: none;
      margin: 0 0.75rem;
      transition: color 0.2s;
    }
    .footer-dark a:hover {
      color: #0d9488;
    }
    .footer-dark p {
      color: #475569;
      font-size: 0.7rem;
      margin-top: 0.75rem;
    }
  `],
  template: `
    <!-- Navbar -->
    <nav class="editor-nav" style="position: sticky; top: 0; z-index: 50; display: flex; align-items: center; justify-content: space-between; padding: 1rem 1.5rem;">
      <a routerLink="/" style="display: flex; align-items: center; gap: 0.5rem; text-decoration: none;">
        <img src="logo.svg" alt="Indifferent" style="height: 2rem;">
      </a>
      <a routerLink="/login" class="btn-primary" style="width: auto; padding: 0.5rem 1.25rem;">Sign In</a>
    </nav>

    <!-- No File: Full-width drop zone -->
    <div *ngIf="!file" style="display: flex; align-items: center; justify-content: center; min-height: calc(100vh - 140px);">
      <div
        class="drop-zone"
        [class.drag-over]="isDragOver"
        (click)="fileInput.click()"
        (dragover)="onDragOver($event)"
        (dragleave)="onDragLeave($event)"
        (drop)="onDrop($event)"
      >
        <div style="margin-bottom: 1rem;">
          <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="#64748b" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" style="margin: 0 auto; display: block;">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
            <polyline points="17 8 12 3 7 8"/>
            <line x1="12" y1="3" x2="12" y2="15"/>
          </svg>
        </div>
        <p class="drop-title">Drop your file here or click to browse</p>
        <p class="drop-subtitle">Supports JPG, PNG, WebP, MP4, WebM</p>
      </div>
      <input
        #fileInput
        type="file"
        accept="image/jpeg,image/png,image/webp,video/mp4,video/webm"
        (change)="onFileSelected($event)"
        style="display: none;"
      />
    </div>

    <!-- Video Coming Soon -->
    <div *ngIf="file && fileType === 'video'" style="display: flex; align-items: center; justify-content: center; min-height: calc(100vh - 140px);">
      <div style="text-align: center; color: #e2e8f0;">
        <p style="display: inline-block; background: #fef3c7; color: #92400e; font-size: 0.75rem; font-weight: 600; padding: 0.25rem 0.5rem; border-radius: 0.25rem; margin-bottom: 1rem;">Coming Soon</p>
        <h2 style="font-size: 1.25rem; font-weight: 600; margin-bottom: 0.5rem;">Video Watermark</h2>
        <p style="color: #94a3b8; margin-bottom: 1.5rem;">Video watermarking is coming in a future update. Please use an image file.</p>
        <button class="btn-secondary" style="width: auto; padding: 0.5rem 1.5rem;" (click)="reset()">Upload a Different File</button>
      </div>
    </div>

    <!-- Image Editor: Canvas + Controls -->
    <div *ngIf="file && fileType === 'image'" class="editor-container">
      <!-- Canvas Area -->
      <div
        class="canvas-area"
        (mousedown)="onCanvasMouseDown($event)"
        (mousemove)="onCanvasMouseMove($event)"
        (mouseup)="onCanvasMouseUp()"
        (mouseleave)="onCanvasMouseUp()"
      >
        <canvas #previewCanvas></canvas>
        <span class="drag-hint">Click and drag to reposition watermark</span>
      </div>

      <!-- Controls Panel -->
      <div class="controls-panel">
        <h3 class="panel-title">Watermark Settings</h3>

        <!-- Text Input -->
        <div class="control-group">
          <label class="control-label">Text</label>
          <input
            type="text"
            class="control-input"
            [(ngModel)]="watermarkText"
            (ngModelChange)="updatePreview()"
            placeholder="Enter watermark text"
          />
        </div>

        <!-- Font Size -->
        <div class="control-group">
          <label class="control-label">Font Size</label>
          <div class="slider-row">
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

        <!-- Opacity -->
        <div class="control-group">
          <label class="control-label">Opacity</label>
          <div class="slider-row">
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

        <!-- Color -->
        <div class="control-group">
          <label class="control-label">Color</label>
          <div class="color-row">
            <input
              type="color"
              [(ngModel)]="color"
              (ngModelChange)="updatePreview()"
            />
            <span class="color-hex">{{ color }}</span>
          </div>
        </div>

        <!-- Rotation -->
        <div class="control-group">
          <label class="control-label">Rotation</label>
          <div class="slider-row">
            <input
              type="range"
              min="-180"
              max="180"
              step="1"
              [(ngModel)]="rotation"
              (ngModelChange)="updatePreview()"
            />
            <span class="slider-value">{{ rotation }}&deg;</span>
          </div>
        </div>

        <div class="divider"></div>

        <!-- Download -->
        <div class="control-group">
          <button
            class="btn-primary"
            (click)="download()"
            [disabled]="!downloadUrl"
          >
            Download Image
          </button>
        </div>

        <!-- Reset -->
        <div class="control-group" style="margin-bottom: 0;">
          <button class="btn-secondary" (click)="reset()">Reset</button>
        </div>
      </div>
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
    <footer class="footer-dark">
      <div>
        <a routerLink="/">Home</a>
        <a routerLink="/about">About</a>
        <a routerLink="/contact">Contact</a>
        <a routerLink="/privacy">Privacy</a>
        <a routerLink="/terms">Terms</a>
      </div>
      <p>&copy; 2025 Indifferent. All rights reserved.</p>
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
