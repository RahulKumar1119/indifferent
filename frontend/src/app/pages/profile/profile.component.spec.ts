import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import {
  provideLucideIcons,
  LucideArrowRight,
  LucideCheck,
  LucideClapperboard,
  LucideCircleCheck,
  LucideClock,
  LucideImage,
  LucideLock,
  LucideMoon,
  LucideMoreVertical,
  LucidePencil,
  LucidePlay,
  LucideRefreshCw,
  LucideScissors,
  LucideShare2,
  LucideSparkles,
  LucideSquarePlay,
  LucideCamera,
  LucideMusic2,
  LucideStar,
  LucideSun,
  LucideUpload,
  LucideUser,
  LucideX,
} from '@lucide/angular';
import { ProfileComponent } from './profile.component';
import { environment } from '../../../environments/environment';

const LS_PROFILE = 'indifferent.creatorProfile.v1';

describe('ProfileComponent', () => {
  beforeEach(() => {
    for (const k of Object.keys(localStorage)) if (k.startsWith('indifferent.creator')) localStorage.removeItem(k);
  });
  afterEach(() => {
    for (const k of Object.keys(localStorage)) if (k.startsWith('indifferent.creator')) localStorage.removeItem(k);
  });

  function setup() {
    TestBed.configureTestingModule({
      imports: [ProfileComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
        provideLucideIcons(
          LucideArrowRight, LucideCheck, LucideClapperboard, LucideCircleCheck, LucideClock, LucideImage, LucideLock, LucideMoon,
          LucideMoreVertical, LucidePencil, LucidePlay, LucideRefreshCw, LucideScissors, LucideShare2, LucideSparkles, LucideSquarePlay, LucideCamera, LucideMusic2, LucideStar,
          LucideSun, LucideUpload, LucideUser, LucideX,
        ),
      ],
    });
    const fixture = TestBed.createComponent(ProfileComponent);
    const http = TestBed.inject(HttpTestingController);
    fixture.detectChanges();
    http.match(`${environment.apiUrl}/auth/me`).forEach((r) => r.flush({ email: 'me@example.com', name: 'Asha Rao', avatarUrl: '', createdAt: '2026-01-01T00:00:00Z' }));
    http.match(`${environment.apiUrl}/projects`).forEach((r) => r.flush({ projects: [] }));
    http.match(`${environment.apiUrl}/shorts`).forEach((r) => r.flush({ jobs: [] }));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const input = (id: string) => el.querySelector(`#${id}`) as HTMLInputElement;
    const typeInto = (id: string, value: string) => {
      input(id).value = value;
      input(id).dispatchEvent(new Event('input'));
    };
    const button = (text: string) => [...el.querySelectorAll('button')].find((b) => b.textContent?.trim() === text) as HTMLButtonElement;
    return { fixture, http, el, input, typeInto, button };
  }

  it('shows profile details as a labelled form with one h1', async () => {
    const { el, fixture } = setup();
    await fixture.whenStable();
    fixture.detectChanges();
    for (const id of ['pf-name', 'pf-username', 'pf-bio']) {
      expect(el.querySelector(`label[for="${id}"]`)).withContext(id).toBeTruthy();
    }
    expect(el.querySelectorAll('h1').length).toBe(1);
    expect(el.querySelector('h1')!.textContent!.trim()).toBe('Profile');
    expect(el.textContent).toContain('saved in this browser');
  });

  it('saves name, username and bio to this browser with the Save action', async () => {
    const { el, fixture, typeInto, button } = setup();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(button('Save changes').disabled).toBeTrue();
    typeInto('pf-username', 'asha_makes');
    typeInto('pf-bio', 'Quiz videos about maps');
    fixture.detectChanges();
    expect(button('Save changes').disabled).toBeFalse();
    (el.querySelector('[data-testid="profile-form"]') as HTMLFormElement).dispatchEvent(new Event('submit'));
    fixture.detectChanges();
    const saved = JSON.parse(localStorage.getItem(LS_PROFILE)!);
    expect(saved.username).toBe('asha_makes');
    expect(saved.bio).toBe('Quiz videos about maps');
    expect(el.querySelector('[data-testid="toast"]')!.textContent).toContain('Profile updated.');
  });

  it('refuses a bad username and keeps the saved one', async () => {
    const { el, fixture, typeInto } = setup();
    await fixture.whenStable();
    fixture.detectChanges();
    typeInto('pf-username', 'no spaces!');
    fixture.detectChanges();
    (el.querySelector('[data-testid="profile-form"]') as HTMLFormElement).dispatchEvent(new Event('submit'));
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')!.textContent).toContain('letters, numbers or underscores');
    expect(JSON.parse(localStorage.getItem(LS_PROFILE)!).username ?? '').toBe('');
  });

  it('discards unsaved edits', async () => {
    const { fixture, input, typeInto, button } = setup();
    await fixture.whenStable();
    fixture.detectChanges();
    typeInto('pf-bio', 'Not kept');
    fixture.detectChanges();
    button('Discard').click();
    fixture.detectChanges();
    await fixture.whenStable();
    expect(input('pf-bio').value).not.toBe('Not kept');
  });

  it('moves sign-out to Account and links the three account screens', () => {
    const { el } = setup();
    expect([...el.querySelectorAll('button')].some((b) => b.textContent?.trim() === 'Sign out')).toBeFalse();
    const hrefs = Array.from(el.querySelectorAll('app-account-nav nav a')).map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(['/profile', '/profile/settings', '/account']);
  });

  it('says plainly that connected accounts and Creator DNA are only kept in this browser', () => {
    const { el } = setup();
    expect(el.textContent).toContain('Publishing to these channels is not built yet');
    expect(el.textContent).toContain('The video generators do not read it yet');
  });

  it('keeps hex colours, gradients and tracked capitals out of the markup', () => {
    const { el } = setup();
    expect(el.innerHTML).not.toMatch(/#[0-9A-Fa-f]{6}\b/);
    expect(el.innerHTML).not.toContain('gradient');
    expect(el.innerHTML).not.toContain('uppercase');
  });
});
