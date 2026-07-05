import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { HttpClient, HttpEventType } from '@angular/common/http';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatIconModule } from '@angular/material/icon';
import { ApiService } from '../../core';

const MAX_FILE_SIZE = 5 * 1024 * 1024; // 5MB

@Component({
  selector: 'app-upload',
  standalone: true,
  imports: [
    CommonModule,
    MatButtonModule,
    MatCardModule,
    MatProgressBarModule,
    MatIconModule,
  ],
  template: `
    <div class="max-w-3xl mx-auto px-4 py-8">
      <h1 class="text-2xl font-bold mb-2">Upload TXT File</h1>
      <p class="text-gray-600 dark:text-gray-400 mb-6">
        Upload a TXT file containing multiple-choice questions. The file must be a plain text (.txt) file under 5MB.
      </p>

      <!-- Drop Zone -->
      <div
        class="relative border-2 border-dashed rounded-lg p-8 text-center transition-all cursor-pointer"
        [class.border-gray-300]="!isDragOver && !selectedFile"
        [class.dark:border-gray-600]="!isDragOver && !selectedFile"
        [class.border-blue-500]="isDragOver"
        [class.bg-blue-50]="isDragOver"
        [class.dark:bg-blue-900/20]="isDragOver"
        [class.border-green-500]="selectedFile && !error"
        [class.bg-green-50]="selectedFile && !error"
        [class.dark:bg-green-900/20]="selectedFile && !error"
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
            <mat-icon class="text-4xl text-gray-400">cloud_upload</mat-icon>
            <p class="text-lg font-medium text-gray-700 dark:text-gray-300">
              Drag & drop your TXT file here
            </p>
            <p class="text-sm text-gray-500 dark:text-gray-400">or</p>
            <button mat-stroked-button color="primary" (click)="$event.stopPropagation()">
              Browse Files
            </button>
            <p class="text-xs text-gray-400 mt-2">Only .txt files, max 5MB</p>
          </div>
        }

        @if (selectedFile && !error) {
          <div class="space-y-2">
            <mat-icon class="text-4xl text-green-500">description</mat-icon>
            <p class="text-lg font-medium">{{ selectedFile.name }}</p>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ formatFileSize(selectedFile.size) }}
            </p>
            <button
              mat-stroked-button
              color="warn"
              (click)="removeFile($event)"
            >
              Remove
            </button>
          </div>
        }
      </div>

      <!-- Error Message -->
      @if (error) {
        <div class="mt-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
          <p class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
        </div>
      }

      <!-- File Preview -->
      @if (filePreview) {
        <mat-card class="mt-6" appearance="outlined">
          <mat-card-header>
            <mat-card-title class="text-sm font-medium">File Preview</mat-card-title>
          </mat-card-header>
          <mat-card-content>
            <pre class="mt-3 text-xs bg-gray-50 dark:bg-gray-800 p-3 rounded overflow-auto max-h-48 whitespace-pre-wrap">{{ filePreview }}</pre>
          </mat-card-content>
        </mat-card>
      }

      <!-- Upload Progress -->
      @if (uploadProgress >= 0) {
        <div class="mt-6">
          <div class="flex justify-between text-sm text-gray-600 dark:text-gray-400 mb-1">
            <span>Uploading...</span>
            <span>{{ uploadProgress }}%</span>
          </div>
          <mat-progress-bar mode="determinate" [value]="uploadProgress"></mat-progress-bar>
        </div>
      }

      <!-- Submit Button -->
      <div class="flex justify-end mt-6">
        <button
          mat-flat-button
          color="primary"
          [disabled]="!selectedFile || !!error || isUploading"
          (click)="startProcessing()"
        >
          @if (isUploading) {
            Uploading...
          } @else {
            Start Processing
          }
        </button>
      </div>

      <!-- Upload Error -->
      @if (uploadError) {
        <div class="mt-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
          <p class="text-sm text-red-600 dark:text-red-400">{{ uploadError }}</p>
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
    private readonly http: HttpClient,
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

    // Step 1: Get signed upload URL from API
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

    // Validate file type
    if (!file.name.toLowerCase().endsWith('.txt') && file.type !== 'text/plain') {
      this.error = 'Invalid file type. Please select a .txt file.';
      return;
    }

    // Validate file size
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
      // Show first 500 characters as preview
      const lines = content.split('\n').slice(0, 10);
      this.filePreview = lines.join('\n');
      if (content.split('\n').length > 10) {
        this.filePreview += '\n...';
      }
    };
    reader.readAsText(file.slice(0, 2048));
  }

  private uploadToS3(signedUrl: string): void {
    this.http
      .put(signedUrl, this.selectedFile, {
        headers: { 'Content-Type': 'text/plain' },
        reportProgress: true,
        observe: 'events',
      })
      .subscribe({
        next: (event) => {
          if (event.type === HttpEventType.UploadProgress && event.total) {
            this.uploadProgress = Math.round((event.loaded / event.total) * 100);
          }
          if (event.type === HttpEventType.Response) {
            this.isUploading = false;
            this.uploadProgress = 100;
            // Navigate to progress page
            this.router.navigate(['/projects', this.projectId, 'progress']);
          }
        },
        error: () => {
          this.isUploading = false;
          this.uploadProgress = -1;
          this.uploadError = 'Upload failed. Please try again.';
        },
      });
  }
}
