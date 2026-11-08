import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { provideLucideIcons, LucideMoon, LucideSun, LucideUser } from '@lucide/angular';
import { SettingsComponent } from './settings.component';
import { ThemeService } from '../../core/services/theme.service';
import { environment } from '../../../environments/environment';

const URL = `${environment.apiUrl}/settings/notifications`;
const stored = { email: 'me@example.com', emailVerified: true, notifyCompleted: true, notifyFailed: false };

describe('SettingsComponent', () => {
  function setup() {
    TestBed.configureTestingModule({
      imports: [SettingsComponent],
      providers: [provideHttpClient(), provideHttpClientTesting(), provideRouter([]), provideLucideIcons(LucideMoon, LucideSun, LucideUser)],
    });
    const fixture = TestBed.createComponent(SettingsComponent);
    const http = TestBed.inject(HttpTestingController);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const sw = (id: string) => el.querySelector(`[data-testid="${id}"]`) as HTMLButtonElement;
    return { fixture, http, el, sw };
  }

  it('loads the notification settings with GET and shows them as labelled switches', () => {
    const { fixture, http, el, sw } = setup();
    http.expectOne((r) => r.method === 'GET' && r.url === URL).flush(stored);
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="address"]')!.textContent).toContain('me@example.com');
    expect(sw('notify-completed').getAttribute('aria-checked')).toBe('true');
    expect(sw('notify-failed').getAttribute('aria-checked')).toBe('false');
    expect(sw('notify-completed').getAttribute('aria-labelledby')).toBe('notify-completed-label');
  });

  it('saves one switch with PUT and reports it', () => {
    const { fixture, http, el, sw } = setup();
    http.expectOne(URL).flush(stored);
    fixture.detectChanges();
    sw('notify-failed').click();
    const put = http.expectOne((r) => r.method === 'PUT' && r.url === URL);
    expect(put.request.body).toEqual({ notifyFailed: true });
    put.flush({ ...stored, notifyFailed: true });
    fixture.detectChanges();
    expect(sw('notify-failed').getAttribute('aria-checked')).toBe('true');
    expect(el.querySelector('[data-testid="save-status"]')!.textContent).toContain('Saved.');
  });

  it('puts the switch back and says so when saving fails', () => {
    const { fixture, http, el, sw } = setup();
    http.expectOne(URL).flush(stored);
    fixture.detectChanges();
    sw('notify-completed').click();
    http.expectOne((r) => r.method === 'PUT').flush('no', { status: 500, statusText: 'Server Error' });
    fixture.detectChanges();
    expect(sw('notify-completed').getAttribute('aria-checked')).toBe('true');
    expect(el.querySelector('[data-testid="save-error"]')).toBeTruthy();
  });

  it('keeps switches off and disabled for an unconfirmed address', () => {
    const { fixture, http, el, sw } = setup();
    http.expectOne(URL).flush({ ...stored, emailVerified: false });
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="unverified"]')).toBeTruthy();
    expect(sw('notify-completed').disabled).toBeTrue();
    expect(sw('notify-completed').getAttribute('aria-checked')).toBe('false');
  });

  it('offers a retry when loading fails', () => {
    const { fixture, http, el } = setup();
    http.expectOne(URL).flush('no', { status: 500, statusText: 'Server Error' });
    fixture.detectChanges();
    const retry = Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes('Try again'))!;
    retry.click();
    http.expectOne(URL).flush(stored);
  });

  it('toggles the theme preference through ThemeService', () => {
    const { fixture, http, sw } = setup();
    http.expectOne(URL).flush(stored);
    const theme = TestBed.inject(ThemeService);
    const spy = spyOn(theme, 'toggleTheme').and.callThrough();
    sw('theme-switch').click();
    fixture.detectChanges();
    expect(spy).toHaveBeenCalled();
    expect(sw('theme-switch').getAttribute('aria-checked')).toBe(theme.currentTheme() === 'dark' ? 'true' : 'false');
    theme.toggleTheme();
  });

  it('links Profile, Settings and Account, with one h1', () => {
    const { el, http } = setup();
    http.expectOne(URL).flush(stored);
    const hrefs = Array.from(el.querySelectorAll('app-account-nav nav a')).map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(['/profile', '/profile/settings', '/account']);
    expect(el.querySelectorAll('h1').length).toBe(1);
  });
});
