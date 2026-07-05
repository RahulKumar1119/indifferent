import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, Observable, tap } from 'rxjs';
import { Router } from '@angular/router';
import { environment } from '../../../environments/environment';
import { AuthTokens } from '../../shared';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly accessToken$ = new BehaviorSubject<string | null>(null);

  constructor(
    private readonly http: HttpClient,
    private readonly router: Router,
  ) {}

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

  logout(): void {
    this.http
      .post(`${environment.apiUrl}/auth/logout`, {}, { withCredentials: true })
      .subscribe({
        complete: () => {
          this.accessToken$.next(null);
          this.router.navigate(['/login']);
        },
        error: () => {
          this.accessToken$.next(null);
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
