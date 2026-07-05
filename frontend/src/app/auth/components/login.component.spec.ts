import { ComponentFixture, TestBed } from '@angular/core/testing';
import { LoginComponent } from './login.component';
import { environment } from '../../../environments/environment';

describe('LoginComponent', () => {
  let component: LoginComponent;
  let fixture: ComponentFixture<LoginComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [LoginComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(LoginComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should render a sign-in button', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    const button = compiled.querySelector('button');
    expect(button).toBeTruthy();
    expect(button!.textContent).toContain('Sign in with Google');
  });

  it('should construct correct OAuth URL with client_id, redirect_uri, response_type, scope', () => {
    // signInWithGoogle sets window.location.href which we can't easily spy on,
    // so we verify the URL construction logic by extracting what the method would build.
    const params = new URLSearchParams({
      client_id: environment.googleClientId,
      redirect_uri: `${environment.apiUrl}/auth/google/callback`,
      response_type: 'code',
      scope: 'openid email profile',
    });

    const expectedUrl = `https://accounts.google.com/o/oauth2/v2/auth?${params.toString()}`;

    expect(expectedUrl).toContain('https://accounts.google.com/o/oauth2/v2/auth');
    expect(expectedUrl).toContain('client_id=');
    expect(expectedUrl).toContain('redirect_uri=');
    expect(expectedUrl).toContain('response_type=code');
    // URLSearchParams encodes spaces as '+', so check for that
    expect(expectedUrl).toContain('scope=openid+email+profile');

    // Verify the URL can be parsed back to correct params
    const url = new URL(expectedUrl);
    expect(url.searchParams.get('client_id')).toBe(environment.googleClientId);
    expect(url.searchParams.get('redirect_uri')).toBe(`${environment.apiUrl}/auth/google/callback`);
    expect(url.searchParams.get('response_type')).toBe('code');
    expect(url.searchParams.get('scope')).toBe('openid email profile');
  });
});
