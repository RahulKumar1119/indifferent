import { Component, ElementRef, OnChanges, ViewChild, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { LucideDynamicIcon } from '@lucide/angular';
import { CreatorProfile } from '../models/creator-profile.model';

/**
 * Profile details: avatar (upload / crop / remove) and a labelled form for
 * name, username and bio with one Save action. Emits the updated profile upward so the
 * parent stays the single source of truth for persistence.
 */
@Component({
  selector: 'app-profile-header',
  standalone: true,
  imports: [FormsModule, LucideDynamicIcon],
  styles: [`
    :host { display: block; }
    .wrap { display: grid; gap: var(--space-lg); }
    @media (min-width: 56rem) { .wrap { grid-template-columns: 16rem minmax(0, 1fr); gap: var(--space-xl); } }
    .who { display: flex; flex-direction: column; align-items: flex-start; gap: var(--space-sm); }
    .avatar { position: relative; width: 6rem; height: 6rem; border-radius: 50%; overflow: hidden; border: 2px solid color-mix(in oklab, var(--color-accent) 35%, transparent); background: color-mix(in oklab, var(--color-accent) 12%, var(--app-card)); padding: 0; cursor: pointer; }
    .avatar img { width: 100%; height: 100%; object-fit: cover; display: block; }
    .initials { display: flex; width: 100%; height: 100%; align-items: center; justify-content: center; font-size: var(--type-xl); font-weight: 600; color: var(--color-accent-text); }
    .name { margin: 0; font-size: var(--type-xl); font-weight: 600; line-height: 1.2; overflow-wrap: anywhere; }
    .handle, .mail { margin: 0; font-size: var(--type-sm); color: var(--color-muted); overflow-wrap: anywhere; }
    .bio { margin: var(--space-2xs) 0 0; color: var(--color-ink-2); line-height: 1.5; }
    .badge { display: inline-flex; align-items: center; gap: var(--space-3xs); color: var(--color-accent-text); }
    form { display: flex; flex-direction: column; gap: var(--space-md); }
    .field { display: flex; flex-direction: column; gap: var(--space-2xs); }
    .prefix { position: relative; }
    .prefix span { position: absolute; left: var(--space-md); top: 50%; transform: translateY(-50%); color: var(--color-muted); pointer-events: none; }
    .prefix .ui-field { padding-left: calc(var(--space-md) + 1rem); }
    textarea.ui-field { min-height: 6rem; padding-top: var(--space-sm); resize: vertical; }
    .count { align-self: flex-end; font-size: var(--type-xs); color: var(--color-muted); font-variant-numeric: tabular-nums; }
    .err { margin: 0; font-size: var(--type-sm); color: var(--color-negative); font-weight: 500; }
    .bar { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-sm); padding-top: var(--space-md); border-top: 1px solid var(--color-rule); }
    .bar .btns { display: flex; gap: var(--space-xs); }
    .local { margin: 0; display: flex; gap: var(--space-xs); align-items: flex-start; font-size: var(--type-xs); color: var(--color-muted); max-width: 40ch; }
    .sk { border-radius: var(--radius-control); background: var(--color-paper-3); }
    .dialog { position: fixed; inset: 0; z-index: var(--z-modal); display: flex; align-items: flex-end; justify-content: center; padding: var(--space-md); }
    @media (min-width: 40rem) { .dialog { align-items: center; } }
    .scrim { position: absolute; inset: 0; background: color-mix(in oklab, var(--color-ink) 55%, transparent); }
    .panel { position: relative; width: 100%; max-width: 28rem; }
    .panel-head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-sm); }
    .icon-btn { width: 44px; height: 44px; display: inline-flex; align-items: center; justify-content: center; border: 0; border-radius: var(--radius-control); background: transparent; color: var(--color-ink-2); cursor: pointer; }
    .icon-btn:hover { background: var(--color-paper-3); }
    .upload { margin-top: var(--space-md); width: 100%; display: flex; flex-direction: column; align-items: center; gap: var(--space-xs); padding: var(--space-xl) var(--space-md); border: 2px dashed var(--color-rule); border-radius: var(--radius-surface); background: transparent; color: var(--color-ink-2); cursor: pointer; font: inherit; }
    .upload:hover { border-color: var(--color-accent); color: var(--color-ink); }
    .crop-frame { position: relative; width: 240px; height: 240px; overflow: hidden; border-radius: 50%; border: 2px solid color-mix(in oklab, var(--color-accent) 45%, transparent); touch-action: none; cursor: grab; user-select: none; }
    .crop-frame:active { cursor: grabbing; }
    .crop { margin-top: var(--space-md); display: flex; flex-direction: column; align-items: center; gap: var(--space-sm); }
    .zoom { display: flex; align-items: center; gap: var(--space-sm); width: 100%; font-size: var(--type-sm); }
    .zoom input { flex: 1; accent-color: var(--color-accent); min-height: 44px; }
    .foot { margin-top: var(--space-md); display: flex; justify-content: space-between; gap: var(--space-xs); }
  `],
  template: `
    <section class="ui-card" aria-labelledby="profile-details">
      @if (loading()) {
        <div class="wrap" aria-hidden="true">
          <div class="who"><div class="sk" style="width: 6rem; height: 6rem; border-radius: 50%"></div><div class="sk" style="width: 10rem; height: 1.25rem"></div></div>
          <div style="display: grid; gap: var(--space-md)"><div class="sk" style="height: 44px"></div><div class="sk" style="height: 44px"></div><div class="sk" style="height: 6rem"></div></div>
        </div>
      } @else {
        <div class="wrap">
          <div class="who">
            <button type="button" class="avatar" (click)="openAvatarDialog()" aria-label="Change profile picture">
              @if (profile().avatarUrl) {
                <img [src]="profile().avatarUrl" alt="" />
              } @else {
                <span class="initials" aria-hidden="true">{{ avatarInitials() }}</span>
              }
            </button>
            <button type="button" class="ui-btn ui-btn--quiet" style="padding-inline: var(--space-2xs); margin-left: calc(-1 * var(--space-2xs))" (click)="openAvatarDialog()">
              <svg lucideIcon="image" [size]="16" aria-hidden="true"></svg>
              Change picture
            </button>
            <div>
              <p class="name">{{ profile().name || 'Your creator name' }}</p>
              <p class="handle">&#64;{{ profile().username || 'username' }}</p>
              @if (profile().email) {
                <p class="mail">{{ profile().email }}</p>
              }
              <p class="handle badge"><svg lucideIcon="sparkles" [size]="14" aria-hidden="true"></svg>{{ profile().badge || 'AI Creator' }}</p>
              <p class="bio">{{ profile().bio || 'Tell the world what you create.' }}</p>
            </div>
          </div>

          <form (ngSubmit)="save()" novalidate data-testid="profile-form">
            <h2 id="profile-details" class="ui-card-title">Profile details</h2>
            <div class="field">
              <label class="ui-label" for="pf-name">Creator name</label>
              <input id="pf-name" name="name" class="ui-field" [(ngModel)]="draftName" maxlength="60" autocomplete="name" />
            </div>
            <div class="field">
              <label class="ui-label" for="pf-username">Username</label>
              <div class="prefix">
                <span aria-hidden="true">&#64;</span>
                <input id="pf-username" name="username" class="ui-field" [(ngModel)]="draftUsername" maxlength="24" autocomplete="username"
                  aria-describedby="pf-username-hint" [attr.aria-invalid]="usernameError ? 'true' : null" />
              </div>
              <span id="pf-username-hint" class="ui-hint">2 to 24 letters, numbers or underscores.</span>
              @if (usernameError) {
                <p class="err" role="alert">{{ usernameError }}</p>
              }
            </div>
            <div class="field">
              <label class="ui-label" for="pf-bio">Bio</label>
              <textarea id="pf-bio" name="bio" class="ui-field" [(ngModel)]="draftBio" maxlength="160" rows="3"></textarea>
              <span class="count" aria-live="polite">{{ draftBio.length }} of 160</span>
            </div>
            <div class="bar">
              <p class="local">
                <svg lucideIcon="lock" [size]="14" aria-hidden="true"></svg>
                Name, username, bio and picture are saved in this browser.
              </p>
              <div class="btns">
                <button type="button" class="ui-btn ui-btn--secondary" (click)="discard()" [disabled]="!dirty()">Discard</button>
                <button type="submit" class="ui-btn ui-btn--primary" [disabled]="!dirty()">Save changes</button>
              </div>
            </div>
          </form>
        </div>
      }
    </section>

    @if (avatarOpen) {
      <div class="dialog hm" role="dialog" aria-modal="true" aria-labelledby="avatar-title" (keydown.escape)="closeAvatarDialog()">
        <div class="scrim" (click)="closeAvatarDialog()" aria-hidden="true"></div>
        <div class="panel ui-card">
          <div class="panel-head">
            <h2 id="avatar-title" class="ui-card-title">Profile picture</h2>
            <button type="button" class="icon-btn" (click)="closeAvatarDialog()" aria-label="Close">
              <svg lucideIcon="x" [size]="18" aria-hidden="true"></svg>
            </button>
          </div>
          <input #fileInput type="file" accept="image/png,image/jpeg,image/webp" hidden (change)="onFileSelected($event)" aria-label="Upload a profile picture" />
          @if (!cropImageSrc) {
            <button type="button" class="upload" (click)="fileInput.click()">
              <svg lucideIcon="upload" [size]="24" aria-hidden="true"></svg>
              Upload a photo
              <span class="ui-hint">PNG, JPG or WebP, cropped to a circle</span>
            </button>
            @if (profile().avatarUrl) {
              <button type="button" class="ui-btn ui-btn--danger" style="margin-top: var(--space-sm)" (click)="removeAvatar()">
                <svg lucideIcon="trash-2" [size]="16" aria-hidden="true"></svg>
                Remove current picture
              </button>
            }
          } @else {
            <div class="crop">
              <div #cropFrame class="crop-frame" (pointerdown)="onCropPointerDown($event)" (pointermove)="onCropPointerMove($event)" (pointerup)="onCropPointerEnd()" (pointerleave)="onCropPointerEnd()">
                <img #cropImg [src]="cropImageSrc" alt="Crop preview" style="position: absolute; max-width: none"
                  [style.width.px]="cropDisplayWidth()" [style.height.px]="cropDisplayHeight()" [style.left.px]="cropOffsetX" [style.top.px]="cropOffsetY" draggable="false" />
              </div>
              <label class="zoom">
                <span>Zoom</span>
                <input type="range" min="1" max="3" step="0.05" [(ngModel)]="cropZoom" (ngModelChange)="onZoomChange()" />
              </label>
              <p class="ui-hint" style="margin: 0">Drag the photo to position it. Use the slider to zoom.</p>
            </div>
            <div class="foot">
              <button type="button" class="ui-btn ui-btn--secondary" (click)="resetCrop()">Choose another</button>
              <button type="button" class="ui-btn ui-btn--primary" (click)="saveCroppedAvatar()">Save picture</button>
            </div>
          }
        </div>
      </div>
    }
  `,
})
export class ProfileHeaderComponent implements OnChanges {
  readonly profile = input.required<CreatorProfile>();
  readonly loading = input(false);
  readonly profileChange = output<Partial<CreatorProfile>>();

  @ViewChild('cropFrame') cropFrame?: ElementRef<HTMLDivElement>;
  @ViewChild('cropImg') cropImg?: ElementRef<HTMLImageElement>;

  draftName = '';
  draftUsername = '';
  draftBio = '';
  usernameError = '';

  avatarOpen = false;
  cropImageSrc = '';
  cropNatW = 0;
  cropNatH = 0;
  cropZoom = 1;
  private lastZoom = 1;
  cropOffsetX = 0;
  cropOffsetY = 0;
  private dragging = false;
  private dragStartX = 0;
  private dragStartY = 0;
  private dragBaseX = 0;
  private dragBaseY = 0;

  avatarInitials(): string {
    const name = this.profile().name.trim();
    if (!name) return 'C';
    const parts = name.split(/\s+/);
    return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase() || 'C';
  }

  // ---------------------------------------------------------- edit profile

  /** Fill the form from the saved profile whenever it changes. */
  ngOnChanges(): void {
    this.discard();
  }

  dirty(): boolean {
    const p = this.profile();
    return this.draftName !== p.name || this.draftUsername !== p.username || this.draftBio !== p.bio;
  }

  discard(): void {
    const p = this.profile();
    this.draftName = p.name;
    this.draftUsername = p.username;
    this.draftBio = p.bio;
    this.usernameError = '';
  }

  save(): void {
    const username = this.draftUsername.trim().replace(/^@/, '');
    if (username && !/^[a-zA-Z0-9_]{2,24}$/.test(username)) {
      this.usernameError = 'Use 2 to 24 letters, numbers or underscores.';
      return;
    }
    this.usernameError = '';
    this.profileChange.emit({
      name: this.draftName.trim().slice(0, 60),
      username: username.toLowerCase(),
      bio: this.draftBio.trim().slice(0, 160),
    });
  }

  // ---------------------------------------------------------------- avatar

  openAvatarDialog(): void {
    this.resetCrop();
    this.avatarOpen = true;
  }

  closeAvatarDialog(): void {
    this.avatarOpen = false;
    this.resetCrop();
  }

  removeAvatar(): void {
    this.profileChange.emit({ avatarUrl: '' });
    this.closeAvatarDialog();
  }

  onFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file || !file.type.startsWith('image/')) return;
    const reader = new FileReader();
    reader.onload = () => {
      const img = new Image();
      img.onload = () => {
        this.cropNatW = img.naturalWidth;
        this.cropNatH = img.naturalHeight;
        this.cropImageSrc = String(reader.result ?? '');
        this.cropZoom = 1;
        this.lastZoom = 1;
        this.centerCrop();
      };
      img.src = String(reader.result ?? '');
    };
    reader.readAsDataURL(file);
  }

  resetCrop(): void {
    this.cropImageSrc = '';
    this.cropNatW = 0;
    this.cropNatH = 0;
    this.cropZoom = 1;
    this.lastZoom = 1;
    this.cropOffsetX = 0;
    this.cropOffsetY = 0;
  }

  /** Cover-fit display size: the image always covers the 240px frame, keeping aspect ratio. */
  cropDisplayWidth(): number {
    if (!this.cropNatW || !this.cropNatH) return 240;
    return (this.cropNatW * Math.max(240 / this.cropNatW, 240 / this.cropNatH) * this.cropZoom);
  }

  cropDisplayHeight(): number {
    if (!this.cropNatW || !this.cropNatH) return 240;
    return (this.cropNatH * Math.max(240 / this.cropNatW, 240 / this.cropNatH) * this.cropZoom);
  }

  private centerCrop(): void {
    this.cropOffsetX = (240 - this.cropDisplayWidth()) / 2;
    this.cropOffsetY = (240 - this.cropDisplayHeight()) / 2;
  }

  /** Scale pan offsets around the frame center so zoom feels anchored. */
  onZoomChange(): void {
    const factor = this.lastZoom > 0 ? this.cropZoom / this.lastZoom : 1;
    this.lastZoom = this.cropZoom;
    const cx = 120;
    const cy = 120;
    this.cropOffsetX = cx + (this.cropOffsetX - cx) * factor;
    this.cropOffsetY = cy + (this.cropOffsetY - cy) * factor;
    this.clampCrop();
  }

  clampCrop(): void {
    this.cropOffsetX = Math.min(0, Math.max(240 - this.cropDisplayWidth(), this.cropOffsetX));
    this.cropOffsetY = Math.min(0, Math.max(240 - this.cropDisplayHeight(), this.cropOffsetY));
  }

  onCropPointerDown(event: PointerEvent): void {
    this.dragging = true;
    this.dragStartX = event.clientX;
    this.dragStartY = event.clientY;
    this.dragBaseX = this.cropOffsetX;
    this.dragBaseY = this.cropOffsetY;
    (event.target as HTMLElement).setPointerCapture?.(event.pointerId);
  }

  onCropPointerMove(event: PointerEvent): void {
    if (!this.dragging) return;
    this.cropOffsetX = this.dragBaseX + (event.clientX - this.dragStartX);
    this.cropOffsetY = this.dragBaseY + (event.clientY - this.dragStartY);
    this.clampCrop();
  }

  onCropPointerEnd(): void {
    this.dragging = false;
  }

  saveCroppedAvatar(): void {
    const img = this.cropImg?.nativeElement;
    if (!img || !this.cropImageSrc) return;
    const OUT = 256;
    const canvas = document.createElement('canvas');
    canvas.width = OUT;
    canvas.height = OUT;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    const displayedW = this.cropDisplayWidth();
    // Uniform scale (aspect preserved), so one factor maps both axes.
    const frameScale = img.naturalWidth / displayedW;
    const sourceX = Math.max(0, -this.cropOffsetX * frameScale);
    const sourceY = Math.max(0, -this.cropOffsetY * frameScale);
    const sourceSize = Math.min(
      img.naturalWidth - sourceX,
      img.naturalHeight - sourceY,
      240 * frameScale,
    );
    ctx.save();
    ctx.beginPath();
    ctx.arc(OUT / 2, OUT / 2, OUT / 2, 0, Math.PI * 2);
    ctx.clip();
    ctx.drawImage(img, sourceX, sourceY, sourceSize, sourceSize, 0, 0, OUT, OUT);
    ctx.restore();
    const dataUrl = canvas.toDataURL('image/png');
    this.profileChange.emit({ avatarUrl: dataUrl });
    this.closeAvatarDialog();
  }
}
