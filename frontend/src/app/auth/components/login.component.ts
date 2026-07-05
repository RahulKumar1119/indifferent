import { Component } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';
import { environment } from '../../../environments/environment';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [MatButtonModule, MatCardModule, MatIconModule],
  template: `
    <div class="flex min-h-screen items-center justify-center bg-gray-50">
      <mat-card class="w-full max-w-md p-8">
        <mat-card-header class="mb-6 justify-center">
          <mat-card-title class="text-2xl font-bold text-center">Welcome</mat-card-title>
          <mat-card-subtitle class="text-center">Sign in to continue</mat-card-subtitle>
        </mat-card-header>
        <mat-card-content class="flex justify-center">
          <button
            mat-raised-button
            color="primary"
            class="w-full"
            (click)="signInWithGoogle()"
          >
            <mat-icon>login</mat-icon>
            Sign in with Google
          </button>
        </mat-card-content>
      </mat-card>
    </div>
  `,
})
export class LoginComponent {
  signInWithGoogle(): void {
    const params = new URLSearchParams({
      client_id: environment.googleClientId,
      redirect_uri: `${environment.apiUrl}/auth/google/callback`,
      response_type: 'code',
      scope: 'openid email profile',
    });

    window.location.href = `https://accounts.google.com/o/oauth2/v2/auth?${params.toString()}`;
  }
}
