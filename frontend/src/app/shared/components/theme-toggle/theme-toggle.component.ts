import { Component, inject } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { ThemeService } from '../../../core/services/theme.service';

/**
 * Theme toggle: switches the app between light and dark. Placed in the app
 * shell so it is available on every page and actually changes the studio
 * chrome (the token-based pages). Marketing pages keep their light editorial
 * theme by design; the toggle still honors the user's choice globally.
 */
@Component({
  selector: 'app-theme-toggle',
  standalone: true,
  imports: [LucideDynamicIcon],
  template: `
    <button
      type="button"
      class="w-9 h-9 rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--card))]/90 backdrop-blur inline-flex items-center justify-center"
      (click)="theme.toggleTheme()"
      [attr.aria-label]="theme.currentTheme() === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'"
      [title]="theme.currentTheme() === 'dark' ? 'Light mode' : 'Dark mode'"
    >
      @if (theme.currentTheme() === 'dark') {
        <svg lucideIcon="sun" [size]="17"></svg>
      } @else {
        <svg lucideIcon="moon" [size]="17"></svg>
      }
    </button>
  `,
})
export class ThemeToggleComponent {
  readonly theme = inject(ThemeService);
}
