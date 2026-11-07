import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { ActivatedRoute, Router, convertToParamMap, provideRouter } from '@angular/router';
import { AuthService } from '../../core/services/auth.service';
import { VerifyEmailComponent } from './verify-email.component';
import { environment } from '../../../environments/environment';

describe('VerifyEmailComponent', () => {
  function setup(query: Record<string, string>, signedIn = false) {
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
    if (signedIn) TestBed.inject(AuthService).setTokens({ accessToken: 'at', refreshToken: 'rt', expiresIn: 3600 });
    const router = TestBed.inject(Router);
    const navigate = spyOn(router, 'navigate').and.resolveTo(true);
    const http = TestBed.inject(HttpTestingController);
    const fixture = TestBed.createComponent(VerifyEmailComponent);
    fixture.detectChanges();
    const text = () => (fixture.nativeElement as HTMLElement).textContent ?? '';
    return { fixture, http, navigate, text };
  }

  afterEach(() => localStorage.clear());

  it('takes the token out of the address bar before anything else', () => {
    const { http, navigate } = setup({ token: 'abc' });
    expect(navigate).toHaveBeenCalledWith([], { queryParams: {}, replaceUrl: true });
    http.expectOne(`${environment.apiUrl}/auth/verify-email`).flush(null, { status: 204, statusText: 'No Content' });
  });

  it('sends the token once and says the address is confirmed', () => {
    const { fixture, http, text } = setup({ token: 'the-token' });
    expect(text()).toContain('Confirming your email address');
    const req = http.expectOne(`${environment.apiUrl}/auth/verify-email`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({ token: 'the-token' });
    req.flush(null, { status: 204, statusText: 'No Content' });
    fixture.detectChanges();
    expect(text()).toContain('Your email address is confirmed');
    expect(text()).toContain('Sign in'); // not signed in on this device
  });

  it('offers the dashboard to someone who is signed in', () => {
    const { fixture, http, text } = setup({ token: 't' }, true);
    http.expectOne(`${environment.apiUrl}/auth/verify-email`).flush(null, { status: 204, statusText: 'No Content' });
    fixture.detectChanges();
    expect(text()).toContain('Go to your dashboard');
  });

  it("shows the server's reason when the link is used up or expired", () => {
    const { fixture, http, text } = setup({ token: 'old' });
    http
      .expectOne(`${environment.apiUrl}/auth/verify-email`)
      .flush({ code: 'INVALID_TOKEN', message: 'This link has expired or was already used.' }, { status: 400, statusText: 'Bad Request' });
    fixture.detectChanges();
    expect(text()).toContain('This link did not work');
    expect(text()).toContain('This link has expired or was already used.');
  });

  it('does not call the server when the link has no token', () => {
    const { fixture, http, text } = setup({});
    http.expectNone(`${environment.apiUrl}/auth/verify-email`);
    fixture.detectChanges();
    expect(text()).toContain('This link did not work');
    expect(text()).toContain('incomplete');
  });

  it('keeps search engines away', () => {
    setup({ token: 'x' });
    expect(document.querySelector('meta[name="robots"]')?.getAttribute('content')).toContain('noindex');
  });
});
