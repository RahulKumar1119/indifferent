import { Component, input, output } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { CONTENT_TABS, ContentTab, CreatorVideo } from '../models/creator-stats.model';
import { VideoCardComponent, VideoCardAction } from './video-card.component';

export interface VideoActionEvent {
  action: VideoCardAction;
  video: CreatorVideo;
}

/** Tabbed content library with loading, empty and per-tab filtering. */
@Component({
  selector: 'app-my-content',
  standalone: true,
  imports: [LucideDynamicIcon, VideoCardComponent],
  template: `
    <section class="glass-card p-5 md:p-7" aria-label="My content">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <h2 class="text-lg font-semibold flex items-center gap-2">
          <svg lucideIcon="clapperboard" [size]="19" class="text-[hsl(var(--primary))]" aria-hidden="true"></svg>
          My Content
        </h2>
        <span class="text-[12.5px] text-[hsl(var(--muted-foreground))]">
          {{ filteredVideos().length }} {{ filteredVideos().length === 1 ? 'item' : 'items' }}
        </span>
      </div>

      <!-- Tabs: horizontal scroll on mobile -->
      <div
        class="mt-4 flex gap-2 overflow-x-auto pb-1 -mx-1 px-1"
        role="tablist"
        aria-label="Content filters"
      >
        @for (tab of tabs; track tab.id) {
          <button
            type="button"
            role="tab"
            [attr.aria-selected]="activeTab() === tab.id"
            (click)="tabChange.emit(tab.id)"
            class="shrink-0 rounded-full px-4 py-2 text-[13px] font-medium transition-colors"
            [class]="activeTab() === tab.id
              ? 'bg-[hsl(var(--foreground))] text-[hsl(var(--background))]'
              : 'border border-[hsl(var(--border))] text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))] hover:border-[hsl(var(--foreground))]/30'"
          >
            {{ tab.label }}
            <span class="ml-1 opacity-70">{{ countFor(tab.id) }}</span>
          </button>
        }
      </div>

      <div class="mt-5">
        @if (loading()) {
          <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
            @for (i of [1, 2, 3]; track i) {
              <div class="skeleton h-[240px] rounded-2xl"></div>
            }
          </div>
        } @else if (filteredVideos().length === 0) {
          <div class="rounded-2xl border border-dashed border-[hsl(var(--border))] px-6 py-12 text-center">
            <span
              class="mx-auto w-12 h-12 rounded-full bg-[hsl(var(--primary))]/10 flex items-center justify-center"
              aria-hidden="true"
            >
              <svg lucideIcon="{{ emptyIcon() }}" [size]="22" class="text-[hsl(var(--primary))]"></svg>
            </span>
            <p class="mt-4 font-medium text-[15px]">{{ emptyTitle() }}</p>
            <p class="mt-1 text-[13.5px] text-[hsl(var(--muted-foreground))]">{{ emptyHint() }}</p>
          </div>
        } @else {
          <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
            @for (video of filteredVideos(); track video.id) {
              <app-video-card [video]="video" (action)="videoAction.emit({ action: $event, video })" />
            }
          </div>
        }
      </div>
    </section>
  `,
})
export class MyContentComponent {
  readonly videos = input.required<CreatorVideo[]>();
  readonly activeTab = input<ContentTab>('all');
  readonly loading = input(false);
  readonly tabChange = output<ContentTab>();
  readonly videoAction = output<VideoActionEvent>();

  readonly tabs = CONTENT_TABS;

  filteredVideos(): CreatorVideo[] {
    const tab = this.activeTab();
    const all = this.videos();
    switch (tab) {
      case 'generated':
        return all.filter((v) => v.status === 'generated' || v.status === 'published');
      case 'published':
        return all.filter((v) => v.status === 'published');
      case 'drafts':
        return all.filter((v) => v.status === 'draft' || v.status === 'processing' || v.status === 'failed');
      case 'favorites':
        return all.filter((v) => v.favorite);
      default:
        return all;
    }
  }

  countFor(tab: ContentTab): number {
    const all = this.videos();
    switch (tab) {
      case 'generated':
        return all.filter((v) => v.status === 'generated' || v.status === 'published').length;
      case 'published':
        return all.filter((v) => v.status === 'published').length;
      case 'drafts':
        return all.filter((v) => v.status === 'draft' || v.status === 'processing' || v.status === 'failed').length;
      case 'favorites':
        return all.filter((v) => v.favorite).length;
      default:
        return all.length;
    }
  }

  emptyIcon(): string {
    switch (this.activeTab()) {
      case 'published':
        return 'upload';
      case 'drafts':
        return 'file-text';
      case 'favorites':
        return 'star';
      default:
        return 'clapperboard';
    }
  }

  emptyTitle(): string {
    switch (this.activeTab()) {
      case 'generated':
        return 'No generated videos yet';
      case 'published':
        return 'Nothing published yet';
      case 'drafts':
        return 'No drafts in progress';
      case 'favorites':
        return 'No favorites yet';
      default:
        return 'No content yet';
    }
  }

  emptyHint(): string {
    switch (this.activeTab()) {
      case 'favorites':
        return 'Tap the star on any video card to save it here.';
      case 'published':
        return 'Use the ⋮ menu on a video to publish it.';
      default:
        return 'Create your first quiz video or AI short to see it here.';
    }
  }
}
