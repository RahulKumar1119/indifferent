import { Component, inject, OnInit, signal } from '@angular/core';
import { DatePipe } from '@angular/common';
import { LucideDynamicIcon } from '@lucide/angular';
import { ApiService } from '../../core/services/api.service';
import { AuthService } from '../../core/services/auth.service';
import { UserProfile } from '../../shared/models/user-profile.model';

@Component({
  selector: 'app-account',
  standalone: true,
  imports: [DatePipe, LucideDynamicIcon],
  template: `
    <div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-6 text-2xl font-bold">Account</h1>

      <div class="glass-card p-6 mb-6">
        <div class="flex items-center gap-6">
          <img
            [src]="profile().avatarUrl || 'https://via.placeholder.com/80'"
            [alt]="profile().name + ' avatar'"
            class="h-20 w-20 rounded-full object-cover border-2 border-[hsl(var(--primary))]/30"
          />
          <div>
            <h2 class="text-xl font-semibold">{{ profile().name }}</h2>
            <p class="text-sm text-[hsl(var(--muted-foreground))]">{{ profile().email }}</p>
          </div>
        </div>
      </div>

      <div class="glass-card p-6 mb-6">
        <div class="space-y-4">
          <div class="flex items-center gap-4 p-3 rounded-lg hover:bg-white/5 transition-colors">
            <div class="w-9 h-9 rounded-lg bg-[hsl(var(--primary))]/10 flex items-center justify-center">
              <svg lucideIcon="user" [size]="18" class="text-[hsl(var(--primary))]"></svg>
            </div>
            <div>
              <p class="text-sm text-[hsl(var(--muted-foreground))]">Name</p>
              <p class="font-medium">{{ profile().name }}</p>
            </div>
          </div>
          <div class="flex items-center gap-4 p-3 rounded-lg hover:bg-white/5 transition-colors">
            <div class="w-9 h-9 rounded-lg bg-[hsl(var(--primary))]/10 flex items-center justify-center">
              <svg lucideIcon="settings" [size]="18" class="text-[hsl(var(--primary))]"></svg>
            </div>
            <div>
              <p class="text-sm text-[hsl(var(--muted-foreground))]">Email</p>
              <p class="font-medium">{{ profile().email }}</p>
            </div>
          </div>
          <div class="flex items-center gap-4 p-3 rounded-lg hover:bg-white/5 transition-colors">
            <div class="w-9 h-9 rounded-lg bg-[hsl(var(--primary))]/10 flex items-center justify-center">
              <svg lucideIcon="clock" [size]="18" class="text-[hsl(var(--primary))]"></svg>
            </div>
            <div>
              <p class="text-sm text-[hsl(var(--muted-foreground))]">Member since</p>
              <p class="font-medium">{{ profile().createdAt | date: 'mediumDate' }}</p>
            </div>
          </div>
        </div>
      </div>

      <button
        class="inline-flex items-center gap-2 px-5 py-3 rounded-lg border border-red-500/30 text-red-400 hover:bg-red-500/10 transition-colors font-medium"
        (click)="signOut()"
      >
        <svg lucideIcon="log-out" [size]="18"></svg>
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
