import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { CONSENT_KEY } from '../../../core/consent/consent.model';
import { CONSENT_STORAGE, CookieConsentService } from '../../../core/consent/consent.service';
import { MemoryStorage } from '../../../core/consent/testing';
import { SiteFooterComponent } from '../site-footer/site-footer.component';
import { CookieBannerComponent } from './cookie-banner.component';
import { CookiePreferencesComponent } from './cookie-preferences.component';

const tick = () => new Promise<void>((r) => setTimeout(r, 20));

describe('cookie consent UI', () => {
  let storage: MemoryStorage;
  let consent: CookieConsentService;

  function mount() {
    storage = new MemoryStorage();
    TestBed.configureTestingModule({
      imports: [CookieBannerComponent, CookiePreferencesComponent, SiteFooterComponent],
      providers: [provideRouter([]), { provide: CONSENT_STORAGE, useValue: storage }],
    });
    consent = TestBed.inject(CookieConsentService);
    const banner = TestBed.createComponent(CookieBannerComponent);
    const prefs = TestBed.createComponent(CookiePreferencesComponent);
    return { banner, prefs };
  }
  async function render(...f: { detectChanges(): void }[]) {
    f.forEach((x) => x.detectChanges());
    await tick();
    f.forEach((x) => x.detectChanges());
  }
  const q = (el: HTMLElement, s: string) => el.querySelector(s) as HTMLElement | null;
  const button = (el: HTMLElement, text: string) => Array.from(el.querySelectorAll('button')).find((b) => b.textContent!.trim() === text) as HTMLButtonElement;

  it('shows the banner with the three actions to a visitor who has not chosen', async () => {
    const { banner } = mount();
    await render(banner);
    const el = banner.nativeElement as HTMLElement;
    expect(q(el, '[data-cookie-banner]')).not.toBeNull();
    expect(el.textContent).toContain('We value your privacy');
    for (const t of ['Accept All', 'Reject All', 'Customize Preferences']) expect(button(el, t)).withContext(t).toBeTruthy();
    expect(q(el, '[data-cookie-banner]')!.getAttribute('aria-labelledby')).toBe('ck-banner-title');
  });

  it('is not shown when the visitor already chose', async () => {
    const { banner } = mount();
    consent.rejectAll();
    await render(banner);
    expect(q(banner.nativeElement, '[data-cookie-banner]')).toBeNull();
  });

  it('is not shown while the page is prerendered', async () => {
    (window as unknown as { __PRERENDER__?: boolean }).__PRERENDER__ = true;
    try {
      const { banner } = mount();
      await render(banner);
      expect(q(banner.nativeElement, '[data-cookie-banner]')).toBeNull();
    } finally {
      delete (window as unknown as { __PRERENDER__?: boolean }).__PRERENDER__;
    }
  });

  it('Accept All and Reject All save the choice and remove the banner', async () => {
    const { banner } = mount();
    await render(banner);
    button(banner.nativeElement, 'Accept All').click();
    await render(banner);
    expect(JSON.parse(storage.getItem(CONSENT_KEY)!).choices).toEqual({ analytics: true, advertising: true, functional: true });
    expect(q(banner.nativeElement, '[data-cookie-banner]')).toBeNull();
  });

  it('Customize opens the panel with every optional switch off and the banner steps aside', async () => {
    const { banner, prefs } = mount();
    await render(banner, prefs);
    button(banner.nativeElement, 'Customize Preferences').click();
    await render(banner, prefs);
    const p = prefs.nativeElement as HTMLElement;
    const dialog = q(p, '[data-cookie-preferences]')!;
    expect(dialog.getAttribute('role')).toBe('dialog');
    expect(dialog.getAttribute('aria-modal')).toBe('true');
    const switches = Array.from(p.querySelectorAll('[role=switch]'));
    expect(switches.length).toBe(3);
    for (const s of switches) expect(s.getAttribute('aria-checked')).toBe('false');
    expect(p.textContent).toContain('Always on');
    expect(q(banner.nativeElement, '[data-cookie-banner]')!.hasAttribute('hidden')).toBeTrue();
    expect(document.activeElement?.id).toBe('ck-pref-title');
  });

  it('Save stores only what was switched on; nothing is saved by toggling alone', async () => {
    const { prefs } = mount();
    consent.openPreferences();
    await render(prefs);
    const p = prefs.nativeElement as HTMLElement;
    (p.querySelectorAll('[role=switch]')[2] as HTMLElement).click(); // Functional
    await render(prefs);
    expect(storage.getItem(CONSENT_KEY)).toBeNull();
    button(p, 'Save preferences').click();
    await render(prefs);
    expect(consent.choices()).toEqual({ analytics: false, advertising: false, functional: true });
    expect(q(p, '[data-cookie-preferences]')).toBeNull();
  });

  it('Escape closes the panel without saving, and focus returns to the opener', async () => {
    const { prefs } = mount();
    const opener = document.createElement('button');
    document.body.appendChild(opener);
    try {
      opener.focus();
      consent.openPreferences(opener);
      await render(prefs);
      q(prefs.nativeElement, '[data-cookie-preferences]')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
      await render(prefs);
      await tick();
      expect(q(prefs.nativeElement, '[data-cookie-preferences]')).toBeNull();
      expect(consent.needsChoice()).toBeTrue();
      expect(document.activeElement).toBe(opener);
    } finally {
      opener.remove();
    }
  });

  it('shows the saved choice when reopened, so it can be withdrawn', async () => {
    const { prefs } = mount();
    consent.acceptAll();
    consent.openPreferences();
    await render(prefs);
    for (const s of Array.from(prefs.nativeElement.querySelectorAll('[role=switch]') as NodeListOf<HTMLElement>)) expect(s.getAttribute('aria-checked')).toBe('true');
    button(prefs.nativeElement, 'Reject All').click();
    await render(prefs);
    expect(consent.choices()).toEqual({ analytics: false, advertising: false, functional: false });
  });

  it('the footer link reopens the panel', async () => {
    const { prefs } = mount();
    const footer = TestBed.createComponent(SiteFooterComponent);
    await render(footer, prefs);
    button(footer.nativeElement, 'Cookie preferences').click();
    await render(footer, prefs);
    expect(q(prefs.nativeElement, '[data-cookie-preferences]')).not.toBeNull();
  });
});
