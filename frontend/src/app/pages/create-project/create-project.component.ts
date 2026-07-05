import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatStepperModule } from '@angular/material/stepper';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ApiService } from '../../core';
import { Project, CreateProjectRequest, Template, Voice } from '../../shared';

interface TemplateOption {
  value: Template;
  label: string;
  enabled: boolean;
}

@Component({
  selector: 'app-create-project',
  standalone: true,
  imports: [
    CommonModule,
    ReactiveFormsModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    MatButtonModule,
    MatCardModule,
    MatStepperModule,
    MatProgressSpinnerModule,
  ],
  template: `
    <div class="max-w-3xl mx-auto px-4 py-8">
      <h1 class="text-2xl font-bold mb-6">Create New Project</h1>

      <mat-stepper linear #stepper class="bg-transparent">
        <mat-step [stepControl]="projectForm" label="Configure">
          <form [formGroup]="projectForm" class="mt-6 space-y-6">
            <!-- Project Name -->
            <mat-form-field appearance="outline" class="w-full">
              <mat-label>Project Name</mat-label>
              <input matInput formControlName="name" placeholder="My Quiz Video" maxlength="100" />
              <mat-hint align="end">{{ projectForm.get('name')?.value?.length || 0 }}/100</mat-hint>
              @if (projectForm.get('name')?.hasError('required') && projectForm.get('name')?.touched) {
                <mat-error>Project name is required</mat-error>
              }
              @if (projectForm.get('name')?.hasError('maxlength')) {
                <mat-error>Maximum 100 characters</mat-error>
              }
            </mat-form-field>

            <!-- Template Selector -->
            <div>
              <label class="block text-sm font-medium mb-3">Template</label>
              <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
                @for (tpl of templates; track tpl.value) {
                  <div
                    class="relative rounded-lg border-2 p-4 cursor-pointer transition-all"
                    [class.border-blue-500]="projectForm.get('template')?.value === tpl.value && tpl.enabled"
                    [class.bg-blue-50]="projectForm.get('template')?.value === tpl.value && tpl.enabled"
                    [class.dark:bg-blue-900/20]="projectForm.get('template')?.value === tpl.value && tpl.enabled"
                    [class.border-gray-200]="projectForm.get('template')?.value !== tpl.value || !tpl.enabled"
                    [class.dark:border-gray-700]="projectForm.get('template')?.value !== tpl.value || !tpl.enabled"
                    [class.opacity-50]="!tpl.enabled"
                    [class.pointer-events-none]="!tpl.enabled"
                    (click)="tpl.enabled && selectTemplate(tpl.value)"
                    (keydown.enter)="tpl.enabled && selectTemplate(tpl.value)"
                    [attr.tabindex]="tpl.enabled ? 0 : -1"
                    [attr.role]="'radio'"
                    [attr.aria-checked]="projectForm.get('template')?.value === tpl.value"
                    [attr.aria-disabled]="!tpl.enabled"
                  >
                    <div class="text-sm font-medium">{{ tpl.label }}</div>
                    @if (!tpl.enabled) {
                      <span
                        class="absolute top-1 right-1 text-xs bg-gray-200 dark:bg-gray-600 text-gray-600 dark:text-gray-300 px-1.5 py-0.5 rounded"
                      >
                        Coming Soon
                      </span>
                    }
                  </div>
                }
              </div>
            </div>

            <!-- Voice Selector -->
            <mat-form-field appearance="outline" class="w-full">
              <mat-label>Narration Voice</mat-label>
              <mat-select formControlName="voice">
                @for (v of voices; track v) {
                  <mat-option [value]="v">{{ v }}</mat-option>
                }
              </mat-select>
              @if (projectForm.get('voice')?.hasError('required') && projectForm.get('voice')?.touched) {
                <mat-error>Voice selection is required</mat-error>
              }
            </mat-form-field>

            @if (projectForm.get('voice')?.value) {
              <p class="text-sm text-gray-600 dark:text-gray-400">
                Selected voice: <span class="font-medium">{{ projectForm.get('voice')?.value }}</span>
              </p>
            }

            <!-- Submit Button -->
            <div class="flex justify-end pt-4">
              <button
                mat-flat-button
                color="primary"
                [disabled]="projectForm.invalid || isSubmitting"
                (click)="createProject()"
              >
                @if (isSubmitting) {
                  <mat-spinner diameter="20" class="inline-block mr-2"></mat-spinner>
                }
                Next: Upload File
              </button>
            </div>

            @if (errorMessage) {
              <p class="text-red-600 dark:text-red-400 text-sm mt-2">{{ errorMessage }}</p>
            }
          </form>
        </mat-step>

        <mat-step label="Upload">
          <p class="mt-4 text-gray-600 dark:text-gray-400">
            Complete the configuration step first, then you'll be taken to the upload page.
          </p>
        </mat-step>
      </mat-stepper>
    </div>
  `,
})
export class CreateProjectComponent {
  projectForm: FormGroup;
  isSubmitting = false;
  errorMessage = '';

  templates: TemplateOption[] = [
    { value: 'classic', label: 'Classic', enabled: true },
    { value: 'modern', label: 'Modern', enabled: false },
    { value: 'education', label: 'Education', enabled: false },
    { value: 'dark', label: 'Dark', enabled: false },
    { value: 'minimal', label: 'Minimal', enabled: false },
    { value: 'neon', label: 'Neon', enabled: false },
  ];

  voices: Voice[] = ['Joanna', 'Matthew', 'Ruth', 'Danielle', 'Aditi'];

  constructor(
    private readonly fb: FormBuilder,
    private readonly api: ApiService,
    private readonly router: Router,
  ) {
    this.projectForm = this.fb.group({
      name: ['', [Validators.required, Validators.maxLength(100)]],
      template: ['classic', Validators.required],
      voice: ['Joanna', Validators.required],
    });
  }

  selectTemplate(template: Template): void {
    this.projectForm.patchValue({ template });
  }

  createProject(): void {
    if (this.projectForm.invalid) return;

    this.isSubmitting = true;
    this.errorMessage = '';

    const request: CreateProjectRequest = this.projectForm.value;

    this.api.post<Project>('/projects', request).subscribe({
      next: (project) => {
        this.isSubmitting = false;
        this.router.navigate(['/projects', project.id, 'upload']);
      },
      error: (err) => {
        this.isSubmitting = false;
        this.errorMessage = err?.error?.message || 'Failed to create project. Please try again.';
      },
    });
  }
}
