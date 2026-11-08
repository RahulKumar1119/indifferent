import { Component, OnInit, inject } from '@angular/core';
import { Router } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { catchError, forkJoin, of } from 'rxjs';
import { ApiService } from '../../core/services/api.service';
import { AuthService } from '../../core/services/auth.service';
import { ProjectService } from '../../core/services/project.service';
import { Project } from '../../shared/models/project.model';
import { ShortsJobSummary, ShortsService } from '../shorts/shorts.service';
import { CreatorProfile, emptyCreatorProfile } from './models/creator-profile.model';
import { CreatorDNA, DEFAULT_CREATOR_DNA } from './models/creator-dna.model';
import {
  ActivityItem,
  ContentTab,
  CreatorStats,
  CreatorVideo,
  emptyCreatorStats,
} from './models/creator-stats.model';
import { ProfileService, SocialConnection, SocialPlatform } from './services/profile.service';
import { ProfileHeaderComponent } from './components/profile-header.component';
import { CreatorStatsComponent } from './components/creator-stats.component';
import { MyContentComponent, VideoActionEvent } from './components/my-content.component';
import { CreatorDnaComponent } from './components/creator-dna.component';
import { ConnectedAccountsComponent } from './components/connected-accounts.component';
import { RecentActivityComponent } from './components/recent-activity.component';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { AccountNavComponent } from '../account/account-nav.component';

/**
 * Profile: profile details (saved in this browser), live stats, the content
 * library (projects + shorts), Creator DNA, connected accounts and recent
 * activity. Account and security live on /account, preferences on /profile/settings. Persists personal settings locally
 * and merges live backend data for content and stats.
 */
@Component({
  selector: 'app-profile',
  standalone: true,
  imports: [
    LucideDynamicIcon,
    NavbarComponent,
    AccountNavComponent,
    ProfileHeaderComponent,
    CreatorStatsComponent,
    MyContentComponent,
    CreatorDnaComponent,
    ConnectedAccountsComponent,
    RecentActivityComponent,
  ],
  styles: [`
    :host { display: block; }
    .grid { display: grid; gap: var(--space-lg); align-items: start; }
    @media (min-width: 72rem) { .grid { grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr); } }
    .side { display: flex; flex-direction: column; gap: var(--space-lg); min-width: 0; }
    .err { text-align: center; border-left: 4px solid var(--color-negative); }
    .err p { margin: 0; }
    .err p + p { margin-top: var(--space-2xs); color: var(--color-ink-2); font-size: var(--type-sm); }
    .err .ui-btn { margin-top: var(--space-md); }
    .toast { position: fixed; top: var(--space-md); right: var(--space-md); z-index: var(--z-toast); max-width: min(24rem, calc(100% - 2 * var(--space-md))); display: flex; align-items: flex-start; gap: var(--space-sm); padding: var(--space-sm) var(--space-md); border-radius: var(--radius-surface); border: 1px solid var(--color-rule); background: var(--app-card); box-shadow: 0 12px 32px -12px color-mix(in oklab, var(--color-ink) 35%, transparent); font-size: var(--type-sm); }
    .toast p { margin: 0; padding-top: var(--space-2xs); }
    .t-error { color: var(--color-negative); } .t-success { color: var(--color-positive); }
  `],
  template: `
    <div class="ui-app">
      <app-navbar />
      <main id="main" class="ui-app-main hm">
        <app-account-nav title="Profile" lead="How you appear in the studio, and what you have made." />

        @if (loadError) {
          <div class="ui-card err" role="alert">
            <p><strong>Couldn't load your content</strong></p>
            <p>{{ loadError }}</p>
            <button type="button" (click)="reload()" class="ui-btn ui-btn--secondary">
              <svg lucideIcon="refresh-cw" [size]="16" aria-hidden="true"></svg>
              Retry
            </button>
          </div>
        }

        <app-profile-header [profile]="profile" [loading]="isLoading" (profileChange)="onProfileChange($event)" />
        <app-creator-stats [stats]="stats" [loading]="isLoading" />

        <div class="grid">
          <app-my-content
            [videos]="videos"
            [activeTab]="activeTab"
            [loading]="isLoading"
            (tabChange)="activeTab = $event"
            (videoAction)="onVideoAction($event)"
          />
          <div class="side">
            <app-creator-dna [dna]="dna" [isDefault]="isDefaultDna" (dnaChange)="onDnaChange($event)" (useDna)="onUseDna()" />
            <app-connected-accounts [connections]="social" (toggle)="onToggleSocial($event)" />
            <app-recent-activity [items]="activity" [loading]="isLoading" />
          </div>
        </div>
      </main>

      @if (toast) {
        <div class="toast hm" role="status" data-testid="toast">
          <span class="ui-chip" [class.t-error]="toast.kind === 'error'" [class.t-success]="toast.kind === 'success'" aria-hidden="true">
            <svg [lucideIcon]="toast.kind === 'error' ? 'x' : 'check'" [size]="16"></svg>
          </span>
          <p>{{ toast.message }}</p>
        </div>
      }
    </div>
  `,
})
export class ProfileComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);
  private readonly projects = inject(ProjectService);
  private readonly shorts = inject(ShortsService);
  private readonly store = inject(ProfileService);

  isLoading = true;
  loadError = '';

  profile: CreatorProfile = emptyCreatorProfile();
  stats: CreatorStats = emptyCreatorStats();
  videos: CreatorVideo[] = [];
  activeTab: ContentTab = 'all';
  dna: CreatorDNA = { ...DEFAULT_CREATOR_DNA };
  isDefaultDna = true;
  social: SocialConnection[] = [];
  activity: ActivityItem[] = [];
  toast: { message: string; kind: 'info' | 'success' | 'error' } | null = null;

  private projectsCache: Project[] = [];
  private shortsCache: ShortsJobSummary[] = [];
  private toastTimer: ReturnType<typeof setTimeout> | null = null;

  ngOnInit(): void {
    this.social = this.store.loadSocial();
    this.dna = this.store.loadDNA();
    this.isDefaultDna = this.store.isDefaultDNA(this.dna);
    this.profile = this.store.loadProfile();
    this.activity = this.store.loadActivity();
    this.reload();
  }

  reload(): void {
    this.isLoading = true;
    this.loadError = '';

    this.store.fetchBackendUser().subscribe((remote) => {
      const merged = this.store.mergeBackendUser(this.profile, remote);
      this.profile = this.store.saveProfile(merged);
      // Signing in as someone else on this browser clears the saved data; show what is left.
      this.social = this.store.loadSocial();
      this.dna = this.store.loadDNA();
      this.isDefaultDna = this.store.isDefaultDNA(this.dna);
      this.activity = this.store.loadActivity();
    });

    forkJoin({
      projects: this.projects.getProjects().pipe(catchError(() => of(null))),
      shorts: this.shorts.listJobs().pipe(catchError(() => of(null))),
    }).subscribe(({ projects, shorts }) => {
      const projectsFailed = projects === null;
      const shortsFailed = shorts === null;
      this.projectsCache = this.normalizeProjects(projects);
      this.shortsCache = Array.isArray(shorts) ? shorts : [];
      this.rebuild();
      this.isLoading = false;
      if (projectsFailed && shortsFailed) {
        this.loadError = 'We could not reach the studio API. Check your connection and try again.';
      } else if (projectsFailed || shortsFailed) {
        this.notify('Some sections may be incomplete: part of your library failed to load.', 'error');
      }
    });
  }

  // ------------------------------------------------------------ profile edits

  onProfileChange(patch: Partial<CreatorProfile>): void {
    if (patch.username !== undefined) {
      patch.username = patch.username.trim().replace(/^@/, '').toLowerCase();
    }
    this.profile = this.store.saveProfile(patch);
    this.store.logActivity('user', 'Updated profile', this.profile.name);
    this.activity = this.store.loadActivity();
    this.notify('Profile updated.', 'success');
  }

  onDnaChange(dna: CreatorDNA): void {
    this.dna = this.store.saveDNA(dna);
    this.isDefaultDna = this.store.isDefaultDNA(this.dna);
    this.activity = this.store.loadActivity();
    this.notify('Creator DNA saved in this browser.', 'success');
  }

  onUseDna(): void {
    this.router.navigate(['/projects/new']);
  }

  onToggleSocial(platform: SocialPlatform): void {
    this.social = this.store.toggleSocial(platform, this.profile.username || this.profile.name);
    this.activity = this.store.loadActivity();
    const conn = this.social.find((c) => c.platform === platform);
    this.notify(
      conn?.connected ? `${this.platformLabel(platform)} marked connected on this device.` : `${this.platformLabel(platform)} disconnected.`,
      conn?.connected ? 'success' : 'info',
    );
  }

  signOut(): void {
    this.auth.logout();
  }

  // ---------------------------------------------------------- content actions

  onVideoAction({ action, video }: VideoActionEvent): void {
    switch (action) {
      case 'play':
        this.playVideo(video);
        break;
      case 'edit':
        this.editVideo(video);
        break;
      case 'duplicate':
        this.duplicateVideo(video);
        break;
      case 'download':
        this.downloadVideo(video);
        break;
      case 'publish':
        this.publishVideo(video);
        break;
      case 'delete':
        this.deleteVideo(video);
        break;
      case 'favorite': {
        const favs = this.store.toggleFavorite(video.id, video.title);
        this.videos = this.videos.map((v) => (v.id === video.id ? { ...v, favorite: favs.includes(v.id) } : v));
        this.rebuildStats();
        this.activity = this.store.loadActivity();
        this.notify(favs.includes(video.id) ? 'Added to favorites.' : 'Removed from favorites.', 'success');
        break;
      }
    }
  }

  private playVideo(v: CreatorVideo): void {
    if (v.kind === 'video') {
      if (v.rawStatus === 'completed') this.router.navigate(['/projects', v.sourceId, 'preview']);
      else if (v.rawStatus === 'failed' || v.rawStatus === 'created') this.router.navigate(['/projects', v.sourceId]);
      else this.router.navigate(['/projects', v.sourceId, 'progress']);
    } else if (v.rawStatus === 'completed' && (v.clipCount ?? 0) > 0) {
      this.router.navigate(['/shorts', v.sourceId, 'clips']);
    } else {
      this.router.navigate(['/shorts', v.sourceId, 'progress']);
    }
  }

  private editVideo(v: CreatorVideo): void {
    if (v.kind === 'video') {
      this.router.navigate(['/projects', v.sourceId]);
    } else {
      this.notify('Shorts are auto-cut by AI: duplicate the source project to remix.', 'info');
    }
  }

  private duplicateVideo(v: CreatorVideo): void {
    if (v.kind !== 'video') {
      this.notify('Short duplication is coming soon.', 'info');
      return;
    }
    const src = this.projectsCache.find((p) => p.id === v.sourceId);
    if (!src) {
      this.notify('Could not duplicate this video.', 'error');
      return;
    }
    this.projects.createProject({ name: `${src.name} (copy)`, template: src.template, voice: src.voice }).subscribe({
      next: (created) => {
        this.store.logActivity('copy', 'Duplicated video', src.name);
        this.notify('Video duplicated.', 'success');
        this.router.navigate(['/projects', (created as Project).id ?? v.sourceId]);
        this.reload();
      },
      error: () => this.notify('Could not duplicate this video.', 'error'),
    });
  }

  private downloadVideo(v: CreatorVideo): void {
    if (v.kind === 'video') {
      if (v.rawStatus !== 'completed') {
        this.notify('This video is not ready to download yet.', 'info');
        return;
      }
      this.api.get<{ downloadUrl?: string; code?: string; message?: string }>(`/projects/${v.sourceId}/download`).subscribe({
        next: (res) => {
          if (res.downloadUrl) {
            this.store.logActivity('download', 'Downloaded video', v.title);
            this.activity = this.store.loadActivity();
            window.open(res.downloadUrl, '_blank', 'noopener');
          } else {
            this.notify(res.message ?? 'This video is restoring from cold storage. Try again later.', 'info');
          }
        },
        error: () => this.notify('Download is not available for this video yet.', 'error'),
      });
      return;
    }
    this.shorts.listClips(v.sourceId).subscribe({
      next: (clips) => {
        const first = [...clips].sort((a, b) => a.rank - b.rank)[0];
        if (!first) {
          this.notify('No rendered clips yet for this short.', 'info');
          return;
        }
        this.shorts.getClipUrl(v.sourceId, first.clipId).subscribe({
          next: (res) => {
            if (res.url) {
              this.store.logActivity('download', 'Downloaded short', first.hookText || v.title);
              this.activity = this.store.loadActivity();
              window.open(res.url, '_blank', 'noopener');
            } else {
              this.notify(res.message ?? 'This clip is restoring from cold storage. Try again later.', 'info');
            }
          },
          error: () => this.notify('Could not fetch a download link for this short.', 'error'),
        });
      },
      error: () => this.notify('Could not fetch clips for this short.', 'error'),
    });
  }

  private publishVideo(v: CreatorVideo): void {
    this.store.markPublished(v.id, v.title);
    this.videos = this.videos.map((x) => (x.id === v.id ? { ...x, status: 'published' as const } : x));
    this.rebuildStats();
    this.activity = this.store.loadActivity();
    this.notify(`"${v.title}" marked as published.`, 'success');
  }

  private deleteVideo(v: CreatorVideo): void {
    const confirmed = window.confirm(`Delete "${v.title}"? This cannot be undone.`);
    if (!confirmed) return;
    if (v.kind === 'video') {
      this.projects.deleteProject(v.sourceId).subscribe({
        next: () => {
          this.projectsCache = this.projectsCache.filter((p) => p.id !== v.sourceId);
          this.store.logActivity('trash-2', 'Deleted video', v.title);
          this.rebuild();
          this.notify('Video deleted.', 'success');
        },
        error: () => this.notify('Could not delete this video.', 'error'),
      });
      return;
    }
    // Shorts: the backend stops any running pipeline, removes all S3 assets,
    // deletes the record and unlinks projects — for every status.
    this.shorts.deleteJob(v.sourceId).subscribe({
      next: () => {
        this.store.logActivity('trash-2', 'Deleted short', v.title);
        this.notify('Short deleted successfully.', 'success');
        this.reload();
      },
      error: () => this.notify('Could not delete this short.', 'error'),
    });
  }

  // ----------------------------------------------------------------- derived

  private normalizeProjects(raw: unknown): Project[] {
    if (Array.isArray(raw)) return raw as Project[];
    if (raw && typeof raw === 'object' && Array.isArray((raw as { projects?: unknown }).projects)) {
      return (raw as { projects: Project[] }).projects;
    }
    return [];
  }

  private rebuild(): void {
    const favs = new Set(this.store.loadFavorites());
    const published = new Set(this.store.loadPublished());
    const list: CreatorVideo[] = [];

    for (const p of this.projectsCache) {
      const status = published.has(`video:${p.id}`)
        ? 'published'
        : p.status === 'completed'
          ? 'generated'
          : p.status === 'failed'
            ? 'failed'
            : p.status === 'created'
              ? 'draft'
              : 'processing';
      list.push({
        id: `video:${p.id}`,
        title: p.name,
        kind: 'video',
        status,
        rawStatus: p.status,
        createdAt: p.createdAt,
        duration: p.status === 'completed' ? 'Ready' : this.stageLabel(p.status),
        aspectRatio: '16:9',
        thumbnailUrl: p.thumbnailUrl,
        favorite: favs.has(`video:${p.id}`),
        sourceId: p.id,
      });
    }

    for (const j of this.shortsCache) {
      const id = `short:${j.jobId}`;
      const status = published.has(id)
        ? 'published'
        : j.status === 'completed'
          ? 'generated'
          : j.status === 'failed'
            ? 'failed'
            : 'processing';
      list.push({
        id,
        title: `Short ${j.jobId.slice(0, 8)}`,
        kind: 'short',
        status,
        rawStatus: j.status,
        createdAt: j.createdAt,
        duration: this.formatDuration(j.sourceDuration),
        aspectRatio: '9:16',
        clipCount: j.clipCount,
        fileType: j.fileType,
        favorite: favs.has(id),
        sourceId: j.jobId,
      });
    }

    list.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
    this.videos = list;
    this.rebuildStats();
    this.rebuildActivity();
  }

  private rebuildStats(): void {
    const totalClips = this.shortsCache.reduce((n, j) => n + (j.clipCount || 0), 0);
    const shortsGenerated = totalClips > 0 ? totalClips : this.shortsCache.length;
    const published = this.store.loadPublished().length;
    const favorites = this.store.loadFavorites().length;
    const videosCreated = this.projectsCache.length;
    this.stats = {
      videosCreated,
      shortsGenerated,
      published,
      favorites,
      creditsUsed: 0,
      creditsQuota: 0,
      minutesSaved: 0,
    };
  }

  private rebuildActivity(): void {
    const backend: ActivityItem[] = [];
    for (const p of this.projectsCache.slice(0, 8)) {
      backend.push({
        id: `act-video-${p.id}`,
        icon: p.status === 'completed' ? 'circle-check' : 'clapperboard',
        text: p.status === 'completed' ? `Completed "${p.name}"` : `Created "${p.name}"`,
        detail: this.stageLabel(p.status),
        timestamp: p.createdAt,
        group: 'Earlier',
      });
    }
    for (const j of this.shortsCache.slice(0, 8)) {
      backend.push({
        id: `act-short-${j.jobId}`,
        icon: j.status === 'completed' ? 'upload' : 'scissors',
        text: j.status === 'completed' ? `Rendered ${j.clipCount} clips` : 'Started AI short',
        detail: j.jobId.slice(0, 8),
        timestamp: j.createdAt,
        group: 'Earlier',
      });
    }
    const merged = [...this.store.loadActivity(), ...backend]
      .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
      .slice(0, 20)
      .map((a) => ({ ...a, group: this.dayGroup(a.timestamp) }));
    this.activity = merged;
  }

  private dayGroup(iso: string): ActivityItem['group'] {
    const d = new Date(iso);
    if (isNaN(d.getTime())) return 'Earlier';
    const now = new Date();
    const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
    const t = d.getTime();
    if (t >= startOfToday) return 'Today';
    if (t >= startOfToday - 86400000) return 'Yesterday';
    return 'Earlier';
  }

  private stageLabel(status: string): string {
    switch (status) {
      case 'created':
        return 'Draft';
      case 'parsing':
        return 'Parsing';
      case 'generating_slides':
        return 'Generating slides';
      case 'narrating':
        return 'Narrating';
      case 'rendering':
        return 'Rendering';
      case 'completed':
        return 'Completed';
      case 'failed':
        return 'Failed';
      default:
        return status;
    }
  }

  private formatDuration(seconds: number): string {
    if (!seconds || seconds <= 0) return 'Unknown';
    const m = Math.floor(seconds / 60);
    const s = Math.round(seconds % 60);
    return m > 0 ? `${m}m ${s}s` : `${s}s`;
  }

  private platformLabel(p: SocialPlatform): string {
    return p === 'youtube' ? 'YouTube' : p === 'instagram' ? 'Instagram' : 'TikTok';
  }

  notify(message: string, kind: 'info' | 'success' | 'error'): void {
    if (this.toastTimer) clearTimeout(this.toastTimer);
    this.toast = { message, kind };
    this.toastTimer = setTimeout(() => {
      this.toast = null;
      this.toastTimer = null;
    }, 4000);
  }
}
