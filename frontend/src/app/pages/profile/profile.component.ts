import { Component, OnInit, inject } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { catchError, forkJoin, of } from 'rxjs';
import { ApiService } from '../../core/services/api.service';
import { AuthService } from '../../core/services/auth.service';
import { ProjectService } from '../../core/services/project.service';
import { Project } from '../../shared/models/project.model';
import { ShortsJobSummary, ShortsService } from '../shorts/shorts.service';
import { CreatorProfile, emptyCreatorProfile, toUsername } from './models/creator-profile.model';
import { CreatorDNA, DEFAULT_CREATOR_DNA } from './models/creator-dna.model';
import {
  ActivityItem,
  ContentTab,
  CreatorStats,
  CreatorVideo,
  CreditUsage,
  emptyCreatorStats,
} from './models/creator-stats.model';
import { ProfileService, SocialConnection, SocialPlatform } from './services/profile.service';
import { ProfileHeaderComponent } from './components/profile-header.component';
import { CreatorStatsComponent } from './components/creator-stats.component';
import { MyContentComponent, VideoActionEvent } from './components/my-content.component';
import { CreatorDnaComponent } from './components/creator-dna.component';
import { AiCreditsComponent } from './components/ai-credits.component';
import { ConnectedAccountsComponent } from './components/connected-accounts.component';
import { RecentActivityComponent } from './components/recent-activity.component';

const CREDITS_QUOTA = 500;
const COST_PER_PROJECT = 18; // script 2 + voiceover 4 + generation 12
const COST_PER_CLIP = 6;

/**
 * Creator Profile: the creator command center. Identity header, live stats,
 * tabbed content library (projects + shorts), Creator DNA, AI credits,
 * connected accounts and recent activity. Persists personal settings locally
 * and merges live backend data for content and stats.
 */
@Component({
  selector: 'app-profile',
  standalone: true,
  imports: [
    RouterLink,
    LucideDynamicIcon,
    ProfileHeaderComponent,
    CreatorStatsComponent,
    MyContentComponent,
    CreatorDnaComponent,
    AiCreditsComponent,
    ConnectedAccountsComponent,
    RecentActivityComponent,
  ],
  template: `
    <div class="bg-[hsl(var(--background))] text-[hsl(var(--foreground))] antialiased min-h-[100dvh]">
      <div class="aurora-bg" aria-hidden="true"></div>

      <!-- App header -->
      <header class="border-b border-[hsl(var(--border))] bg-[hsl(var(--background))]/90 backdrop-blur sticky top-0 z-40">
        <div class="max-w-7xl mx-auto px-4 md:px-8 h-16 flex items-center justify-between gap-2">
          <a routerLink="/" class="font-semibold tracking-tight text-[17px]">Indifferent<span class="text-[#D96C3D]">.</span></a>
          <nav class="flex items-center gap-1 sm:gap-2 text-[13.5px]" aria-label="Creator sections">
            <a routerLink="/dashboard" class="hidden sm:inline-flex rounded-full px-4 py-2 font-medium hover:bg-white/5 transition-colors">Dashboard</a>
            <a routerLink="/projects" class="hidden sm:inline-flex rounded-full px-4 py-2 font-medium hover:bg-white/5 transition-colors">Projects</a>
            <a routerLink="/shorts/history" class="hidden md:inline-flex rounded-full px-4 py-2 font-medium hover:bg-white/5 transition-colors">Shorts</a>
            <a routerLink="/projects/new" class="btn-interactive inline-flex items-center gap-2 rounded-full bg-[hsl(var(--foreground))] text-[hsl(var(--background))] pl-4 pr-1.5 py-1.5 font-semibold">
              New project
              <span class="w-7 h-7 rounded-full bg-black/10 dark:bg-white/15 flex items-center justify-center">
                <svg lucideIcon="plus" [size]="15"></svg>
              </span>
            </a>
          </nav>
        </div>
      </header>

      <main class="max-w-7xl mx-auto px-4 md:px-8 py-8 md:py-12">
        <div class="flex flex-wrap items-end justify-between gap-3">
          <div>
            <p class="text-[11.5px] uppercase tracking-[0.2em] text-[hsl(var(--muted-foreground))]">Creator · Command Center</p>
            <h1 class="mt-2 font-medium tracking-[-0.02em] leading-none text-[clamp(2rem,4.5vw,3.2rem)]">Profile.</h1>
          </div>
          <div class="flex items-center gap-2">
            <a routerLink="/settings" class="text-[13px] font-medium text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))] underline underline-offset-4">Settings</a>
            <button
              type="button"
              (click)="signOut()"
              class="inline-flex items-center gap-1.5 rounded-full border border-[hsl(var(--border))] px-4 py-2 text-[13px] font-medium hover:bg-white/5"
            >
              <svg lucideIcon="log-out" [size]="14" aria-hidden="true"></svg>
              Sign out
            </button>
          </div>
        </div>

        @if (loadError) {
          <div class="mt-6 rounded-2xl border border-red-500/30 bg-red-500/5 p-6 text-center" role="alert">
            <p class="font-medium text-[15px]">Couldn't load your content</p>
            <p class="mt-1 text-[13.5px] text-[hsl(var(--muted-foreground))]">{{ loadError }}</p>
            <button
              type="button"
              (click)="reload()"
              class="btn-interactive mt-4 inline-flex items-center gap-2 rounded-full bg-[hsl(var(--foreground))] text-[hsl(var(--background))] px-5 py-2.5 text-[13.5px] font-semibold"
            >
              <svg lucideIcon="refresh-cw" [size]="15" aria-hidden="true"></svg>
              Retry
            </button>
          </div>
        }

        <div class="mt-6">
          <app-profile-header
            [profile]="profile"
            [loading]="isLoading"
            (profileChange)="onProfileChange($event)"
          />
        </div>

        <div class="mt-4 md:mt-5">
          <app-creator-stats [stats]="stats" [loading]="isLoading" />
        </div>

        <div class="mt-4 md:mt-5 grid grid-cols-1 xl:grid-cols-[1.6fr_1fr] gap-4 md:gap-5 items-start">
          <app-my-content
            [videos]="videos"
            [activeTab]="activeTab"
            [loading]="isLoading"
            (tabChange)="activeTab = $event"
            (videoAction)="onVideoAction($event)"
          />

          <div class="flex flex-col gap-4 md:gap-5">
            <app-creator-dna
              [dna]="dna"
              [isDefault]="isDefaultDna"
              (dnaChange)="onDnaChange($event)"
              (useDna)="onUseDna()"
            />
            <app-ai-credits
              [stats]="stats"
              [usage]="creditUsage"
              [resetsIn]="creditsResetDays"
              (buy)="notify('Credit purchase is coming soon. Your current quota still applies.', 'info')"
              (upgrade)="notify('Subscription upgrades are coming soon.', 'info')"
            />
            <app-connected-accounts [connections]="social" (toggle)="onToggleSocial($event)" />
            <app-recent-activity [items]="activity" [loading]="isLoading" />
          </div>
        </div>

        <footer class="mt-10 pt-6 border-t border-[hsl(var(--border))] flex flex-col sm:flex-row justify-between gap-2 text-[12.5px] text-[hsl(var(--muted-foreground))]">
          <span>© {{ currentYear }} Indifferent. All rights reserved.</span>
          <span class="flex gap-5">
            <a routerLink="/" class="hover:text-[hsl(var(--foreground))]">Home</a>
            <a routerLink="/dashboard" class="hover:text-[hsl(var(--foreground))]">Dashboard</a>
            <a routerLink="/contact" class="hover:text-[hsl(var(--foreground))]">Contact</a>
          </span>
        </footer>
      </main>

      <!-- Toast -->
      @if (toast) {
        <div
          class="toast-enter fixed top-4 right-4 z-[60] max-w-sm rounded-2xl border border-[hsl(var(--border))] bg-[hsl(var(--card))] px-4 py-3 shadow-2xl flex items-start gap-3"
          role="status"
        >
          <span
            class="w-8 h-8 rounded-full flex items-center justify-center shrink-0"
            [class]="toast.kind === 'error' ? 'bg-red-500/15 text-red-400' : toast.kind === 'success' ? 'bg-emerald-500/15 text-emerald-400' : 'bg-[hsl(var(--primary))]/15 text-[hsl(var(--primary))]'"
            aria-hidden="true"
          >
            <svg lucideIcon="{{ toast.kind === 'error' ? 'x' : toast.kind === 'success' ? 'check' : 'sparkles' }}" [size]="16"></svg>
          </span>
          <p class="text-[13.5px] leading-snug pt-1">{{ toast.message }}</p>
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

  currentYear = new Date().getFullYear();
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
  creditUsage: CreditUsage[] = [];
  creditsResetDays = 12;
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
    this.creditsResetDays = this.daysUntilReset();
    this.reload();
  }

  reload(): void {
    this.isLoading = true;
    this.loadError = '';

    this.store.fetchBackendUser().subscribe((remote) => {
      const merged = this.store.mergeBackendUser(this.profile, remote);
      if (!merged.username && merged.name) merged.username = toUsername(merged.name, merged.email);
      this.profile = this.store.saveProfile(merged);
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
    this.notify('Creator DNA saved. It will be reused for future generations.', 'success');
  }

  onUseDna(): void {
    this.notify(`Generating with your DNA: ${this.dna.niche} · ${this.dna.tone} · ${this.dna.format}.`, 'info');
    this.router.navigate(['/projects/new']);
  }

  onToggleSocial(platform: SocialPlatform): void {
    this.social = this.store.toggleSocial(platform, this.profile.username || this.profile.name);
    this.activity = this.store.loadActivity();
    const conn = this.social.find((c) => c.platform === platform);
    this.notify(
      conn?.connected ? `${this.platformLabel(platform)} connected as ${conn?.handle}.` : `${this.platformLabel(platform)} disconnected.`,
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
    if (['uploaded', 'transcribing', 'ranking', 'rendering'].includes(v.rawStatus)) {
      this.shorts.cancelJob(v.sourceId).subscribe({
        next: () => {
          this.notify('Shorts job cancelled.', 'success');
          this.reload();
        },
        error: () => this.notify('Could not cancel this shorts job.', 'error'),
      });
      return;
    }
    this.notify('Completed shorts are retained automatically. No delete needed.', 'info');
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
        duration: p.status === 'completed' ? 'MP4 · 16:9' : this.stageLabel(p.status),
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
        title: j.clipCount > 0 ? `Short ${j.jobId.slice(0, 8)} · ${j.clipCount} clips` : `Short ${j.jobId.slice(0, 8)}`,
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
    const creditsUsed = videosCreated * COST_PER_PROJECT + totalClips * COST_PER_CLIP;
    this.stats = {
      videosCreated,
      shortsGenerated,
      published,
      favorites,
      creditsUsed,
      creditsQuota: CREDITS_QUOTA,
      minutesSaved: videosCreated * 8 + totalClips * 3,
    };
    const usage: CreditUsage[] = [];
    if (videosCreated > 0) {
      usage.push({ label: `Video Generation × ${videosCreated}`, cost: videosCreated * 12, when: 'This cycle' });
      usage.push({ label: `Voiceover × ${videosCreated}`, cost: videosCreated * 4, when: 'This cycle' });
      usage.push({ label: `AI Script × ${videosCreated}`, cost: videosCreated * 2, when: 'This cycle' });
    }
    if (totalClips > 0) {
      usage.push({ label: `Shorts Clips × ${totalClips}`, cost: totalClips * COST_PER_CLIP, when: 'This cycle' });
    }
    this.creditUsage = usage;
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

  private daysUntilReset(): number {
    const now = new Date();
    const next = new Date(now.getFullYear(), now.getMonth() + 1, 1);
    return Math.max(1, Math.ceil((next.getTime() - now.getTime()) / 86400000));
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
