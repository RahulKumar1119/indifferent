import { Injectable } from '@angular/core';
import { HttpClient, HttpContext } from '@angular/common/http';
import { BehaviorSubject, Observable, catchError, finalize, map, of, shareReplay, tap, throwError } from 'rxjs';
import { Router } from '@angular/router';
import { PLATFORM_ID, inject } from '@angular/core';
import { isPlatformBrowser } from '@angular/common';
import { environment } from '../../../environments/environment';
import { AuthTokens } from '../../shared';
import { SUPPRESS_GLOBAL_ERROR } from '../interceptors/error.interceptor';

/**
 * The refresh token lives in an HttpOnly cookie that the API sets (SEC-04), so
 * no script on the page can read it. The access token stays memory-only.
 *
 * localStorage keeps only a marker that a sign-in exists in this browser. It is
 * not a secret: it lets a first-time visitor skip a pointless refresh call, and
 * its value changes on every refresh, which is how one tab notices that another
 * tab has just rotated the cookie.
 */
const SESSION_MARKER_KEY = 'indifferent.session';
/**
 * The key older versions of the app kept the refresh token under. A browser that
 * still has one sends it once, in the body, so its sign-in carries over to the
 * cookie; it is then deleted.
 */
const LEGACY_REFRESH_TOKEN_KEY = 'indifferent.refreshToken';

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
  private refreshInFlight: Observable<AuthTokens> | null = null;

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

  /** The marker for this browser's sign-in, or null when there is none. Changes on every refresh. */
  private sessionMarker(): string | null {
    if (!this.isBrowser) return null;
    try {
      return localStorage.getItem(SESSION_MARKER_KEY);
    } catch {
      return null;
    }
  }

  /** A refresh token an older version of the app left in this browser (migration only). */
  private legacyRefreshToken(): string | null {
    if (!this.isBrowser) return null;
    try {
      return localStorage.getItem(LEGACY_REFRESH_TOKEN_KEY);
    } catch {
      return null;
    }
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
    if (!this.isBrowser) return;
    try {
      // The refresh token is in the cookie the API just set; it is not kept here.
      const previous = localStorage.getItem(SESSION_MARKER_KEY);
      let marker = String(Date.now());
      if (marker === previous) marker += '.1';
      localStorage.setItem(SESSION_MARKER_KEY, marker);
      localStorage.removeItem(LEGACY_REFRESH_TOKEN_KEY);
    } catch {
      // Private-mode quota: the cookie still keeps the sign-in; only the marker is lost.
    }
  }

  refreshToken(): Observable<AuthTokens> {
    // Singleflight: concurrent callers (guard vs interceptor, double clicks)
    // share one in-flight refresh instead of racing the same token.
    if (!this.refreshInFlight) {
      this.refreshInFlight = this.refreshOnce().pipe(
        finalize(() => {
          this.refreshInFlight = null;
        }),
        shareReplay({ bufferSize: 1, refCount: true }),
      );
    }
    return this.refreshInFlight;
  }

  private refreshOnce(retried = false): Observable<AuthTokens> {
    const legacy = this.legacyRefreshToken();
    const markerAtStart = this.sessionMarker();
    if (!legacy && !markerAtStart) {
      // No sign-in in this browser: fail fast instead of calling the API for nothing.
      return throwError(() => new Error('No session to restore'));
    }
    // The cookie goes with the request by itself. A browser that still holds an
    // older version's token sends it once so the sign-in carries over.
    const body = legacy ? { refreshToken: legacy } : {};
    return this.http
      .post<AuthTokens>(`${environment.apiUrl}/auth/refresh`, body, { withCredentials: true })
      .pipe(
        tap((tokens) => this.setTokens(tokens)),
        catchError((err) => {
          // Cross-tab recovery: another tab may have rotated the cookie while this
          // request was in flight, which shows as a changed marker. Retry once
          // instead of treating that conflict as theft (which would end the whole
          // sign-in). A genuine reuse or an expired sign-in still fails.
          const current = this.sessionMarker();
          if (!retried && current && current !== markerAtStart) {
            return this.refreshOnce(true);
          }
          return throwError(() => err);
        }),
      );
  }

  /** POST /auth/signup: register with email + password. */
  signup(email: string, name: string, password: string): Observable<AuthTokens> {
    return this.http
      .post<AuthTokens>(
        `${environment.apiUrl}/auth/signup`,
        { email, name, password },
        { withCredentials: true },
      )
      .pipe(tap((tokens) => this.setTokens(tokens)));
  }

  /** POST /auth/login: sign in with email + password. */
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
      // Server prerender has no browser storage: skip session restore. The
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
    if (!this.isBrowser) return;
    try {
      localStorage.removeItem(SESSION_MARKER_KEY);
      localStorage.removeItem(LEGACY_REFRESH_TOKEN_KEY);
    } catch {
      // Already gone or unavailable: nothing to clear.
    }
  }

  logout(): void {
    // Optimistic: clear local session and leave immediately, even offline or
    // on a hanging request. The server-side revoke is fire-and-forget.
    const legacy = this.legacyRefreshToken();
    this.clearAccessToken();
    this.router.navigate(['/login']);
    // The cookie identifies the session; an older version's token is sent as well if one is left.
    this.http
      .post(`${environment.apiUrl}/auth/logout`, legacy ? { refreshToken: legacy } : {}, { withCredentials: true })
      .subscribe({ error: () => {} });
  }

  /**
   * POST /auth/logout-all: ends every sign-in of this account, on every device.
   * Unlike logout() this waits for the server, because the point is that it
   * happened; local state is only cleared once it did. Other devices lose access
   * when their current access token runs out (up to 15 minutes).
   */
  logoutAll(): Observable<void> {
    return this.http
      .post<void>(`${environment.apiUrl}/auth/logout-all`, {}, { withCredentials: true })
      .pipe(
        tap(() => {
          this.clearAccessToken();
          this.router.navigate(['/login']);
        }),
      );
  }

  /**
   * POST /auth/verify-email: spends the single-use token from the confirmation
   * email. It needs no sign-in (the link may be opened on another device) and a
   * failure is shown by the page itself, so the global error toast is skipped.
   */
  verifyEmail(token: string): Observable<void> {
    return this.http.post<void>(
      `${environment.apiUrl}/auth/verify-email`,
      { token },
      { context: new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true) },
    );
  }

  /** POST /auth/resend-verification: mail a fresh confirmation link to the signed-in account's address. */
  resendVerification(): Observable<void> {
    return this.http.post<void>(
      `${environment.apiUrl}/auth/resend-verification`,
      {},
      { withCredentials: true, context: new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true) },
    );
  }

  isAuthenticated(): boolean {
    return this.accessToken$.getValue() !== null;
  }

  get accessTokenChanges(): Observable<string | null> {
    return this.accessToken$.asObservable();
  }
}
