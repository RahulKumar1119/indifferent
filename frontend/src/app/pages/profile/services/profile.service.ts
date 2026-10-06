import { Injectable, PLATFORM_ID, inject } from '@angular/core';
import { isPlatformBrowser } from '@angular/common';
import { Observable, catchError, map, of } from 'rxjs';
import { ApiService } from '../../../core/services/api.service';
import { UserProfile } from '../../../shared/models/user-profile.model';
import {
  CreatorProfile,
  emptyCreatorProfile,
  toUsername,
} from '../models/creator-profile.model';
import {
  CREATOR_DNA_OPTIONS,
  CreatorDNA,
  DEFAULT_CREATOR_DNA,
} from '../models/creator-dna.model';
import { ActivityItem } from '../models/creator-stats.model';

const LS_PROFILE = 'indifferent.creatorProfile.v1';
const LS_DNA = 'indifferent.creatorDna.v1';
const LS_SOCIAL = 'indifferent.creatorSocial.v1';
const LS_FAVORITES = 'indifferent.creatorFavorites.v1';
const LS_PUBLISHED = 'indifferent.creatorPublished.v1';
const LS_ACTIVITY = 'indifferent.creatorActivity.v1';

export type SocialPlatform = 'youtube' | 'instagram' | 'tiktok';

export interface SocialConnection {
  platform: SocialPlatform;
  connected: boolean;
  handle: string;
  connectedAt?: string;
}

export const SOCIAL_PLATFORMS: { platform: SocialPlatform; label: string; icon: string }[] = [
  { platform: 'youtube', label: 'YouTube', icon: 'square-play' },
  { platform: 'instagram', label: 'Instagram', icon: 'camera' },
  { platform: 'tiktok', label: 'TikTok', icon: 'music-2' },
];

/**
 * Profile state lives in localStorage (per-browser) and merges with the
 * backend user record when GET /auth/me is available. Avatar images are
 * stored as data URLs; everything else is plain JSON. All reads are
 * SSR-safe (server prerender gets defaults, the client hydrates after).
 */
@Injectable({ providedIn: 'root' })
export class ProfileService {
  private readonly api = inject(ApiService);
  private readonly isBrowser = isPlatformBrowser(inject(PLATFORM_ID));

  // ---------------------------------------------------------------- profile

  /** Best-effort backend user; null when the endpoint is not deployed yet. */
  fetchBackendUser(): Observable<UserProfile | null> {
    return this.api.get<UserProfile>('/auth/me').pipe(
      map((u) => u ?? null),
      catchError(() => of(null)),
    );
  }

  loadProfile(): CreatorProfile {
    const stored = this.read<CreatorProfile>(LS_PROFILE);
    return { ...emptyCreatorProfile(), ...(stored ?? {}) };
  }

  saveProfile(patch: Partial<CreatorProfile>): CreatorProfile {
    const next = { ...this.loadProfile(), ...patch };
    this.write(LS_PROFILE, next);
    return next;
  }

  /** Merge backend identity into the local profile (backend wins for name/email/avatar when present). */
  mergeBackendUser(local: CreatorProfile, remote: UserProfile | null): CreatorProfile {
    if (!remote) return local;
    const merged: CreatorProfile = {
      ...local,
      name: remote.name || local.name,
      email: remote.email || local.email,
      avatarUrl: local.avatarUrl || remote.avatarUrl || '',
      createdAt: remote.createdAt || local.createdAt,
    };
    if (!local.username && merged.name) merged.username = toUsername(merged.name, merged.email);
    if (!merged.bio) {
      merged.bio = 'Creating AI-powered Shorts with Indifferent';
    }
    return merged;
  }

  // -------------------------------------------------------------- creator DNA

  loadDNA(): CreatorDNA {
    const stored = this.read<Partial<CreatorDNA>>(LS_DNA);
    return { ...DEFAULT_CREATOR_DNA, ...(stored ?? {}) };
  }

  saveDNA(dna: CreatorDNA): CreatorDNA {
    const clean: CreatorDNA = { ...dna };
    (Object.keys(clean) as (keyof CreatorDNA)[]).forEach((k) => {
      const allowed = CREATOR_DNA_OPTIONS[k];
      if (!allowed.includes(clean[k])) clean[k] = DEFAULT_CREATOR_DNA[k];
    });
    this.write(LS_DNA, clean);
    this.logActivity('sparkles', 'Updated Creator DNA', clean.niche + ' · ' + clean.tone);
    return clean;
  }

  isDefaultDNA(dna: CreatorDNA): boolean {
    return (Object.keys(DEFAULT_CREATOR_DNA) as (keyof CreatorDNA)[]).every(
      (k) => dna[k] === DEFAULT_CREATOR_DNA[k],
    );
  }

  // -------------------------------------------------------- social accounts

  loadSocial(): SocialConnection[] {
    const stored = this.read<SocialConnection[]>(LS_SOCIAL);
    if (Array.isArray(stored) && stored.length > 0) return stored;
    return SOCIAL_PLATFORMS.map((s) => ({
      platform: s.platform,
      connected: false,
      handle: '',
    }));
  }

  /** Toggle a connection. Connecting assigns a placeholder handle from the username. */
  toggleSocial(platform: SocialPlatform, username: string): SocialConnection[] {
    const next = this.loadSocial().map((c) => {
      if (c.platform !== platform) return c;
      if (c.connected) return { ...c, connected: false, handle: '', connectedAt: undefined };
      const handle = '@' + toUsername(username || 'creator');
      const entry: SocialConnection = {
        ...c,
        connected: true,
        handle,
        connectedAt: new Date().toISOString(),
      };
      this.logActivity(
        'share-2',
        `Connected ${platform === 'youtube' ? 'YouTube' : platform === 'instagram' ? 'Instagram' : 'TikTok'}`,
        handle,
      );
      return entry;
    });
    this.write(LS_SOCIAL, next);
    return next;
  }

  // ----------------------------------------------------- favorites / publish

  loadFavorites(): string[] {
    const v = this.read<string[]>(LS_FAVORITES);
    return Array.isArray(v) ? v : [];
  }

  toggleFavorite(id: string, title: string): string[] {
    const favs = this.loadFavorites();
    const next = favs.includes(id) ? favs.filter((f) => f !== id) : [...favs, id];
    this.write(LS_FAVORITES, next);
    if (!favs.includes(id)) this.logActivity('star', 'Added video to favorites', title);
    return next;
  }

  loadPublished(): string[] {
    const v = this.read<string[]>(LS_PUBLISHED);
    return Array.isArray(v) ? v : [];
  }

  markPublished(id: string, title: string): string[] {
    const list = this.loadPublished();
    if (list.includes(id)) return list;
    const next = [...list, id];
    this.write(LS_PUBLISHED, next);
    this.logActivity('upload', 'Published video', title);
    return next;
  }

  // ---------------------------------------------------------------- activity

  loadActivity(): ActivityItem[] {
    const v = this.read<ActivityItem[]>(LS_ACTIVITY);
    return Array.isArray(v) ? v : [];
  }

  logActivity(icon: string, text: string, detail = ''): ActivityItem[] {
    const entry: ActivityItem = {
      id: `${Date.now()}-${Math.floor(Math.random() * 1e6)}`,
      icon,
      text,
      detail,
      timestamp: new Date().toISOString(),
      group: 'Today',
    };
    const next = [entry, ...this.loadActivity()].slice(0, 60);
    this.write(LS_ACTIVITY, next);
    return next;
  }

  // ------------------------------------------------------------------ helpers

  private read<T>(key: string): T | null {
    if (!this.isBrowser) return null;
    try {
      const raw = localStorage.getItem(key);
      return raw ? (JSON.parse(raw) as T) : null;
    } catch {
      return null;
    }
  }

  private write(key: string, value: unknown): void {
    if (!this.isBrowser) return;
    try {
      localStorage.setItem(key, JSON.stringify(value));
    } catch {
      // Quota exceeded (large avatar data URL): keep memory-only behavior.
    }
  }
}
