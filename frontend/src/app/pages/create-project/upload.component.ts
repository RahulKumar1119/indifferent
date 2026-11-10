import { Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ApiService } from '../../core';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { StepIndicatorComponent } from '../../shared/components/step-indicator/step-indicator.component';
import { FORMAT_GUIDE_PATH, SAMPLE_FILE_PATH, STORY_PATH } from '../progress/failure-message';
import { QUIZ_STEPS } from './quiz-steps';

const MAX_FILE_SIZE = 5 * 1024 * 1024; // 5MB

@Component({
  selector: 'app-upload',
  standalone: true,
  imports: [CommonModule, LucideDynamicIcon, RouterLink, NavbarComponent, StepIndicatorComponent],
  styles: [`
    :host { display: block; }
    .narrow { width: min(48rem, 100%); }
    .back { display: inline-flex; align-items: center; min-height: 44px; font-size: var(--type-sm); }
    .hint h2, .sec-title { margin: 0; font-size: var(--type-md); font-weight: 600; }
    .example { margin: var(--space-sm) 0 0; padding: var(--space-sm) var(--space-md); overflow-x: auto; border-radius: var(--radius-control); background: var(--app-page); border: 1px solid var(--color-rule); font-size: var(--type-sm); line-height: 1.6; color: var(--color-ink); }
    .hint p { margin: var(--space-sm) 0 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .links { display: flex; flex-wrap: wrap; gap: 0 var(--space-lg); }
    .links a, .alt a { display: inline-flex; align-items: center; min-height: 44px; font-weight: 600; }
    .hint .alt { margin-top: var(--space-2xs); padding-top: var(--space-2xs); border-top: 1px solid var(--color-rule); }
    .zone { display: flex; flex-direction: column; align-items: center; gap: var(--space-xs); padding: var(--space-xl) var(--space-md); text-align: center; border: 2px dashed var(--color-muted); cursor: pointer; transition: border-color var(--dur-short) var(--ease-out), background-color var(--dur-short) var(--ease-out); }
    .zone:hover, .zone.over { border-color: var(--color-accent); background: color-mix(in oklab, var(--color-accent) 6%, var(--app-card)); }
    .zone:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .zone .big { margin: 0; font-size: var(--type-md); font-weight: 600; }
    .zone .ui-hint { margin: 0; }
    .fake { pointer-events: none; }
    .file { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-sm); }
    .file .meta { flex: 1; min-width: 0; }
    .file-actions { display: flex; flex-wrap: wrap; gap: var(--space-xs); }
    @media (max-width: 39.99rem) { .file-actions { flex-basis: 100%; } }
    .file .name { display: block; font-weight: 600; overflow-wrap: anywhere; }
    .peek { margin: var(--space-sm) 0 0; padding: var(--space-sm); max-height: 12rem; overflow: auto; white-space: pre-wrap; border-radius: var(--radius-control); background: var(--app-page); border: 1px solid var(--color-rule); font-size: var(--type-xs); color: var(--color-ink-2); }
    .row-between { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-sm); }
    .shot { display: block; width: 100%; height: auto; margin-top: var(--space-md); border: 1px solid var(--color-rule); border-radius: var(--radius-control); }
    .bar-head { display: flex; justify-content: space-between; font-size: var(--type-sm); color: var(--color-ink-2); }
    .bar { margin-top: var(--space-xs); height: 0.5rem; border-radius: var(--radius-pill); background: var(--color-paper-3); overflow: hidden; }
    .bar > span { display: block; height: 100%; background: var(--color-accent); transition: width var(--dur-short) var(--ease-out); }
    .actions { display: flex; justify-content: flex-end; }
    @media (max-width: 39.99rem) { .actions .ui-btn { width: 100%; } }
    .ui-alert { margin: 0; display: flex; align-items: center; gap: var(--space-xs); }
    @media (prefers-reduced-motion: reduce) { .zone, .bar > span { transition: none; } }
  `],
  template: `
    <div class="ui-app">
      <app-navbar />
      <main id="main" class="ui-app-main narrow hm">
        <div>
          <a [routerLink]="['/projects', projectId]" class="ui-link back">Back to the project</a>
          <h1 class="ui-h1">Add your questions</h1>
          <p class="ui-lead">Upload a plain text (.txt) file of multiple-choice questions, up to 5 MB. We check it before anything renders.</p>
        </div>
        <app-step-indicator [steps]="steps" [current]="1" label="Quiz video steps" />

        <!-- What a working file looks like -->
        <section class="ui-card hint" aria-labelledby="format-hint-title">
          <h2 id="format-hint-title">What a working file looks like</h2>
          <pre class="example" tabindex="0" aria-label="Example question"><code>1. What is the capital of France?
A. London
B. Paris
C. Berlin
D. Madrid
Answer: B</code></pre>
          <p>Number each question, list lettered options and add an Answer line.</p>
          <p class="links">
            <a [routerLink]="formatGuidePath" class="ui-link">Read the format guide</a>
            <a [href]="sampleFilePath" download="quiz-sample.txt" class="ui-link">Download a sample file</a>
          </p>
          <p class="alt" data-testid="story-alt">Have a story or a script instead? <a [routerLink]="storyPath" class="ui-link">Use Story video</a></p>
        </section>

        <input #fileInput type="file" accept=".txt,text/plain" hidden (change)="onFileSelected($event); fileInput.value = ''" aria-hidden="true" tabindex="-1" />

        @if (!selectedFile) {
          <!-- The whole zone is the control: it opens the file picker by click, Enter or Space, and takes a dropped file. -->
          <div
            class="ui-card zone"
            [class.over]="isDragOver"
            (dragover)="onDragOver($event)"
            (dragleave)="onDragLeave($event)"
            (drop)="onDrop($event)"
            (click)="fileInput.click()"
            (keydown.enter)="fileInput.click()"
            (keydown.space)="$event.preventDefault(); fileInput.click()"
            tabindex="0"
            role="button"
            aria-label="Drop zone for TXT file upload. Press Enter to browse files."
            aria-describedby="zone-rules"
          >
            <span class="ui-chip" aria-hidden="true"><svg lucideIcon="upload" [size]="18"></svg></span>
            <p class="big">Drag and drop your TXT file here</p>
            <span class="ui-btn ui-btn--secondary fake" aria-hidden="true">Browse files</span>
            <p id="zone-rules" class="ui-hint">Only .txt files, up to 5 MB</p>
          </div>
        } @else {
          <section class="ui-card" aria-labelledby="file-title">
            <h2 id="file-title" class="sec-title">Your file</h2>
            <div class="file" style="margin-top: var(--space-sm)">
              <span class="ui-chip" aria-hidden="true"><svg lucideIcon="file-text" [size]="18"></svg></span>
              <span class="meta">
                <span class="name" data-testid="file-name">{{ selectedFile.name }}</span>
                <span class="ui-hint">{{ formatFileSize(selectedFile.size) }}</span>
              </span>
              <span class="file-actions">
                <button type="button" class="ui-btn ui-btn--secondary" (click)="fileInput.click()" [disabled]="isUploading">Choose another</button>
                <button type="button" class="ui-btn ui-btn--quiet" (click)="removeFile($event)" [disabled]="isUploading">Remove</button>
              </span>
            </div>
            @if (filePreview) {
              <pre class="peek" tabindex="0" aria-label="First lines of your file">{{ filePreview }}</pre>
            }
          </section>

          <section class="ui-card" aria-labelledby="preview-title">
            <div class="row-between">
              <div>
                <h2 id="preview-title" class="sec-title">First question preview</h2>
                <p class="ui-card-sub">Uses this project's design and options. No render is started.</p>
              </div>
              <button type="button" class="ui-btn ui-btn--secondary" [disabled]="previewLoading || !fileText" [attr.aria-busy]="previewLoading" (click)="previewFirstQuestion()">
                {{ previewLoading ? 'Rendering…' : 'Preview first question' }}
              </button>
            </div>
            @if (previewError) {
              <p class="ui-alert ui-alert--bad" role="alert" style="margin-top: var(--space-sm)">{{ previewError }}</p>
            }
            @if (previewUrl) {
              <img [src]="previewUrl" alt="Preview of the first question rendered with this project's design" class="shot" />
            }
          </section>
        }

        @if (error) {
          <p class="ui-alert ui-alert--bad" role="alert"><svg lucideIcon="circle-x" [size]="16" aria-hidden="true"></svg>{{ error }}</p>
        }

        @if (uploadProgress >= 0) {
          <div>
            <div class="bar-head"><span>Uploading…</span><span>{{ uploadProgress }}%</span></div>
            <div class="bar" role="progressbar" aria-label="Upload progress" aria-valuemin="0" aria-valuemax="100" [attr.aria-valuenow]="uploadProgress">
              <span [style.width.%]="uploadProgress"></span>
            </div>
          </div>
        }

        @if (uploadError) {
          <p class="ui-alert ui-alert--bad" role="alert"><svg lucideIcon="circle-x" [size]="16" aria-hidden="true"></svg>{{ uploadError }}</p>
        }

        <div class="actions">
          <button type="button" class="ui-btn ui-btn--primary" [disabled]="!selectedFile || !!error || isUploading" [attr.aria-busy]="isUploading" (click)="startProcessing()">
            @if (isUploading) {
              <svg lucideIcon="loader-2" [size]="18" class="ui-spin" aria-hidden="true"></svg>
              Uploading…
            } @else {
              <svg lucideIcon="play" [size]="18" aria-hidden="true"></svg>
              Start processing
            }
          </button>
        </div>
      </main>
    </div>
  `,
})
export class UploadComponent implements OnInit, OnDestroy {
  readonly steps = QUIZ_STEPS;
  readonly formatGuidePath = FORMAT_GUIDE_PATH;
  readonly sampleFilePath = SAMPLE_FILE_PATH;
  readonly storyPath = STORY_PATH;
  projectId = '';
  selectedFile: File | null = null;
  filePreview = '';
  fileText = '';
  previewUrl = '';
  previewLoading = false;
  previewError = '';
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
    this.fileText = '';
    this.previewError = '';
    this.revokePreview();
    this.error = '';
  }

  ngOnDestroy(): void {
    this.revokePreview();
  }

  previewFirstQuestion(): void {
    if (!this.fileText || this.previewLoading) return;
    this.previewLoading = true;
    this.previewError = '';
    this.revokePreview();

    this.api.postBlob(`/projects/${this.projectId}/slide-preview`, { text: this.fileText }).subscribe({
      next: (blob) => {
        this.previewLoading = false;
        this.previewUrl = URL.createObjectURL(blob);
      },
      error: () => {
        this.previewLoading = false;
        this.previewError = 'Could not render the first question. Check the file format and try again.';
      },
    });
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

    this.api.post<{ uploadUrl: string }>(`/projects/${this.projectId}/upload`, { size: this.selectedFile.size }).subscribe({
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
    this.fileText = '';
    this.previewError = '';
    this.revokePreview();
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
      this.fileText = content;
      const lines = content.split('\n').slice(0, 10);
      this.filePreview = lines.join('\n');
      if (content.split('\n').length > 10) {
        this.filePreview += '\n...';
      }
    };
    reader.readAsText(file);
  }

  private revokePreview(): void {
    if (this.previewUrl) {
      URL.revokeObjectURL(this.previewUrl);
      this.previewUrl = '';
    }
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
