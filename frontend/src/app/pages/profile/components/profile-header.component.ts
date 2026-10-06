import { Component, ElementRef, ViewChild, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { LucideDynamicIcon } from '@lucide/angular';
import { CreatorProfile } from '../models/creator-profile.model';

/**
 * Profile header: avatar (upload / crop / remove), name, username, badge,
 * bio and the Edit Profile action. Emits the updated profile upward so the
 * parent stays the single source of truth for persistence.
 */
@Component({
  selector: 'app-profile-header',
  standalone: true,
  imports: [FormsModule, LucideDynamicIcon],
  styles: [
    `
      .crop-frame {
        touch-action: none;
        cursor: grab;
      }
      .crop-frame:active {
        cursor: grabbing;
      }
    `,
  ],
  template: `
    <section
      class="glass-card spotlight-card relative overflow-hidden p-6 md:p-8"
      aria-label="Creator profile header"
    >
      <div
        class="pointer-events-none absolute -top-24 -right-24 h-64 w-64 rounded-full opacity-20 blur-3xl"
        style="background: radial-gradient(circle, hsl(var(--primary)), transparent 70%)"
        aria-hidden="true"
      ></div>

      @if (loading()) {
        <div class="flex items-center gap-6">
          <div class="skeleton h-20 w-20 md:h-24 md:w-24 rounded-full shrink-0"></div>
          <div class="flex-1 space-y-3">
            <div class="skeleton h-6 w-48"></div>
            <div class="skeleton h-4 w-32"></div>
            <div class="skeleton h-4 w-64"></div>
          </div>
        </div>
      } @else {
        <div class="flex flex-col sm:flex-row sm:items-center gap-6">
          <!-- Avatar -->
          <div class="relative shrink-0 self-start">
            <button
              type="button"
              (click)="openAvatarDialog()"
              class="group relative block h-20 w-20 md:h-24 md:w-24 rounded-full overflow-hidden border-2 border-[hsl(var(--primary))]/30 hover:border-[hsl(var(--primary))]/60 transition-colors"
              aria-label="Change profile avatar"
              title="Change avatar"
            >
              @if (profile().avatarUrl) {
                <img
                  [src]="profile().avatarUrl"
                  [alt]="profile().name + ' avatar'"
                  class="h-full w-full object-cover"
                />
              } @else {
                <span
                  class="flex h-full w-full items-center justify-center text-2xl md:text-3xl font-bold bg-[hsl(var(--primary))]/10 text-[hsl(var(--primary))]"
                  aria-hidden="true"
                >
                  {{ avatarInitials() }}
                </span>
              }
              <span
                class="absolute inset-0 flex items-center justify-center bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity"
                aria-hidden="true"
              >
                <svg lucideIcon="image" [size]="20" class="text-white"></svg>
              </span>
            </button>
            <span
              class="absolute -bottom-1 -right-1 w-7 h-7 rounded-full bg-[hsl(var(--primary))] text-white flex items-center justify-center border-2 border-[hsl(var(--background))]"
              aria-hidden="true"
            >
              <svg lucideIcon="pencil" [size]="12"></svg>
            </span>
          </div>

          <!-- Identity -->
          <div class="flex-1 min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h1 class="text-xl md:text-2xl font-bold tracking-tight truncate">
                {{ profile().name || 'Your creator name' }}
              </h1>
              <span
                class="inline-flex items-center gap-1 rounded-full bg-[hsl(var(--primary))]/10 text-[hsl(var(--primary))] px-2.5 py-1 text-[11.5px] font-semibold"
              >
                <svg lucideIcon="sparkles" [size]="12" aria-hidden="true"></svg>
                {{ profile().badge || 'AI Creator' }}
              </span>
            </div>
            <p class="mt-0.5 text-sm text-[hsl(var(--muted-foreground))]">
              &#64;{{ profile().username || 'username' }}
              @if (profile().email) {
                <span aria-hidden="true"> · </span>{{ profile().email }}
              }
            </p>
            <p class="mt-2 text-[14.5px] leading-relaxed max-w-[52ch]">
              {{ profile().bio || 'Tell the world what you create.' }}
            </p>
          </div>

          <button
            type="button"
            (click)="openEditDialog()"
            class="btn-interactive shrink-0 inline-flex items-center gap-2 rounded-full bg-[hsl(var(--foreground))] text-[hsl(var(--background))] px-5 py-2.5 text-[13.5px] font-semibold"
          >
            <svg lucideIcon="pencil" [size]="15" aria-hidden="true"></svg>
            Edit Profile
          </button>
        </div>
      }
    </section>

    <!-- Edit profile dialog -->
    @if (editOpen) {
      <div
        class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4"
        role="dialog"
        aria-modal="true"
        aria-label="Edit profile"
      >
        <div class="absolute inset-0 bg-black/60" (click)="closeEditDialog()" aria-hidden="true"></div>
        <div
          class="relative w-full max-w-md rounded-2xl border border-[hsl(var(--border))] bg-[hsl(var(--card))] p-6 shadow-2xl"
        >
          <div class="flex items-center justify-between">
            <h2 class="text-lg font-semibold">Edit Profile</h2>
            <button
              type="button"
              (click)="closeEditDialog()"
              class="w-9 h-9 rounded-lg hover:bg-white/5 inline-flex items-center justify-center"
              aria-label="Close edit profile"
            >
              <svg lucideIcon="x" [size]="18"></svg>
            </button>
          </div>
          <div class="mt-4 space-y-4">
            <label class="block">
              <span class="text-[13px] font-medium">Creator name</span>
              <input
                [(ngModel)]="draftName"
                maxlength="60"
                class="mt-1.5 w-full rounded-xl border border-[hsl(var(--border))] bg-[hsl(var(--secondary))] px-3.5 py-2.5 text-[14px] outline-none focus:border-[hsl(var(--primary))]"
                placeholder="Rahul Kumar"
              />
            </label>
            <label class="block">
              <span class="text-[13px] font-medium">Username</span>
              <div class="mt-1.5 flex items-center rounded-xl border border-[hsl(var(--border))] bg-[hsl(var(--secondary))] focus-within:border-[hsl(var(--primary))]">
                <span class="pl-3.5 text-[14px] text-[hsl(var(--muted-foreground))]">&#64;</span>
                <input
                  [(ngModel)]="draftUsername"
                  maxlength="24"
                  pattern="[a-zA-Z0-9_]+"
                  class="w-full bg-transparent px-1.5 py-2.5 text-[14px] outline-none"
                  placeholder="rahulkumar"
                />
              </div>
              @if (usernameError) {
                <span class="mt-1 block text-[12px] text-red-400">{{ usernameError }}</span>
              }
            </label>
            <label class="block">
              <span class="text-[13px] font-medium">Bio</span>
              <textarea
                [(ngModel)]="draftBio"
                maxlength="160"
                rows="3"
                class="mt-1.5 w-full rounded-xl border border-[hsl(var(--border))] bg-[hsl(var(--secondary))] px-3.5 py-2.5 text-[14px] outline-none focus:border-[hsl(var(--primary))] resize-none"
                placeholder="Creating AI-powered Shorts with Indifferent"
              ></textarea>
              <span class="mt-1 block text-right text-[11.5px] text-[hsl(var(--muted-foreground))]">
                {{ draftBio.length }}/160
              </span>
            </label>
          </div>
          <div class="mt-5 flex justify-end gap-2">
            <button
              type="button"
              (click)="closeEditDialog()"
              class="rounded-full px-5 py-2.5 text-[13.5px] font-medium border border-[hsl(var(--border))] hover:bg-white/5"
            >
              Cancel
            </button>
            <button
              type="button"
              (click)="saveEditDialog()"
              class="btn-interactive rounded-full px-5 py-2.5 text-[13.5px] font-semibold bg-[hsl(var(--primary))] text-white"
            >
              Save changes
            </button>
          </div>
        </div>
      </div>
    }

    <!-- Avatar dialog: upload, crop (zoom + drag), preview, remove -->
    @if (avatarOpen) {
      <div
        class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4"
        role="dialog"
        aria-modal="true"
        aria-label="Change avatar"
      >
        <div class="absolute inset-0 bg-black/60" (click)="closeAvatarDialog()" aria-hidden="true"></div>
        <div
          class="relative w-full max-w-md rounded-2xl border border-[hsl(var(--border))] bg-[hsl(var(--card))] p-6 shadow-2xl"
        >
          <div class="flex items-center justify-between">
            <h2 class="text-lg font-semibold">Profile Avatar</h2>
            <button
              type="button"
              (click)="closeAvatarDialog()"
              class="w-9 h-9 rounded-lg hover:bg-white/5 inline-flex items-center justify-center"
              aria-label="Close avatar editor"
            >
              <svg lucideIcon="x" [size]="18"></svg>
            </button>
          </div>

          <input
            #fileInput
            type="file"
            accept="image/png,image/jpeg,image/webp"
            class="hidden"
            (change)="onFileSelected($event)"
            aria-label="Upload avatar image"
          />

          @if (!cropImageSrc) {
            <button
              type="button"
              (click)="fileInput.click()"
              class="mt-4 flex w-full flex-col items-center gap-2 rounded-2xl border border-dashed border-[hsl(var(--border))] px-4 py-10 text-[14px] text-[hsl(var(--muted-foreground))] hover:border-[hsl(var(--primary))] hover:text-[hsl(var(--foreground))] transition-colors"
            >
              <svg lucideIcon="upload" [size]="24" aria-hidden="true"></svg>
              Upload a photo
              <span class="text-[12px]">PNG, JPG or WebP · cropped to a circle</span>
            </button>
            @if (profile().avatarUrl) {
              <button
                type="button"
                (click)="removeAvatar()"
                class="mt-3 inline-flex items-center gap-2 text-[13.5px] text-red-400 hover:text-red-300"
              >
                <svg lucideIcon="trash-2" [size]="15" aria-hidden="true"></svg>
                Remove current avatar
              </button>
            }
          } @else {
            <div class="mt-4 flex flex-col items-center">
              <div
                #cropFrame
                class="crop-frame relative h-60 w-60 overflow-hidden rounded-full border-2 border-[hsl(var(--primary))]/40 select-none"
                (pointerdown)="onCropPointerDown($event)"
                (pointermove)="onCropPointerMove($event)"
                (pointerup)="onCropPointerEnd()"
                (pointerleave)="onCropPointerEnd()"
                aria-label="Drag to position the crop"
              >
                <img
                  #cropImg
                  [src]="cropImageSrc"
                  alt="Avatar crop preview"
                  class="absolute max-w-none"
                  [style.width.px]="cropDisplayWidth()"
                  [style.height.px]="cropDisplayHeight()"
                  [style.left.px]="cropOffsetX"
                  [style.top.px]="cropOffsetY"
                  draggable="false"
                />
              </div>
              <label class="mt-4 flex w-full items-center gap-3 text-[13px]">
                <span class="shrink-0 text-[hsl(var(--muted-foreground))]">Zoom</span>
                <input
                  type="range"
                  min="1"
                  max="3"
                  step="0.05"
                  [(ngModel)]="cropZoom"
                  (ngModelChange)="onZoomChange()"
                  class="w-full accent-[hsl(var(--primary))]"
                  aria-label="Crop zoom"
                />
              </label>
              <p class="mt-1 text-[12px] text-[hsl(var(--muted-foreground))]">Drag to reposition · slider to zoom</p>
            </div>
            <div class="mt-5 flex justify-between gap-2">
              <button
                type="button"
                (click)="resetCrop()"
                class="rounded-full px-5 py-2.5 text-[13.5px] font-medium border border-[hsl(var(--border))] hover:bg-white/5"
              >
                Choose different
              </button>
              <button
                type="button"
                (click)="saveCroppedAvatar()"
                class="btn-interactive rounded-full px-5 py-2.5 text-[13.5px] font-semibold bg-[hsl(var(--primary))] text-white"
              >
                Save avatar
              </button>
            </div>
          }
        </div>
      </div>
    }
  `,
})
export class ProfileHeaderComponent {
  readonly profile = input.required<CreatorProfile>();
  readonly loading = input(false);
  readonly profileChange = output<Partial<CreatorProfile>>();

  @ViewChild('cropFrame') cropFrame?: ElementRef<HTMLDivElement>;
  @ViewChild('cropImg') cropImg?: ElementRef<HTMLImageElement>;

  editOpen = false;
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

  openEditDialog(): void {
    const p = this.profile();
    this.draftName = p.name;
    this.draftUsername = p.username;
    this.draftBio = p.bio;
    this.usernameError = '';
    this.editOpen = true;
  }

  closeEditDialog(): void {
    this.editOpen = false;
  }

  saveEditDialog(): void {
    const username = this.draftUsername.trim().replace(/^@/, '');
    if (username && !/^[a-zA-Z0-9_]{2,24}$/.test(username)) {
      this.usernameError = 'Use 2–24 letters, numbers or underscores.';
      return;
    }
    this.editOpen = false;
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
