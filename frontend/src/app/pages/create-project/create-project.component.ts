import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { MatStepperModule } from '@angular/material/stepper';
import { LucideDynamicIcon } from '@lucide/angular';
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
    MatStepperModule,
    LucideDynamicIcon,
  ],
  template: `
    <div class="max-w-3xl mx-auto px-4 py-8">
      <h1 class="text-2xl font-bold mb-6">Create New Project</h1>

      <div class="glass-card p-8">
        <mat-stepper linear #stepper class="bg-transparent">
          <mat-step [stepControl]="projectForm" label="Configure">
            <form [formGroup]="projectForm" class="mt-6 space-y-6">
              <!-- Project Name -->
              <div>
                <label class="block text-sm font-medium mb-2">Project Name</label>
                <input
                  type="text"
                  formControlName="name"
                  placeholder="My Quiz Video"
                  maxlength="100"
                  class="w-full px-4 py-3 rounded-lg bg-[hsl(var(--secondary))] border border-[hsl(var(--border))] focus:outline-none focus:ring-2 focus:ring-[hsl(var(--ring))] focus:border-transparent transition-all text-[hsl(var(--foreground))] placeholder-[hsl(var(--muted-foreground))]"
                />
                <div class="flex justify-between mt-1">
                  @if (projectForm.get('name')?.hasError('required') && projectForm.get('name')?.touched) {
                    <span class="text-red-400 text-xs">Project name is required</span>
                  } @else {
                    <span></span>
                  }
                  <span class="text-xs text-[hsl(var(--muted-foreground))]">{{ projectForm.get('name')?.value?.length || 0 }}/100</span>
                </div>
              </div>

              <!-- Template Selector -->
              <div>
                <label class="block text-sm font-medium mb-3">Template</label>
                <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
                  @for (tpl of templates; track tpl.value) {
                    <div
                      class="relative rounded-lg p-4 cursor-pointer transition-all border-2"
                      [class.border-[hsl(var(--primary))]]="projectForm.get('template')?.value === tpl.value && tpl.enabled"
                      [class.shadow-[0_0_15px_rgba(120,60,255,0.2)]]="projectForm.get('template')?.value === tpl.value && tpl.enabled"
                      [class.bg-[hsl(var(--primary))]/5]="projectForm.get('template')?.value === tpl.value && tpl.enabled"
                      [class.border-[hsl(var(--border))]]="projectForm.get('template')?.value !== tpl.value || !tpl.enabled"
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
                      @if (projectForm.get('template')?.value === tpl.value && tpl.enabled) {
                        <svg lucideIcon="check" [size]="14" class="absolute top-2 right-2 text-[hsl(var(--primary))]"></svg>
                      }
                      @if (!tpl.enabled) {
                        <span
                          class="absolute top-1 right-1 text-xs bg-[hsl(var(--muted))] text-[hsl(var(--muted-foreground))] px-1.5 py-0.5 rounded"
                        >
                          Soon
                        </span>
                      }
                    </div>
                  }
                </div>
              </div>

              <!-- Voice Selector -->
              <div>
                <label class="block text-sm font-medium mb-2">Narration Voice</label>
                <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
                  @for (v of voices; track v) {
                    <div
                      class="rounded-lg p-3 cursor-pointer transition-all border-2 text-center text-sm font-medium"
                      [class.border-[hsl(var(--primary))]]="projectForm.get('voice')?.value === v"
                      [class.shadow-[0_0_15px_rgba(120,60,255,0.2)]]="projectForm.get('voice')?.value === v"
                      [class.bg-[hsl(var(--primary))]/5]="projectForm.get('voice')?.value === v"
                      [class.border-[hsl(var(--border))]]="projectForm.get('voice')?.value !== v"
                      (click)="selectVoice(v)"
                      (keydown.enter)="selectVoice(v)"
                      tabindex="0"
                      role="radio"
                      [attr.aria-checked]="projectForm.get('voice')?.value === v"
                    >
                      <svg lucideIcon="mic" [size]="16" class="mx-auto mb-1" [class.text-[hsl(var(--primary))]]="projectForm.get('voice')?.value === v"></svg>
                      {{ v }}
                    </div>
                  }
                </div>
              </div>

              <!-- Submit Button -->
              <div class="flex justify-end pt-4">
                <button
                  class="glow-btn"
                  [disabled]="projectForm.invalid || isSubmitting"
                  [class.opacity-50]="projectForm.invalid || isSubmitting"
                  [class.pointer-events-none]="projectForm.invalid || isSubmitting"
                  (click)="createProject()"
                >
                  @if (isSubmitting) {
                    <svg lucideIcon="loader-2" [size]="18" class="animate-spin"></svg>
                  }
                  Next: Upload File
                  <svg lucideIcon="arrow-right" [size]="18"></svg>
                </button>
              </div>

              @if (errorMessage) {
                <p class="text-red-400 text-sm mt-2">{{ errorMessage }}</p>
              }
            </form>
          </mat-step>

          <mat-step label="Upload">
            <p class="mt-4 text-[hsl(var(--muted-foreground))]">
              Complete the configuration step first, then you'll be taken to the upload page.
            </p>
          </mat-step>
        </mat-stepper>
      </div>
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

  voices: Voice[] = ['Joanna', 'Matthew', 'Amy', 'Brian', 'Aditi'];

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

  selectVoice(voice: Voice): void {
    this.projectForm.patchValue({ voice });
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
