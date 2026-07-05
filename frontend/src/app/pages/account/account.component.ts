import { Component, inject, OnInit, signal } from '@angular/core';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatListModule } from '@angular/material/list';
import { MatIconModule } from '@angular/material/icon';
import { DatePipe } from '@angular/common';
import { ApiService } from '../../core/services/api.service';
import { AuthService } from '../../core/services/auth.service';
import { UserProfile } from '../../shared/models/user-profile.model';

@Component({
  selector: 'app-account',
  standalone: true,
  imports: [MatCardModule, MatButtonModule, MatListModule, MatIconModule, DatePipe],
  template: `
    <div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-6 text-2xl font-bold">Account</h1>

      <mat-card class="mb-6">
        <mat-card-content class="flex items-center gap-6 p-6">
          <img
            [src]="profile().avatarUrl || 'https://via.placeholder.com/80'"
            [alt]="profile().name + ' avatar'"
            class="h-20 w-20 rounded-full object-cover"
          />
          <div>
            <h2 class="text-xl font-semibold">{{ profile().name }}</h2>
            <p class="text-sm text-gray-600 dark:text-gray-400">{{ profile().email }}</p>
          </div>
        </mat-card-content>
      </mat-card>

      <mat-card class="mb-6">
        <mat-card-content class="p-6">
          <mat-list>
            <mat-list-item>
              <mat-icon matListItemIcon>email</mat-icon>
              <span matListItemTitle>Email</span>
              <span matListItemLine>{{ profile().email }}</span>
            </mat-list-item>
            <mat-list-item>
              <mat-icon matListItemIcon>person</mat-icon>
              <span matListItemTitle>Name</span>
              <span matListItemLine>{{ profile().name }}</span>
            </mat-list-item>
            <mat-list-item>
              <mat-icon matListItemIcon>calendar_today</mat-icon>
              <span matListItemTitle>Member since</span>
              <span matListItemLine>{{ profile().createdAt | date: 'mediumDate' }}</span>
            </mat-list-item>
          </mat-list>
        </mat-card-content>
      </mat-card>

      <button mat-raised-button color="warn" (click)="signOut()">
        <mat-icon>logout</mat-icon>
        Sign Out
      </button>
    </div>
  `,
})
export class AccountComponent implements OnInit {
  private readonly apiService = inject(ApiService);
  private readonly authService = inject(AuthService);

  readonly profile = signal<UserProfile>({
    email: '',
    name: '',
    avatarUrl: '',
    createdAt: '',
  });

  ngOnInit(): void {
    this.apiService.get<UserProfile>('/auth/me').subscribe({
      next: (user) => this.profile.set(user),
      error: () => {
        // Fallback if endpoint not ready
      },
    });
  }

  signOut(): void {
    this.authService.logout();
  }
}
