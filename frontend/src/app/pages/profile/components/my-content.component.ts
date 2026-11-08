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
  styles: [`
    :host { display: block; }
    .head { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: var(--space-xs); }
    .count { font-size: var(--type-sm); color: var(--color-muted); font-variant-numeric: tabular-nums; }
    .tabs { margin-top: var(--space-md); display: flex; gap: var(--space-2xs); overflow-x: auto; padding-bottom: var(--space-3xs); }
    .tab { flex: none; min-height: 44px; padding: 0 var(--space-sm); border-radius: var(--radius-pill); border: 1px solid var(--color-rule); background: transparent; color: var(--color-ink-2); font: 500 var(--type-sm)/1 var(--font-body); cursor: pointer; display: inline-flex; align-items: center; gap: var(--space-2xs); }
    .tab:hover { background: var(--color-paper-3); color: var(--color-ink); }
    .tab[aria-pressed='true'] { background: var(--color-ink); border-color: var(--color-ink); color: var(--color-paper); }
    .tab span { opacity: 0.75; font-variant-numeric: tabular-nums; }
    .grid { margin-top: var(--space-md); display: grid; gap: var(--space-md); }
    @media (min-width: 40rem) { .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
    .sk { height: 15rem; border-radius: var(--radius-surface); background: var(--color-paper-3); }
    .empty { margin-top: var(--space-md); padding: var(--space-xl) var(--space-md); text-align: center; border-radius: var(--radius-control); background: var(--color-paper-3); display: flex; flex-direction: column; align-items: center; gap: var(--space-2xs); }
    .empty p { margin: 0; } .empty p + p { font-size: var(--type-sm); color: var(--color-ink-2); }
  `],
  template: `
    <section class="ui-card" aria-labelledby="content-heading">
      <div class="head">
        <h2 id="content-heading" class="ui-card-title">My content</h2>
        <span class="count">{{ filteredVideos().length }} {{ filteredVideos().length === 1 ? 'item' : 'items' }}</span>
      </div>
      <div class="tabs" role="group" aria-label="Filter content">
        @for (tab of tabs; track tab.id) {
          <button type="button" class="tab" [attr.aria-pressed]="activeTab() === tab.id" (click)="tabChange.emit(tab.id)">
            {{ tab.label }} <span>{{ countFor(tab.id) }}</span>
          </button>
        }
      </div>
      @if (loading()) {
        <div class="grid" aria-hidden="true">
          @for (i of [1, 2]; track i) {
            <div class="sk"></div>
          }
        </div>
      } @else if (filteredVideos().length === 0) {
        <div class="empty">
          <span class="ui-chip" aria-hidden="true"><svg [lucideIcon]="emptyIcon()" [size]="18"></svg></span>
          <p><strong>{{ emptyTitle() }}</strong></p>
          <p>{{ emptyHint() }}</p>
        </div>
      } @else {
        <div class="grid">
          @for (video of filteredVideos(); track video.id) {
            <app-video-card [video]="video" (action)="videoAction.emit({ action: $event, video })" />
          }
        </div>
      }
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
        return all.filter((v) => v.status === 'generated');
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
        return all.filter((v) => v.status === 'generated').length;
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
        return 'Use the star on a video to save it here.';
      case 'published':
        return 'Use More actions on a video to mark it published.';
      default:
        return 'Create your first quiz video or AI short to see it here.';
    }
  }
}
