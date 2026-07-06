import { Component, inject } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { ThemeService } from '../../core/services/theme.service';

@Component({
  selector: 'app-settings',
  standalone: true,
  imports: [LucideDynamicIcon],
  template: `
    <div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-6 text-2xl font-bold">Settings</h1>

      <div class="glass-card p-6 mb-6">
        <h2 class="mb-4 text-lg font-semibold flex items-center gap-2">
          <svg lucideIcon="sun" [size]="20" class="text-[hsl(var(--primary))]"></svg>
          Appearance
        </h2>
        <div class="flex items-center justify-between">
          <div>
            <p class="font-medium">Dark Mode</p>
            <p class="text-sm text-[hsl(var(--muted-foreground))]">
              Toggle between light and dark theme
            </p>
          </div>
          <!-- Custom toggle switch -->
          <button
            class="relative w-12 h-6 rounded-full transition-colors cursor-pointer"
            [class.bg-[hsl(var(--primary))]]="themeService.currentTheme() === 'dark'"
            [class.bg-[hsl(var(--muted))]]="themeService.currentTheme() === 'light'"
            (click)="themeService.toggleTheme()"
            role="switch"
            [attr.aria-checked]="themeService.currentTheme() === 'dark'"
            aria-label="Toggle dark mode"
          >
            <span
              class="absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white shadow-sm transition-transform flex items-center justify-center"
              [class.translate-x-6]="themeService.currentTheme() === 'dark'"
            >
              @if (themeService.currentTheme() === 'dark') {
                <svg lucideIcon="moon" [size]="12" class="text-[hsl(var(--primary))]"></svg>
              } @else {
                <svg lucideIcon="sun" [size]="12" class="text-yellow-500"></svg>
              }
            </span>
          </button>
        </div>
      </div>

      <div class="glass-card p-6">
        <h2 class="mb-4 text-lg font-semibold flex items-center gap-2">
          <svg lucideIcon="settings" [size]="20" class="text-[hsl(var(--primary))]"></svg>
          Notifications
        </h2>
        <p class="mb-4 text-sm text-[hsl(var(--muted-foreground))]">Coming Soon</p>

        <div class="space-y-3">
          <label class="flex items-center gap-3 opacity-50 cursor-not-allowed">
            <div class="w-5 h-5 rounded border border-[hsl(var(--border))] bg-[hsl(var(--secondary))]"></div>
            <span class="text-sm">Email notifications for completed videos</span>
          </label>
          <label class="flex items-center gap-3 opacity-50 cursor-not-allowed">
            <div class="w-5 h-5 rounded border border-[hsl(var(--border))] bg-[hsl(var(--secondary))]"></div>
            <span class="text-sm">Email notifications for processing errors</span>
          </label>
          <label class="flex items-center gap-3 opacity-50 cursor-not-allowed">
            <div class="w-5 h-5 rounded border border-[hsl(var(--border))] bg-[hsl(var(--secondary))]"></div>
            <span class="text-sm">Weekly usage summary</span>
          </label>
        </div>
      </div>
    </div>
  `,
})
export class SettingsComponent {
  readonly themeService = inject(ThemeService);
}
