import { Component, input, output } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { SOCIAL_PLATFORMS, SocialConnection, SocialPlatform } from '../services/profile.service';

/**
 * Connected social accounts. This is UI + local connection state only:
 * real OAuth/API wiring is a separate integration (per spec).
 */
@Component({
  selector: 'app-connected-accounts',
  standalone: true,
  imports: [LucideDynamicIcon],
  template: `
    <section class="glass-card p-5 md:p-7" aria-label="Connected social accounts">
      <h2 class="text-lg font-semibold flex items-center gap-2">
        <svg lucideIcon="share-2" [size]="19" class="text-[hsl(var(--primary))]" aria-hidden="true"></svg>
        Connected Accounts
      </h2>
      <p class="mt-1 text-[13px] text-[hsl(var(--muted-foreground))]">
        Publish straight to your channels. OAuth integration coming soon. Connections are remembered on this device.
      </p>
      <ul class="mt-4 space-y-3">
        @for (platform of platforms; track platform.platform) {
          @let conn = connectionFor(platform.platform);
          <li
            class="flex items-center gap-3.5 rounded-2xl border border-[hsl(var(--border))]/60 bg-white/[0.02] p-4"
          >
            <span
              class="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
              [class]="conn?.connected ? 'bg-[hsl(var(--primary))]/10 text-[hsl(var(--primary))]' : 'bg-[hsl(var(--secondary))] text-[hsl(var(--muted-foreground))]'"
              aria-hidden="true"
            >
              <svg lucideIcon="{{ platform.icon }}" [size]="19"></svg>
            </span>
            <span class="flex-1 min-w-0">
              <span class="block font-medium text-[14.5px]">{{ platform.label }}</span>
              @if (conn?.connected) {
                <span class="block text-[12.5px] text-emerald-500 font-medium">✓ Connected · {{ conn?.handle }}</span>
              } @else {
                <span class="block text-[12.5px] text-[hsl(var(--muted-foreground))]">○ Not connected</span>
              }
            </span>
            @if (conn?.connected) {
              <button
                type="button"
                (click)="toggle.emit(platform.platform)"
                class="btn-interactive shrink-0 rounded-full border border-[hsl(var(--border))] px-4 py-2 text-[13px] font-medium hover:bg-white/5"
                [attr.aria-label]="'Manage ' + platform.label + ' connection'"
              >
                Manage
              </button>
            } @else {
              <button
                type="button"
                (click)="toggle.emit(platform.platform)"
                class="btn-interactive shrink-0 rounded-full bg-[hsl(var(--foreground))] text-[hsl(var(--background))] px-4 py-2 text-[13px] font-semibold"
                [attr.aria-label]="'Connect ' + platform.label"
              >
                Connect
              </button>
            }
          </li>
        }
      </ul>
    </section>
  `,
})
export class ConnectedAccountsComponent {
  readonly connections = input.required<SocialConnection[]>();
  readonly toggle = output<SocialPlatform>();

  readonly platforms = SOCIAL_PLATFORMS;

  connectionFor(platform: SocialPlatform): SocialConnection | undefined {
    return this.connections().find((c) => c.platform === platform);
  }
}
