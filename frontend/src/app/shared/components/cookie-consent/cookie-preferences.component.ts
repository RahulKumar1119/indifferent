import { CdkTrapFocus } from '@angular/cdk/a11y';
import { DOCUMENT } from '@angular/common';
import { ChangeDetectionStrategy, Component, Injector, afterNextRender, computed, effect, inject, signal, untracked } from '@angular/core';
import { RouterLink } from '@angular/router';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  CATEGORIES,
  ConsentChoices,
  NO_OPTIONAL,
  OptionalCategory,
} from '../../../core/consent/consent.model';
import { CookieConsentService } from '../../../core/consent/consent.service';

/**
 * The preferences panel (#321): every category with what it does, a switch for
 * each optional one (all off until the visitor turns them on), and Save. It
 * opens from the banner and from the "Cookie preferences" link in the footer,
 * the privacy page and Settings, traps focus while open, closes with Escape and
 * hands focus back to what opened it.
 */
@Component({
  selector: 'app-cookie-preferences',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CdkTrapFocus, RouterLink],
  styles: [`
    :host { display: contents; }
    .backdrop { position: fixed; inset: 0; z-index: 1100; background: color-mix(in oklab, var(--color-ink) 55%, transparent); }
    .panel {
      position: fixed; z-index: 1101; inset-block-start: 50%; inset-inline-start: 50%; transform: translate(-50%, -50%);
      width: min(40rem, calc(100vw - 2 * var(--space-sm))); max-height: calc(100dvh - 2 * var(--space-sm)); overflow-y: auto;
      padding: var(--space-lg); background: var(--color-paper); color: var(--color-ink);
      border: 1px solid var(--color-rule); border-radius: var(--radius-surface); font-family: var(--font-body);
      box-shadow: 0 24px 60px -16px color-mix(in oklab, var(--color-ink) 55%, transparent);
    }
    :host-context([dir=rtl]) .panel { transform: translate(50%, -50%); }
    h2 { margin: 0; font-size: var(--type-xl); font-weight: 600; }
    h2:focus { outline: none; }
    .lead, .note { margin: var(--space-xs) 0 0; font-size: var(--type-sm); line-height: 1.5; color: var(--color-ink-2); }
    ul { list-style: none; margin: var(--space-md) 0 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-sm); }
    li { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-md); padding: var(--space-sm) var(--space-md); border: 1px solid var(--color-rule); border-radius: var(--radius-control); }
    .text { min-width: 0; }
    .name { font-weight: 600; }
    .desc { margin: 2px 0 0; font-size: var(--type-sm); line-height: 1.45; color: var(--color-ink-2); }
    .always { flex: none; font-size: var(--type-sm); font-weight: 600; color: var(--color-ink-2); white-space: nowrap; padding-block-start: 2px; }
    .actions { display: flex; flex-wrap: wrap; gap: var(--space-xs); margin-block-start: var(--space-md); }
    .actions .ui-btn { flex: 1 1 8rem; justify-content: center; }
  `],
  template: `
    <p class="sr-only" role="status" aria-live="polite" data-testid="consent-status">{{ status() }}</p>
    @if (open()) {
      <div class="backdrop" (click)="close()" aria-hidden="true"></div>
      <div class="panel" role="dialog" aria-modal="true" aria-labelledby="ck-pref-title" cdkTrapFocus (keydown.escape)="close()" data-cookie-preferences>
        <h2 id="ck-pref-title" tabindex="-1" #title>Cookie preferences</h2>
        <p class="lead">
          Choose which optional cookies and browser storage this site may use. Nothing optional is on until you turn it on.
          <a routerLink="/privacy" class="ui-link" (click)="close()">Privacy Policy</a>
        </p>
        <ul>
          @for (c of categories; track c.id) {
            <li>
              <div class="text">
                <div class="name" [id]="'ck-name-' + c.id">{{ c.title }}</div>
                <p class="desc" [id]="'ck-desc-' + c.id">{{ c.description }}</p>
              </div>
              @if (c.id === 'necessary') {
                <span class="always">Always on</span>
              } @else {
                <button
                  type="button" role="switch" class="ui-switch"
                  [attr.aria-checked]="draft()[asOptional(c.id)]"
                  [attr.aria-labelledby]="'ck-name-' + c.id" [attr.aria-describedby]="'ck-desc-' + c.id"
                  (click)="toggle(asOptional(c.id))"
                ><span></span></button>
              }
            </li>
          }
        </ul>
        <p class="note">
          Turning a category off stops it from being used from now on, and removes the cookies and storage this site can reach. Cookies that Google or Amazon keep on their
          own domains, and cookies marked HttpOnly, are not visible to this site and may stay until they expire. You can delete them in your browser settings.
        </p>
        <div class="actions">
          <button type="button" class="ui-btn ui-btn--primary" (click)="save()">Save preferences</button>
          <button type="button" class="ui-btn ui-btn--primary" (click)="consent.acceptAll()">Accept All</button>
          <button type="button" class="ui-btn ui-btn--primary" (click)="consent.rejectAll()">Reject All</button>
          <button type="button" class="ui-btn ui-btn--secondary" (click)="close()">Close</button>
        </div>
      </div>
    }
  `,
})
export class CookiePreferencesComponent {
  protected readonly consent = inject(CookieConsentService);
  private readonly document = inject(DOCUMENT);
  private readonly injector = inject(Injector);

  protected readonly categories = CATEGORIES;
  protected readonly open = computed(() => this.consent.preferencesOpen());
  /** The switches as the visitor has set them in the panel; saved only by Save. They start from what is allowed now (all off for a new visitor). */
  protected readonly draft = signal<ConsentChoices>({ ...NO_OPTIONAL });
  protected readonly status = signal('');
  private wasOpen = false;

  constructor() {
    effect(() => {
      const isOpen = this.open();
      untracked(() => {
        if (isOpen && !this.wasOpen) {
          this.draft.set({ ...this.consent.choices() });
          afterNextRender(() => this.document.getElementById('ck-pref-title')?.focus(), { injector: this.injector });
        } else if (!isOpen && this.wasOpen) {
          this.returnFocus();
        }
        this.wasOpen = isOpen;
      });
    });
    this.consent.changes$.pipe(takeUntilDestroyed()).subscribe(() => {
      // Cleared first so the same sentence is read again after a second save.
      this.status.set('');
      queueMicrotask(() => this.status.set('Cookie preferences saved.'));
    });
  }

  protected asOptional(id: string): OptionalCategory {
    return id as OptionalCategory;
  }

  protected toggle(category: OptionalCategory): void {
    this.draft.update((d) => ({ ...d, [category]: !d[category] }));
  }

  protected save(): void {
    this.consent.save(this.draft());
  }

  protected close(): void {
    this.consent.closePreferences();
  }

  /** Back to whatever opened the panel; if that is gone (the banner left the page), to the page's main content. */
  private returnFocus(): void {
    const opener = this.consent.takeOpener();
    afterNextRender(
      () => {
        if (opener?.isConnected && !opener.hidden && opener.offsetParent !== null) {
          opener.focus();
          return;
        }
        const main = this.document.getElementById('main');
        if (main) {
          if (!main.hasAttribute('tabindex')) main.setAttribute('tabindex', '-1');
          main.focus({ preventScroll: true });
        }
      },
      { injector: this.injector },
    );
  }
}
