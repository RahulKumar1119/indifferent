import { Component, OnInit, inject } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { HttpClient } from '@angular/common/http';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { environment } from '../../../environments/environment';
import { AuthService } from '../../core/services/auth.service';
import { AuthTokens } from '../../shared';

@Component({
  selector: 'app-auth-callback',
  standalone: true,
  imports: [MatProgressSpinnerModule],
  template: `
    <div class="flex min-h-screen items-center justify-center bg-gray-50">
      <mat-spinner diameter="48"></mat-spinner>
    </div>
  `,
})
export class AuthCallbackComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly http = inject(HttpClient);
  private readonly authService = inject(AuthService);

  ngOnInit(): void {
    const code = this.route.snapshot.queryParamMap.get('code');

    if (!code) {
      this.router.navigate(['/login'], { queryParams: { error: 'missing_code' } });
      return;
    }

    this.http
      .post<AuthTokens>(`${environment.apiUrl}/auth/google/callback`, { code }, { withCredentials: true })
      .subscribe({
        next: (tokens) => {
          this.authService.setTokens(tokens);
          this.router.navigate(['/dashboard']);
        },
        error: () => {
          this.router.navigate(['/login'], { queryParams: { error: 'auth_failed' } });
        },
      });
  }
}
