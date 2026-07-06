import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ApiService } from '../../core';

const MAX_FILE_SIZE = 5 * 1024 * 1024; // 5MB

@Component({
  selector: 'app-upload',
  standalone: true,
  imports: [
    CommonModule,
    LucideDynamicIcon,
  ],
  template: `
    <div class="max-w-3xl mx-auto px-4 py-8">
      <h1 class="text-2xl font-bold mb-2">Upload TXT File</h1>
      <p class="text-[hsl(var(--muted-foreground))] mb-6">
        Upload a TXT file containing multiple-choice questions. The file must be a plain text (.txt) file under 5MB.
      </p>

      <!-- Drop Zone -->
      <div
        class="relative glass-card p-10 text-center transition-all cursor-pointer"
        [class.!border-[hsl(var(--primary))]]="isDragOver"
        [class.!shadow-[0_0_30px_rgba(120,60,255,0.2)]]="isDragOver"
        [class.!border-green-500]="selectedFile && !error"
        (dragover)="onDragOver($event)"
        (dragleave)="onDragLeave($event)"
        (drop)="onDrop($event)"
        (click)="fileInput.click()"
        (keydown.enter)="fileInput.click()"
        tabindex="0"
        role="button"
        aria-label="Drop zone for TXT file upload"
      >
        <input
          #fileInput
          type="file"
          accept=".txt,text/plain"
          class="hidden"
          (change)="onFileSelected($event)"
          aria-hidden="true"
        />

        @if (!selectedFile) {
          <div class="space-y-3">
            <div class="w-16 h-16 rounded-full bg-[hsl(var(--primary))]/10 flex items-center justify-center mx-auto">
              <svg lucideIcon="upload" [size]="32" class="text-[hsl(var(--primary))]" [class.animate-pulse]="isDragOver"></svg>
            </div>
            <p class="text-lg font-medium">
              Drag & drop your TXT file here
            </p>
            <p class="text-sm text-[hsl(var(--muted-foreground))]">or</p>
            <button
              class="px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium"
              (click)="$event.stopPropagation()"
            >
              Browse Files
            </button>
            <p class="text-xs text-[hsl(var(--muted-foreground))] mt-2">Only .txt files, max 5MB</p>
          </div>
        }

        @if (selectedFile && !error) {
          <div class="space-y-3">
            <div class="w-16 h-16 rounded-full bg-green-500/10 flex items-center justify-center mx-auto">
              <svg lucideIcon="file-text" [size]="32" class="text-green-400"></svg>
            </div>
            <p class="text-lg font-medium">{{ selectedFile.name }}</p>
            <p class="text-sm text-[hsl(var(--muted-foreground))]">
              {{ formatFileSize(selectedFile.size) }}
            </p>
            <button
              class="px-4 py-2 rounded-lg border border-red-500/30 text-red-400 hover:bg-red-500/10 transition-colors text-sm"
              (click)="removeFile($event)"
            >
              Remove
            </button>
          </div>
        }
      </div>

      <!-- Error Message -->
      @if (error) {
        <div class="mt-4 p-3 glass-card !border-red-500/30">
          <p class="text-sm text-red-400 flex items-center gap-2">
            <svg lucideIcon="circle-x" [size]="16"></svg>
            {{ error }}
          </p>
        </div>
      }

      <!-- File Preview -->
      @if (filePreview) {
        <div class="mt-6 glass-card p-4">
          <p class="text-sm font-medium mb-3">File Preview</p>
          <pre class="text-xs bg-[hsl(var(--secondary))] p-3 rounded-lg overflow-auto max-h-48 whitespace-pre-wrap text-[hsl(var(--muted-foreground))]">{{ filePreview }}</pre>
        </div>
      }

      <!-- Upload Progress -->
      @if (uploadProgress >= 0) {
        <div class="mt-6">
          <div class="flex justify-between text-sm text-[hsl(var(--muted-foreground))] mb-2">
            <span>Uploading...</span>
            <span>{{ uploadProgress }}%</span>
          </div>
          <div class="h-2 rounded-full bg-[hsl(var(--secondary))] overflow-hidden">
            <div
              class="h-full rounded-full bg-gradient-to-r from-[hsl(var(--primary))] to-purple-400 transition-all duration-300"
              [style.width.%]="uploadProgress"
            ></div>
          </div>
        </div>
      }

      <!-- Submit Button -->
      <div class="flex justify-end mt-6">
        <button
          class="glow-btn"
          [disabled]="!selectedFile || !!error || isUploading"
          [class.opacity-50]="!selectedFile || !!error || isUploading"
          [class.pointer-events-none]="!selectedFile || !!error || isUploading"
          (click)="startProcessing()"
        >
          @if (isUploading) {
            <svg lucideIcon="loader-2" [size]="18" class="animate-spin"></svg>
            Uploading...
          } @else {
            <svg lucideIcon="play" [size]="18"></svg>
            Start Processing
          }
        </button>
      </div>

      <!-- Upload Error -->
      @if (uploadError) {
        <div class="mt-4 p-3 glass-card !border-red-500/30">
          <p class="text-sm text-red-400 flex items-center gap-2">
            <svg lucideIcon="circle-x" [size]="16"></svg>
            {{ uploadError }}
          </p>
        </div>
      }
    </div>
  `,
})
export class UploadComponent implements OnInit {
  projectId = '';
  selectedFile: File | null = null;
  filePreview = '';
  isDragOver = false;
  error = '';
  uploadError = '';
  isUploading = false;
  uploadProgress = -1;

  constructor(
    private readonly route: ActivatedRoute,
    private readonly router: Router,
    private readonly api: ApiService,
  ) {}

  ngOnInit(): void {
    this.projectId = this.route.snapshot.paramMap.get('id') || '';
    if (!this.projectId) {
      this.router.navigate(['/projects/new']);
    }
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

  removeFile(event: Event): void {
    event.stopPropagation();
    this.selectedFile = null;
    this.filePreview = '';
    this.error = '';
  }

  formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  }

  startProcessing(): void {
    if (!this.selectedFile || this.isUploading) return;

    this.isUploading = true;
    this.uploadError = '';
    this.uploadProgress = 0;

    this.api.post<{ uploadUrl: string }>(`/projects/${this.projectId}/upload`).subscribe({
      next: (response) => {
        this.uploadToS3(response.uploadUrl);
      },
      error: (err) => {
        this.isUploading = false;
        this.uploadProgress = -1;
        this.uploadError = err?.error?.message || 'Failed to initiate upload. Please try again.';
      },
    });
  }

  private handleFile(file: File): void {
    this.error = '';
    this.filePreview = '';
    this.selectedFile = null;

    if (!file.name.toLowerCase().endsWith('.txt') && file.type !== 'text/plain') {
      this.error = 'Invalid file type. Please select a .txt file.';
      return;
    }

    if (file.size > MAX_FILE_SIZE) {
      this.error = `File is too large (${this.formatFileSize(file.size)}). Maximum size is 5MB.`;
      return;
    }

    if (file.size === 0) {
      this.error = 'File is empty. Please select a file with content.';
      return;
    }

    this.selectedFile = file;
    this.readFilePreview(file);
  }

  private readFilePreview(file: File): void {
    const reader = new FileReader();
    reader.onload = () => {
      const content = reader.result as string;
      const lines = content.split('\n').slice(0, 10);
      this.filePreview = lines.join('\n');
      if (content.split('\n').length > 10) {
        this.filePreview += '\n...';
      }
    };
    reader.readAsText(file.slice(0, 2048));
  }

  private uploadToS3(signedUrl: string): void {
    const file = this.selectedFile!;

    const xhr = new XMLHttpRequest();
    xhr.open('PUT', signedUrl, true);
    xhr.setRequestHeader('Content-Type', 'text/plain');
    xhr.setRequestHeader('ngsw-bypass', 'true');

    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) {
        this.uploadProgress = Math.round((event.loaded / event.total) * 100);
      }
    };

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        this.uploadProgress = 100;
        this.startPipeline();
      } else {
        this.isUploading = false;
        this.uploadProgress = -1;
        this.uploadError = `Upload failed with status ${xhr.status}. Please try again.`;
      }
    };

    xhr.onerror = () => {
      this.isUploading = false;
      this.uploadProgress = -1;
      this.uploadError = 'Upload failed. Please try again.';
    };

    xhr.send(file);
  }

  private startPipeline(): void {
    this.api.post(`/projects/${this.projectId}/start`).subscribe({
      next: () => {
        this.isUploading = false;
        this.router.navigate(['/projects', this.projectId, 'progress']);
      },
      error: (err) => {
        this.isUploading = false;
        this.uploadProgress = -1;
        this.uploadError = err?.error?.message || 'Failed to start processing. Please try again.';
      },
    });
  }
}
