import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { EmailConfirmBannerComponent } from './email-confirm-banner.component';
import { environment } from '../../../../environments/environment';

describe('EmailConfirmBannerComponent', () => {
  function setup(profile: object | 'error') {
    TestBed.configureTestingModule({
      imports: [EmailConfirmBannerComponent],
      providers: [provideHttpClient(), provideHttpClientTesting(), provideRouter([])],
    });
    const fixture = TestBed.createComponent(EmailConfirmBannerComponent);
    const http = TestBed.inject(HttpTestingController);
    fixture.detectChanges();
    const me = http.expectOne(`${environment.apiUrl}/auth/me`);
    if (profile === 'error') me.flush('x', { status: 500, statusText: 'Server Error' });
    else me.flush(profile);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = () => el.querySelector('button') as HTMLButtonElement | null;
    return { fixture, http, el, button };
  }

  it('is shown, with the address, only for an account marked unconfirmed', () => {
    const { el } = setup({ email: 'me@example.com', emailVerified: false, canConfirmEmail: true, emailConfirmationRequired: true });
    expect(el.textContent).toContain('Confirm your email address');
    expect(el.textContent).toContain('me@example.com');
  });

  it('says costly actions wait, for an account that is held back', () => {
    const { el } = setup({ email: 'me@example.com', emailVerified: false, canConfirmEmail: true, emailConfirmationRequired: true });
    expect(el.textContent).toContain('cannot create projects or Shorts');
  });

  it('does not claim that for an older account, which is not held back', () => {
    const { el } = setup({ email: 'old@example.com', emailVerified: false, canConfirmEmail: true, emailConfirmationRequired: false });
    expect(el.textContent).toContain('Confirm old@example.com to get emails');
    expect(el.textContent).not.toContain('cannot create');
  });

  it('shows nothing while confirmation is switched off', () => {
    const { el, button } = setup({ email: 'me@example.com', emailVerified: false, canConfirmEmail: false, emailConfirmationRequired: false });
    expect(button()).toBeNull();
    expect(el.textContent?.trim()).toBe('');
  });

  for (const [name, profile] of [
    ['a confirmed account', { email: 'a@b.co', emailVerified: true }],
    ['an older API that sends no flag', { email: 'a@b.co' }],
    ['a failed profile request', 'error'],
  ] as const) {
    it(`shows nothing for ${name}`, () => {
      const { el, button } = setup(profile as object | 'error');
      expect(el.textContent?.trim()).toBe('');
      expect(button()).toBeNull();
    });
  }

  it('asks for a new email once, then says it was sent', () => {
    const { fixture, http, el, button } = setup({ email: 'me@example.com', emailVerified: false, canConfirmEmail: true, emailConfirmationRequired: true });
    button()!.click();
    button()!.click(); // a second click while sending does nothing
    fixture.detectChanges();
    expect(button()!.disabled).toBeTrue();
    const req = http.expectOne(`${environment.apiUrl}/auth/resend-verification`);
    expect(req.request.method).toBe('POST');
    req.flush(null, { status: 204, statusText: 'No Content' });
    fixture.detectChanges();
    expect(el.textContent).toContain('Email sent');
    expect(el.textContent).toContain('spam');
    expect(button()!.disabled).toBeTrue();
  });

  it('shows the reason and allows another try when sending fails', () => {
    const { fixture, http, el, button } = setup({ email: 'me@example.com', emailVerified: false, canConfirmEmail: true, emailConfirmationRequired: true });
    button()!.click();
    http
      .expectOne(`${environment.apiUrl}/auth/resend-verification`)
      .flush({ code: 'QUOTA_EXCEEDED', message: "You have reached today's limit for sending confirmation emails." }, { status: 429, statusText: 'Too Many Requests' });
    fixture.detectChanges();
    expect(el.querySelector('[role=alert]')?.textContent).toContain("today's limit");
    expect(button()!.disabled).toBeFalse();
  });
});
