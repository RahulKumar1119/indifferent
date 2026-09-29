import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
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
    RouterLink,
    MatStepperModule,
    LucideDynamicIcon,
  ],
  styles: [`
    .serif { font-family: 'Cormorant Garamond', 'Playfair Display', Georgia, serif; }
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .btn-primary { transition: transform .2s cubic-bezier(0.23,1,0.32,1), background-color .2s; }
    .btn-primary:hover { transform: translateY(-1px); }
    .btn-primary:active { transform: translateY(1px) scale(.98); }
    .pick { transition: border-color .2s, background-color .2s, transform .2s; }
    .pick:hover { transform: translateY(-1px); }
    @media (prefers-reduced-motion: reduce) {
      .btn-primary, .pick { transition: none; }
    }
  `],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh]">
      <!-- App header -->
      <header class="border-b border-black/[0.07] bg-[#FAF7F2]/90 backdrop-blur sticky top-0 z-40">
        <div class="max-w-7xl mx-auto px-5 md:px-8 h-16 flex items-center justify-between">
          <a routerLink="/" class="font-semibold tracking-tight text-[17px]">Indifferent<span class="text-[#D96C3D]">.</span></a>
          <div class="flex items-center gap-2 sm:gap-3">
            <a routerLink="/" class="hidden sm:inline-flex rounded-full px-4 py-2 text-[13.5px] font-medium text-[#3E3A33] hover:bg-black/[0.05] transition-colors">Home</a>
            <a routerLink="/dashboard" class="hidden sm:inline-flex rounded-full px-4 py-2 text-[13.5px] font-medium text-[#3E3A33] hover:bg-black/[0.05] transition-colors">Dashboard</a>
            <a routerLink="/projects" class="hidden sm:inline-flex rounded-full px-4 py-2 text-[13.5px] font-medium text-[#3E3A33] hover:bg-black/[0.05] transition-colors">Projects</a>
          </div>
        </div>
      </header>

      <main class="max-w-3xl mx-auto px-5 md:px-8 py-10 md:py-14">
        <p class="text-[11.5px] uppercase tracking-[0.2em] text-[#6B6560]">Studio · Configure</p>
        <h1 class="serif mt-2 font-medium leading-none tracking-[-0.02em] text-[clamp(2.4rem,5vw,3.8rem)]">New project.</h1>
        <p class="mt-3 text-[15px] text-[#6B6560]">Name it, pick a theme and a voice — then upload your TXT file.</p>

        <div class="mt-8 rounded-[20px] bg-white border border-black/[0.07] p-5 md:p-8">
          <mat-stepper linear #stepper class="bg-transparent">
            <mat-step [stepControl]="projectForm" label="Configure">
              <form [formGroup]="projectForm" class="mt-6 space-y-7">
                <!-- Project Name -->
                <div>
                  <label class="block text-[11.5px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-2">Project name</label>
                  <input
                    type="text"
                    formControlName="name"
                    placeholder="My Quiz Video"
                    maxlength="100"
                    class="w-full px-4 h-12 rounded-[12px] bg-[#1A1714]/[.03] border border-black/15 focus:outline-none focus:border-[#BC5227] focus:ring-2 focus:ring-[#BC5227]/20 transition-all text-[#1A1714] placeholder-[#6B6560]/70"
                  />
                  <div class="flex justify-between mt-1.5">
                    @if (projectForm.get('name')?.hasError('required') && projectForm.get('name')?.touched) {
                      <span class="text-[#B3372F] text-xs">Project name is required</span>
                    } @else {
                      <span></span>
                    }
                    <span class="text-xs text-[#6B6560]">{{ projectForm.get('name')?.value?.length || 0 }}/100</span>
                  </div>
                </div>

                <!-- Template Selector -->
                <div>
                  <label class="block text-[11.5px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-3">Template</label>
                  <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
                    @for (tpl of templates; track tpl.value) {
                      <div
                        class="pick relative rounded-[12px] p-4 cursor-pointer border-2"
                        [class.border-[#BC5227]]="projectForm.get('template')?.value === tpl.value && tpl.enabled"
                        [class.bg-[#BC5227]/[.06]]="projectForm.get('template')?.value === tpl.value && tpl.enabled"
                        [class.border-black/10]="projectForm.get('template')?.value !== tpl.value || !tpl.enabled"
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
                          <svg lucideIcon="check" [size]="14" class="absolute top-2 right-2 text-[#BC5227]"></svg>
                        }
                        @if (!tpl.enabled) {
                          <span class="absolute top-1.5 right-1.5 text-[11px] font-medium bg-black/[0.06] text-[#6B6560] px-1.5 py-0.5 rounded-full">Soon</span>
                        }
                      </div>
                    }
                  </div>
                </div>

                <!-- Voice Selector -->
                <div>
                  <label class="block text-[11.5px] font-semibold uppercase tracking-[0.14em] text-[#6B6560] mb-3">Narration voice</label>
                  <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
                    @for (v of voices; track v) {
                      <div
                        class="pick rounded-[12px] p-3 cursor-pointer border-2 text-center text-sm font-medium"
                        [class.border-[#BC5227]]="projectForm.get('voice')?.value === v"
                        [class.bg-[#BC5227]/[.06]]="projectForm.get('voice')?.value === v"
                        [class.border-black/10]="projectForm.get('voice')?.value !== v"
                        (click)="selectVoice(v)"
                        (keydown.enter)="selectVoice(v)"
                        tabindex="0"
                        role="radio"
                        [attr.aria-checked]="projectForm.get('voice')?.value === v"
                      >
                        <svg lucideIcon="mic" [size]="16" class="mx-auto mb-1" [class.text-[#BC5227]]="projectForm.get('voice')?.value === v"></svg>
                        {{ v }}
                      </div>
                    }
                  </div>
                </div>

                <!-- Submit Button -->
                <div class="flex justify-end pt-2">
                  <button
                    class="btn-primary inline-flex items-center gap-2 rounded-full bg-[#1A1714] text-white pl-7 pr-2.5 py-2.5 font-semibold text-[14.5px] disabled:opacity-40"
                    [disabled]="projectForm.invalid || isSubmitting"
                    [class.pointer-events-none]="projectForm.invalid || isSubmitting"
                    (click)="createProject()"
                  >
                    @if (isSubmitting) {
                      <svg lucideIcon="loader-2" [size]="18" class="animate-spin"></svg>
                    }
                    Next: upload file
                    <span class="w-8 h-8 rounded-full bg-white/15 flex items-center justify-center">
                      <svg lucideIcon="arrow-right" [size]="16"></svg>
                    </span>
                  </button>
                </div>

                @if (errorMessage) {
                  <p class="text-[#B3372F] text-sm mt-2">{{ errorMessage }}</p>
                }
              </form>
            </mat-step>

            <mat-step label="Upload">
              <p class="mt-4 text-[#6B6560]">
                Complete the configuration step first, then you'll be taken to the upload page.
              </p>
            </mat-step>
          </mat-stepper>
        </div>

        <footer class="mt-10 pt-6 border-t border-black/[0.07] flex flex-col sm:flex-row justify-between gap-2 text-[12.5px] text-[#6B6560]">
          <span>© {{ currentYear }} Indifferent. All rights reserved.</span>
          <span class="flex gap-5">
            <a routerLink="/" class="hover:text-[#1A1714]">Home</a>
            <a routerLink="/projects" class="hover:text-[#1A1714]">Projects</a>
            <a routerLink="/contact" class="hover:text-[#1A1714]">Contact</a>
          </span>
        </footer>
      </main>
    </div>
  `,
})
export class CreateProjectComponent {
  currentYear = new Date().getFullYear();
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
