import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, Observable, catchError, finalize, map, of, tap, throwError } from 'rxjs';
import { Router } from '@angular/router';
import { environment } from '../../../environments/environment';
import { AuthTokens } from '../../shared';

/** localStorage key for the refresh token. The access token stays memory-only. */
const REFRESH_TOKEN_KEY = 'indifferent.refreshToken';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly accessToken$ = new BehaviorSubject<string | null>(null);
  private readonly initialized$ = new BehaviorSubject<boolean>(false);

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

  /**
   * Best-effort session restore on app boot. Swallows errors so
   * APP_INITIALIZER never blocks bootstrap on 401/network failure.
   */
  initialize(): Observable<AuthTokens | null> {
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
