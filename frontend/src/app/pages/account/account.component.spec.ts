import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { LucideClock, LucideLogOut, LucideSettings, LucideUser, provideLucideIcons } from '@lucide/angular';
import { AccountComponent } from './account.component';
import { environment } from '../../../environments/environment';

describe('AccountComponent sign out of all devices', () => {
  function setup() {
    TestBed.configureTestingModule({
      imports: [AccountComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
        provideLucideIcons(LucideUser, LucideSettings, LucideClock, LucideLogOut),
      ],
    });
    const fixture = TestBed.createComponent(AccountComponent);
    const http = TestBed.inject(HttpTestingController);
    fixture.detectChanges();
    // The page and the confirmation banner each read the profile.
    http.match(`${environment.apiUrl}/auth/me`).forEach((r) => r.flush({ email: 'a@b.co', name: 'A', avatarUrl: '', createdAt: '2026-01-01T00:00:00Z', emailVerified: true }));
    const button = () =>
      [...(fixture.nativeElement as HTMLElement).querySelectorAll('button')].find((b) => /all devices/i.test(b.textContent ?? ''))!;
    return { fixture, http, button };
  }

  it('has the button and says what it does', () => {
    const { fixture, button } = setup();
    expect(button()).toBeTruthy();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('15 minutes');
  });

  it('calls the API once, even if clicked twice', () => {
    const { fixture, http, button } = setup();
    button().click();
    button().click();
    fixture.detectChanges();
    expect(button().disabled).toBeTrue();
    http.expectOne(`${environment.apiUrl}/auth/logout-all`).flush(null, { status: 204, statusText: 'No Content' });
  });

  it('shows an error and lets the user retry when the call fails', () => {
    const { fixture, http, button } = setup();
    button().click();
    http.expectOne(`${environment.apiUrl}/auth/logout-all`).flush('boom', { status: 500, statusText: 'Server Error' });
    fixture.detectChanges();
    expect(button().disabled).toBeFalse();
    expect((fixture.nativeElement as HTMLElement).querySelector('[role=alert]')?.textContent).toContain('Please try again');
  });
});
