import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { LucideLoader2, provideLucideIcons } from '@lucide/angular';
import { environment } from '../../environments/environment';
import { AdsConversionService } from '../core/services/ads-conversion.service';
import { VerifyEmailComponent } from '../pages/verify-email/verify-email.component';
import { AuthCallbackComponent } from './components/auth-callback.component';
import { LoginComponent } from './components/login.component';

describe('login page', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([{ path: 'login', component: LoginComponent }]),
        provideLucideIcons(LucideLoader2),
        { provide: AdsConversionService, useValue: { prepare: () => undefined, trackSignInClick: (n: () => void) => n() } },
      ],
    });
  });
  afterEach(() => {
    document.head.querySelectorAll('meta[name="robots"], link[rel="canonical"]').forEach((el) => el.remove());
    localStorage.clear();
  });

  async function open(url = '/login') {
    const harness = await RouterTestingHarness.create();
    const cmp = await harness.navigateByUrl(url, LoginComponent);
    harness.detectChanges();
    return { harness, cmp, page: harness.routeNativeElement as HTMLElement };
  }

  it('has one h1, the new sentence and the true supporting line', async () => {
    const { page } = await open();
    expect(page.querySelectorAll('h1').length).toBe(1);
    expect(page.textContent).toContain('Sign in to turn stories, quizzes and long videos into short videos.');
    expect(page.textContent).toContain('Continue with Google, or use your email. No credit card.');
    expect(page.textContent).not.toContain('upload TXT files and render narrated quiz films');
    expect(page.textContent).not.toContain('No passwords');
  });

  it('has one main landmark holding the form', async () => {
    const { page } = await open();
    expect(page.querySelectorAll('main').length).toBe(1);
    expect(page.querySelector('main form')).not.toBeNull();
  });

  it('keeps the Google button first in the page, with one separator only', async () => {
    const { page } = await open();
    expect(page.querySelector('button')!.textContent).toContain('Continue with Google');
    expect(page.textContent).toContain('or continue with email');
    expect(page.textContent).not.toContain('Secure OAuth');
  });

  it('gives every input a programmatic label', async () => {
    const { page } = await open('/login?mode=signup');
    const inputs = Array.from(page.querySelectorAll('form input')) as HTMLInputElement[];
    expect(inputs.length).toBe(3);
    for (const input of inputs) {
      const label = page.querySelector(`label[for="${input.id}"]`);
      expect(label).withContext(input.outerHTML).not.toBeNull();
      expect(label!.textContent!.trim().length).toBeGreaterThan(3);
    }
  });

  it('swaps the heading, the button and the name field between sign in and sign up', async () => {
    const { harness, page } = await open();
    expect(page.querySelector('h2')!.textContent).toContain('Sign in to your studio.');
    expect(page.querySelector('form button[type="submit"]')!.textContent).toContain('Sign in');
    expect(page.querySelector('input[name="name"]')).toBeNull();
    await harness.navigateByUrl('/login?mode=signup', LoginComponent);
    harness.detectChanges();
    expect(page.querySelector('h2')!.textContent).toContain('Create your account.');
    expect(page.querySelector('form button[type="submit"]')!.textContent).toContain('Create account');
    expect(page.querySelector('input[name="name"]')).not.toBeNull();
  });

  it('announces an error from the server in an alert, and ties it to the fields', async () => {
    const { harness, cmp, page } = await open();
    const http = TestBed.inject(HttpTestingController);
    cmp.email = 'me@example.com';
    cmp.password = 'a-long-password';
    cmp.submitPasswordForm();
    http.expectOne(`${environment.apiUrl}/auth/login`).flush(
      { code: 'RATE_LIMITED', message: 'Too many sign-in attempts. Try again in 12 minutes.' },
      { status: 429, statusText: 'Too Many Requests' },
    );
    harness.detectChanges();
    const alert = page.querySelector('[role="alert"]')!;
    expect(alert.textContent).toContain('Too many sign-in attempts. Try again in 12 minutes.');
    expect(page.querySelector('#auth-email')!.getAttribute('aria-describedby')).toContain('auth-error');
  });

  it('keeps the form rules: it asks for an email and 8 characters before calling the server', async () => {
    const { harness, cmp, page } = await open();
    const http = TestBed.inject(HttpTestingController);
    cmp.email = 'me@example.com';
    cmp.password = 'short';
    cmp.submitPasswordForm();
    harness.detectChanges();
    http.expectNone(`${environment.apiUrl}/auth/login`);
    expect(page.querySelector('[role="alert"]')!.textContent).toContain('a password of 8+ characters');
  });

  it('reads ?error= and shows its message', async () => {
    const { page } = await open('/login?error=invalid_state');
    expect(page.querySelector('[role="alert"]')!.textContent).toContain('could not be verified');
  });

  it('links the terms and the privacy policy and has no gradient, glow, grain or tracked capitals', async () => {
    const { page } = await open();
    expect(page.querySelector('a[href="/terms"]')).not.toBeNull();
    expect(page.querySelector('a[href="/privacy"]')).not.toBeNull();
    expect(page.innerHTML).not.toMatch(/gradient\(|grain|uppercase|tracking-\[/);
    expect(page.textContent).not.toMatch(/[\u2197\u2192\u2190]/);
  });
});

describe('callback page', () => {
  it('says what is happening next to the spinner and announces it politely', () => {
    sessionStorage.clear();
    TestBed.configureTestingModule({
      imports: [AuthCallbackComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
        { provide: ActivatedRoute, useValue: { snapshot: { queryParamMap: convertToParamMap({}) } } },
      ],
    });
    const fixture = TestBed.createComponent(AuthCallbackComponent);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const status = el.querySelector('[role="status"]')!;
    expect(status.getAttribute('aria-live')).toBe('polite');
    expect(status.textContent).toContain('Signing you in');
    expect(el.querySelector('mat-spinner')).not.toBeNull();
    TestBed.resetTestingModule();
  });
});

describe('verify-email page states', () => {
  function open(query: Record<string, string>) {
    localStorage.clear();
    TestBed.configureTestingModule({
      imports: [VerifyEmailComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
        { provide: ActivatedRoute, useValue: { snapshot: { queryParamMap: convertToParamMap(query) } } },
      ],
    });
    const f = TestBed.createComponent(VerifyEmailComponent);
    f.detectChanges();
    return { f, http: TestBed.inject(HttpTestingController), el: f.nativeElement as HTMLElement };
  }
  afterEach(() => {
    document.head.querySelectorAll('meta[name="robots"]').forEach((e) => e.remove());
    localStorage.clear();
  });

  it('working: one h1 announced as a status', () => {
    const { el } = open({ token: 'abc' });
    expect(el.querySelectorAll('h1').length).toBe(1);
    expect(el.querySelector('h1')!.getAttribute('role')).toBe('status');
  });

  it('done: one h1 and one primary action', () => {
    const { f, http, el } = open({ token: 'abc' });
    http.expectOne(`${environment.apiUrl}/auth/verify-email`).flush(null, { status: 204, statusText: 'No Content' });
    f.detectChanges();
    expect(el.querySelectorAll('h1').length).toBe(1);
    expect(el.querySelector('h1')!.textContent).toContain('is confirmed');
    expect(el.querySelectorAll('a.ui-btn--primary').length).toBe(1);
  });

  it('failed: the reason is an alert and the action leads somewhere useful', () => {
    const { el } = open({});
    expect(el.querySelector('h1')!.textContent).toContain('did not work');
    expect(el.querySelector('[role="alert"]')!.textContent).toContain('incomplete');
    expect(el.querySelectorAll('a.ui-btn--primary').length).toBe(1);
  });
});
