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
  styles: [`
    :host { display: block; }
    ul { margin: var(--space-md) 0 0; padding: 0; list-style: none; display: flex; flex-direction: column; }
    li { display: flex; align-items: center; gap: var(--space-sm); padding: var(--space-sm) 0; }
    li + li { border-top: 1px solid var(--color-rule); }
    .main { flex: 1; min-width: 0; }
    .name { display: block; font-weight: 500; }
    .state { display: inline-flex; align-items: center; gap: var(--space-3xs); font-size: var(--type-xs); color: var(--color-muted); }
    .state.on { color: var(--color-positive); font-weight: 600; }
  `],
  template: `
    <section class="ui-card" aria-labelledby="social-heading">
      <h2 id="social-heading" class="ui-card-title">Connected accounts</h2>
      <p class="ui-card-sub">
        Publishing to these channels is not built yet. Marking a channel connected only remembers it on this device; nothing is linked to the channel itself.
      </p>
      <ul>
        @for (platform of platforms; track platform.platform) {
          @let conn = connectionFor(platform.platform);
          <li>
            <span class="ui-chip" aria-hidden="true"><svg [lucideIcon]="platform.icon" [size]="18"></svg></span>
            <span class="main">
              <span class="name">{{ platform.label }}</span>
              <span class="state" [class.on]="conn?.connected">
                @if (conn?.connected) {
                  <svg lucideIcon="check" [size]="12" aria-hidden="true"></svg>Marked connected on this device
                } @else {
                  Not connected
                }
              </span>
            </span>
            <button type="button" class="ui-btn ui-btn--secondary" (click)="toggle.emit(platform.platform)"
              [attr.aria-label]="(conn?.connected ? 'Disconnect ' : 'Mark connected: ') + platform.label">
              {{ conn?.connected ? 'Disconnect' : 'Mark connected' }}
            </button>
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
