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
  styles: [`
    :host { display: block; }
    article { overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-surface); background: var(--app-card); height: 100%; display: flex; flex-direction: column; }
    .thumb { position: relative; aspect-ratio: 16 / 9; background: var(--color-paper-3); }
    .thumb img { width: 100%; height: 100%; object-fit: cover; display: block; }
    .ph { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; color: var(--color-accent-text); background: color-mix(in oklab, var(--color-accent) 10%, var(--color-paper-3)); }
    .open { position: absolute; inset: 0; border: 0; background: transparent; cursor: pointer; }
    .open:focus-visible { outline: 2px solid var(--color-focus); outline-offset: -3px; }
    .status { position: absolute; top: var(--space-xs); left: var(--space-xs); background: var(--app-card); pointer-events: none; }
    .fav { position: absolute; top: 0; right: 0; width: 44px; height: 44px; border: 0; background: transparent; cursor: pointer; display: inline-flex; align-items: center; justify-content: center; }
    .fav span { width: 2rem; height: 2rem; border-radius: 50%; background: var(--app-card); display: inline-flex; align-items: center; justify-content: center; color: var(--color-ink-2); border: 1px solid var(--color-rule); }
    .fav[aria-pressed='true'] span { color: var(--color-accent-text); }
    .ratio { position: absolute; bottom: var(--space-xs); right: var(--space-xs); padding: 0 var(--space-2xs); border-radius: 4px; background: var(--app-card); font-size: var(--type-xs); color: var(--color-ink-2); }
    .body { padding: var(--space-md); display: flex; flex-direction: column; gap: var(--space-2xs); flex: 1; }
    h3 { margin: 0; font-size: var(--type-md); font-weight: 600; line-height: 1.3; overflow-wrap: anywhere; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
    .meta { display: flex; flex-wrap: wrap; gap: 0 var(--space-sm); font-size: var(--type-xs); color: var(--color-muted); }
    .row { margin-top: auto; padding-top: var(--space-sm); display: flex; align-items: center; gap: var(--space-xs); }
    .row > .ui-btn:first-child { flex: 1; }
    .more { position: relative; }
    .more > .ui-btn { width: 44px; padding: 0; }
    .menu { position: absolute; right: 0; bottom: calc(100% + var(--space-2xs)); z-index: var(--z-dropdown); width: 12rem; padding: var(--space-2xs) 0; border-radius: var(--radius-control); border: 1px solid var(--color-rule); background: var(--app-card); box-shadow: 0 12px 32px -12px color-mix(in oklab, var(--color-ink) 35%, transparent); }
    .menu button { width: 100%; min-height: 44px; display: flex; align-items: center; gap: var(--space-xs); padding: 0 var(--space-md); border: 0; background: transparent; color: var(--color-ink); font: inherit; font-size: var(--type-sm); text-align: left; cursor: pointer; }
    .menu button:hover { background: var(--color-paper-3); }
    .menu button.del { color: var(--color-negative); }
    .s-published, .s-generated { color: var(--color-positive); } .s-processing { color: var(--color-caution); }
    .s-failed { color: var(--color-negative); } .s-draft { color: var(--color-ink-2); }
  `],
  template: `
    <article [attr.aria-label]="video().title">
      <div class="thumb">
        @if (video().thumbnailUrl) {
          <img [src]="video().thumbnailUrl" alt="" loading="lazy" />
        } @else {
          <div class="ph" aria-hidden="true"><svg [lucideIcon]="video().kind === 'short' ? 'scissors' : 'clapperboard'" [size]="32"></svg></div>
        }
        <button type="button" class="open" (click)="action.emit('play')" [attr.aria-label]="'Open ' + video().title"></button>
        <span class="ui-pill status" [class]="'ui-pill status s-' + video().status">{{ statusLabel() }}</span>
        <button type="button" class="fav" (click)="action.emit('favorite')" [attr.aria-label]="video().favorite ? 'Remove from favourites' : 'Add to favourites'" [attr.aria-pressed]="video().favorite">
          <span><svg lucideIcon="star" [size]="15" [attr.fill]="video().favorite ? 'currentColor' : 'none'" aria-hidden="true"></svg></span>
        </button>
        @if (video().kind === 'short') {
          <span class="ratio" aria-hidden="true">9:16</span>
        }
      </div>
      <div class="body">
        <h3>{{ video().title }}</h3>
        <p class="meta" style="margin: 0">
          <span>{{ video().kind === 'short' ? 'AI Shorts' : 'Quiz video' }}</span>
          <span>{{ video().duration }}</span>
          <span>{{ video().aspectRatio }}</span>
          @if (video().clipCount) {
            <span>{{ video().clipCount }} {{ video().clipCount === 1 ? 'clip' : 'clips' }}</span>
          }
        </p>
        <div class="row">
          @if (playLink(); as link) {
            <a [routerLink]="link" class="ui-btn ui-btn--secondary">
              <svg lucideIcon="play" [size]="14" aria-hidden="true"></svg>
              {{ video().status === 'draft' || video().status === 'processing' ? 'Track' : 'Open' }}
            </a>
          } @else {
            <button type="button" class="ui-btn ui-btn--secondary" (click)="action.emit('play')">
              <svg lucideIcon="play" [size]="14" aria-hidden="true"></svg>
              Open
            </button>
          }
          <div class="more">
            <button type="button" class="ui-btn ui-btn--quiet" (click)="menuOpen = !menuOpen" aria-label="More actions" aria-haspopup="menu" [attr.aria-expanded]="menuOpen">
              <svg lucideIcon="more-vertical" [size]="16" aria-hidden="true"></svg>
            </button>
            @if (menuOpen) {
              <div class="menu" role="menu" (keydown.escape)="menuOpen = false">
                @for (item of menuItems(); track item.action) {
                  <button type="button" role="menuitem" [class.del]="item.action === 'delete'" (click)="menuOpen = false; action.emit(item.action)">
                    <svg [lucideIcon]="item.icon" [size]="15" aria-hidden="true"></svg>
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
        return 'Published';
      case 'generated':
        return 'Generated';
      case 'draft':
        return 'Draft';
      case 'processing':
        return 'In progress';
      case 'failed':
        return 'Failed';
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
    if (v.status !== 'published') items.push({ action: 'publish', label: 'Mark published', icon: 'upload' });
    items.push({ action: 'delete', label: 'Delete', icon: 'trash-2' });
    return items;
  }
}
