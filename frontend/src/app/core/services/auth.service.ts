import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, Observable, catchError, finalize, map, of, tap } from 'rxjs';
import { Router } from '@angular/router';
import { environment } from '../../../environments/environment';
import { AuthTokens } from '../../shared';

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

  setTokens(tokens: AuthTokens): void {
    this.accessToken$.next(tokens.accessToken);
    // Refresh token is stored as httpOnly cookie by the API response
  }

  refreshToken(): Observable<AuthTokens> {
    return this.http
      .post<AuthTokens>(`${environment.apiUrl}/auth/refresh`, {}, { withCredentials: true })
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
  }

  logout(): void {
    this.http
      .post(`${environment.apiUrl}/auth/logout`, {}, { withCredentials: true })
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
