import { Component, inject } from '@angular/core';
import { MatCardModule } from '@angular/material/card';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatDividerModule } from '@angular/material/divider';
import { ThemeService } from '../../core/services/theme.service';

@Component({
  selector: 'app-settings',
  standalone: true,
  imports: [MatCardModule, MatSlideToggleModule, MatCheckboxModule, MatDividerModule],
  template: `
    <div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-6 text-2xl font-bold">Settings</h1>

      <mat-card class="mb-6">
        <mat-card-content class="p-6">
          <h2 class="mb-4 text-lg font-semibold">Appearance</h2>
          <div class="flex items-center justify-between">
            <div>
              <p class="font-medium">Dark Mode</p>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                Toggle between light and dark theme
              </p>
            </div>
            <mat-slide-toggle
              [checked]="themeService.currentTheme() === 'dark'"
              (change)="themeService.toggleTheme()"
              aria-label="Toggle dark mode"
            />
          </div>
        </mat-card-content>
      </mat-card>

      <mat-card>
        <mat-card-content class="p-6">
          <h2 class="mb-4 text-lg font-semibold">Notifications</h2>
          <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">Coming Soon</p>

          <div class="space-y-3">
            <mat-checkbox [disabled]="true">Email notifications for completed videos</mat-checkbox>
            <mat-checkbox [disabled]="true">Email notifications for processing errors</mat-checkbox>
            <mat-checkbox [disabled]="true">Weekly usage summary</mat-checkbox>
          </div>
        </mat-card-content>
      </mat-card>
    </div>
  `,
})
export class SettingsComponent {
  readonly themeService = inject(ThemeService);
}
