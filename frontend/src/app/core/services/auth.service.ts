import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, Observable, catchError, finalize, map, of, tap, throwError } from 'rxjs';
import { Router } from '@angular/router';
import { PLATFORM_ID, inject } from '@angular/core';
import { isPlatformBrowser } from '@angular/common';
import { environment } from '../../../environments/environment';
import { AuthTokens } from '../../shared';

/** localStorage key for the refresh token. The access token stays memory-only. */
const REFRESH_TOKEN_KEY = 'indifferent.refreshToken';

/** sessionStorage key for the post-auth destination (?next= passthrough). */
const POST_LOGIN_NEXT_KEY = 'indifferent.postLoginNext';

/** Fallback landing after signup/login: the feature picker. */
export const DEFAULT_POST_AUTH_PATH = '/new';

/** Only same-origin absolute paths survive; everything else falls back. */
export function sanitizeNextPath(raw: string | null): string {
  if (raw && raw.startsWith('/') && !raw.startsWith('//')) {
    return raw;
  }
  return DEFAULT_POST_AUTH_PATH;
}

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly accessToken$ = new BehaviorSubject<string | null>(null);
  private readonly initialized$ = new BehaviorSubject<boolean>(false);
  private readonly isBrowser = isPlatformBrowser(inject(PLATFORM_ID));

  constructor(
    private readonly http: HttpClient,
    private readonly router: Router,
  ) {}

  /** Resolves once the initial session restore has finished (success or fail). */
  get authReady$(): Observable<boolean> {
    return this.initialized$.asObservable();
  }

  getAccessToken(): string | null {
    return this.accessToken$.getValue();
  }

  /** Refresh token persisted across reloads (the API expects it in the request body). */
  getRefreshToken(): string | null {
    return localStorage.getItem(REFRESH_TOKEN_KEY);
  }

  /** Remember where to land after the OAuth round-trip (survives Google). */
  setPostLoginNext(path: string): void {
    sessionStorage.setItem(POST_LOGIN_NEXT_KEY, path);
  }

  /** Read + clear the remembered post-auth destination (safe default). */
  consumePostLoginNext(): string {
    const raw = sessionStorage.getItem(POST_LOGIN_NEXT_KEY);
    sessionStorage.removeItem(POST_LOGIN_NEXT_KEY);
    return sanitizeNextPath(raw);
  }

  setTokens(tokens: AuthTokens): void {
    this.accessToken$.next(tokens.accessToken);
    localStorage.setItem(REFRESH_TOKEN_KEY, tokens.refreshToken);
  }

  refreshToken(): Observable<AuthTokens> {
    const refreshToken = this.getRefreshToken();
    if (!refreshToken) {
      // No session to restore — fail fast instead of sending {}
      // (the API rejects it with 400 INVALID_INPUT).
      return throwError(() => new Error('No refresh token available'));
    }
    return this.http
      .post<AuthTokens>(
        `${environment.apiUrl}/auth/refresh`,
        { refreshToken },
        { withCredentials: true },
      )
      .pipe(tap((tokens) => this.setTokens(tokens)));
  }

  /** POST /auth/signup — register with email + password. */
  signup(email: string, name: string, password: string): Observable<AuthTokens> {
    return this.http
      .post<AuthTokens>(
        `${environment.apiUrl}/auth/signup`,
        { email, name, password },
        { withCredentials: true },
      )
      .pipe(tap((tokens) => this.setTokens(tokens)));
  }

  /** POST /auth/login — sign in with email + password. */
  loginWithPassword(email: string, password: string): Observable<AuthTokens> {
    return this.http
      .post<AuthTokens>(
        `${environment.apiUrl}/auth/login`,
        { email, password },
        { withCredentials: true },
      )
      .pipe(tap((tokens) => this.setTokens(tokens)));
  }

  /**
   * Best-effort session restore on app boot. Swallows errors so
   * APP_INITIALIZER never blocks bootstrap on 401/network failure.
   */
  initialize(): Observable<AuthTokens | null> {
    if (!this.isBrowser) {
      // Server prerender has no localStorage: skip session restore. The
      // client hydrates and restores the session on boot instead.
      this.initialized$.next(true);
      return of(null);
    }
    if (this.getAccessToken() !== null) {
      this.initialized$.next(true);
      return of(null);
    }
    return this.refreshToken().pipe(
      map((tokens) => tokens as AuthTokens | null),
      catchError(() => {
        this.clearAccessToken();
        return of(null);
      }),
      finalize(() => this.initialized$.next(true)),
    );
  }

  clearAccessToken(): void {
    this.accessToken$.next(null);
    localStorage.removeItem(REFRESH_TOKEN_KEY);
  }

  logout(): void {
    const refreshToken = this.getRefreshToken();
    this.http
      .post(`${environment.apiUrl}/auth/logout`, { refreshToken }, { withCredentials: true })
      .subscribe({
        complete: () => {
          this.clearAccessToken();
          this.router.navigate(['/login']);
        },
        error: () => {
          this.clearAccessToken();
          this.router.navigate(['/login']);
        },
      });
  }

  isAuthenticated(): boolean {
    return this.accessToken$.getValue() !== null;
  }

  get accessTokenChanges(): Observable<string | null> {
    return this.accessToken$.asObservable();
  }
}
