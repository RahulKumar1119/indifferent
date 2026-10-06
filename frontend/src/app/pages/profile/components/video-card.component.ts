import { Component, input, output } from '@angular/core';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { CreatorVideo } from '../models/creator-stats.model';

export type VideoCardAction =
  | 'play'
  | 'edit'
  | 'duplicate'
  | 'download'
  | 'publish'
  | 'delete'
  | 'favorite';

/** A single content card: thumbnail, meta, status and an overflow action menu. */
@Component({
  selector: 'app-video-card',
  standalone: true,
  imports: [RouterLink, LucideDynamicIcon],
  template: `
    <article
      class="glass-card group overflow-hidden rounded-2xl transition-all duration-200 hover:-translate-y-1 hover:shadow-xl"
      [attr.aria-label]="video().title"
    >
      <!-- Thumbnail -->
      <div class="relative aspect-video overflow-hidden bg-[hsl(var(--secondary))]">
        @if (video().thumbnailUrl) {
          <img
            [src]="video().thumbnailUrl"
            [alt]="video().title + ' thumbnail'"
            loading="lazy"
            class="h-full w-full object-cover"
          />
        } @else {
          <div
            class="flex h-full w-full items-center justify-center"
            [class]="video().kind === 'short' ? 'bg-gradient-to-br from-violet-600/30 via-fuchsia-500/20 to-orange-400/20' : 'bg-gradient-to-br from-indigo-600/25 via-sky-500/15 to-emerald-400/15'"
            aria-hidden="true"
          >
            <svg
              lucideIcon="{{ video().kind === 'short' ? 'scissors' : 'clapperboard' }}"
              [size]="36"
              class="text-[hsl(var(--muted-foreground))] opacity-60"
            ></svg>
          </div>
        }
        <button
          type="button"
          (click)="action.emit('play')"
          class="absolute inset-0 flex items-center justify-center bg-black/0 hover:bg-black/35 transition-colors"
          [attr.aria-label]="'Preview ' + video().title"
        >
          <span
            class="w-12 h-12 rounded-full bg-white/90 text-black flex items-center justify-center opacity-0 group-hover:opacity-100 scale-90 group-hover:scale-100 transition-all"
            aria-hidden="true"
          >
            <svg lucideIcon="play" [size]="20"></svg>
          </span>
        </button>
        <span
          class="absolute top-2.5 left-2.5 rounded-full px-2.5 py-1 text-[11px] font-semibold backdrop-blur"
          [class]="statusClasses()"
          aria-hidden="true"
        >
          {{ statusLabel() }}
        </span>
        <button
          type="button"
          (click)="action.emit('favorite')"
          class="absolute top-2 right-2 w-8 h-8 rounded-full bg-black/55 backdrop-blur flex items-center justify-center transition-transform hover:scale-110"
          [attr.aria-label]="video().favorite ? 'Remove from favorites' : 'Add to favorites'"
          [attr.aria-pressed]="video().favorite"
        >
          <svg
            lucideIcon="star"
            [size]="15"
            [class]="video().favorite ? 'text-amber-400 fill-amber-400' : 'text-white'"
          ></svg>
        </button>
        @if (video().kind === 'short') {
          <span
            class="absolute bottom-2.5 right-2.5 rounded-md bg-black/65 text-white text-[11px] px-2 py-0.5"
            aria-hidden="true"
            >9:16</span
          >
        }
      </div>

      <!-- Body -->
      <div class="p-4">
        <h3 class="font-medium text-[14.5px] leading-snug line-clamp-2 min-h-[2.6em]">
          {{ video().title }}
        </h3>
        <p class="mt-1 text-[12.5px] text-[hsl(var(--muted-foreground))]">
          {{ video().duration }} · {{ video().aspectRatio }}
          @if (video().clipCount) {
            · {{ video().clipCount }} {{ video().clipCount === 1 ? 'clip' : 'clips' }}
          }
        </p>
        <div class="mt-3 flex items-center gap-2">
          @if (playLink(); as link) {
            <a
              [routerLink]="link"
              class="btn-interactive flex-1 inline-flex items-center justify-center gap-1.5 rounded-full bg-[hsl(var(--foreground))] text-[hsl(var(--background))] py-2 text-[13px] font-semibold"
            >
              <svg lucideIcon="play" [size]="13" aria-hidden="true"></svg>
              {{ video().status === 'draft' || video().status === 'processing' ? 'Track' : 'Open' }}
            </a>
          } @else {
            <button
              type="button"
              (click)="action.emit('play')"
              class="btn-interactive flex-1 inline-flex items-center justify-center gap-1.5 rounded-full bg-[hsl(var(--foreground))] text-[hsl(var(--background))] py-2 text-[13px] font-semibold"
            >
              <svg lucideIcon="play" [size]="13" aria-hidden="true"></svg>
              Open
            </button>
          }
          <div class="relative">
            <button
              type="button"
              (click)="menuOpen = !menuOpen"
              class="btn-interactive w-9 h-9 rounded-full border border-[hsl(var(--border))] inline-flex items-center justify-center"
              aria-label="More actions"
              aria-haspopup="menu"
              [attr.aria-expanded]="menuOpen"
            >
              <svg lucideIcon="more-vertical" [size]="16"></svg>
            </button>
            @if (menuOpen) {
              <div
                class="absolute right-0 bottom-11 z-20 w-44 overflow-hidden rounded-xl border border-[hsl(var(--border))] bg-[hsl(var(--card))] shadow-2xl py-1"
                role="menu"
              >
                @for (item of menuItems(); track item.action) {
                  <button
                    type="button"
                    role="menuitem"
                    (click)="menuOpen = false; action.emit(item.action)"
                    class="flex w-full items-center gap-2.5 px-4 py-2.5 text-[13.5px] hover:bg-white/5 text-left"
                    [class.text-red-400]="item.action === 'delete'"
                  >
                    <svg lucideIcon="{{ item.icon }}" [size]="15" aria-hidden="true"></svg>
                    {{ item.label }}
                  </button>
                }
              </div>
            }
          </div>
        </div>
      </div>
    </article>
  `,
})
export class VideoCardComponent {
  readonly video = input.required<CreatorVideo>();
  readonly action = output<VideoCardAction>();

  menuOpen = false;

  statusLabel(): string {
    switch (this.video().status) {
      case 'published':
        return '✓ Published';
      case 'generated':
        return '✓ Generated';
      case 'draft':
        return '○ Draft';
      case 'processing':
        return '◌ Processing';
      case 'failed':
        return '✕ Failed';
    }
  }

  statusClasses(): string {
    const base = 'bg-black/55 text-white ';
    switch (this.video().status) {
      case 'published':
        return base + 'text-emerald-300';
      case 'generated':
        return base + 'text-sky-300';
      case 'draft':
        return base + 'text-white/80';
      case 'processing':
        return base + 'text-amber-300';
      case 'failed':
        return base + 'text-red-300';
    }
  }

  /** Deep link when one exists; otherwise the parent handles the action. */
  playLink(): string[] | null {
    const v = this.video();
    if (v.kind === 'video') {
      if (v.status === 'draft' || v.status === 'processing') return ['/projects', v.sourceId, 'progress'];
      if (v.status === 'failed') return ['/projects', v.sourceId];
      return ['/projects', v.sourceId, 'preview'];
    }
    if (v.rawStatus === 'completed' && (v.clipCount ?? 0) > 0) return ['/shorts', v.sourceId, 'clips'];
    return ['/shorts', v.sourceId, 'progress'];
  }

  menuItems(): { action: VideoCardAction; label: string; icon: string }[] {
    const v = this.video();
    const items: { action: VideoCardAction; label: string; icon: string }[] = [
      { action: 'edit', label: 'Edit', icon: 'pencil' },
      { action: 'duplicate', label: 'Duplicate', icon: 'copy' },
      { action: 'download', label: 'Download', icon: 'download' },
    ];
    if (v.status !== 'published') items.push({ action: 'publish', label: 'Publish', icon: 'upload' });
    items.push({ action: 'delete', label: 'Delete', icon: 'trash-2' });
    return items;
  }
}
