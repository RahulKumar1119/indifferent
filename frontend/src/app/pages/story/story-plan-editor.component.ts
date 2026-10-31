import { DOCUMENT } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  HostListener,
  Injector,
  OnDestroy,
  OnInit,
  afterNextRender,
  computed,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { Subscription } from 'rxjs';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { setPageSeo, setRobots, ROBOTS_NOINDEX } from '../../shared/seo';
import { StoryAttributionComponent } from './story-attribution.component';
import { StoryMediaPickerComponent } from './story-media-picker.component';
import {
  PendingMedia,
  buildEdits,
  changedSceneCount,
  currentRef,
  sceneMediaView,
  withPick,
  withRemoval,
} from './story-plan-edits';
import { StoryPlanProblem, describeProblem } from './story-plan-problem';
import { StockAsset, StoryPlanRecord } from './story-plan.models';
import { StoryPlanService } from './story-plan.service';
import { StoryProblemComponent } from './story-problem.component';
import { StorySceneCardComponent } from './story-scene-card.component';
import { StoryVideoPanelComponent } from './story-video-panel.component';
import { videoBlocker } from './story-video-blockers';

/**
 * The plan for one story: the hook, then each scene with its narration and its
 * picture. Narration is edited in place and pictures are chosen from search
 * results; everything is saved together, against the revision that was loaded,
 * so an edit made from an old copy is refused instead of overwriting a newer one.
 */
@Component({
  selector: 'app-story-plan-editor',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [
    RouterLink,
    LucideDynamicIcon,
    NavbarComponent,
    StoryAttributionComponent,
    StoryMediaPickerComponent,
    StoryProblemComponent,
    StorySceneCardComponent,
    StoryVideoPanelComponent,
  ],
  styles: [
    `
      .serif {
        font-family: 'Cormorant Garamond', Georgia, serif;
      }
    `,
  ],
  template: `
    <app-navbar></app-navbar>
    <main class="mx-auto w-full max-w-5xl px-4 py-8">
      <a
        routerLink="/story"
        class="inline-flex min-h-11 items-center gap-2 text-sm underline underline-offset-2 hover:text-[hsl(var(--muted-foreground))]"
        >Plan another video</a
      >

      @if (loading()) {
        <p role="status" class="mt-6 flex items-center gap-3 text-[hsl(var(--muted-foreground))]">
          <svg lucideIcon="loader-2" [size]="18" aria-hidden="true" class="animate-spin motion-reduce:animate-none"></svg>
          Loading your plan…
        </p>
      } @else if (loadProblem(); as p) {
        <div class="mt-6 max-w-xl space-y-4">
          <app-story-problem [problem]="p" [actionLabel]="p.retryable ? 'Try again' : ''" (action)="load()" />
          @if (!p.retryable) {
            <a
              routerLink="/story"
              class="inline-flex min-h-11 items-center rounded-full bg-[#BC5227] px-5 text-sm font-semibold text-white hover:bg-[#a84822]"
              >Plan a new video</a
            >
          }
        </div>
      } @else if (record(); as rec) {
        @if (rec.plan; as plan) {
          <h1 #heading tabindex="-1" class="serif mt-4 text-4xl font-medium leading-tight sm:text-5xl">Your video plan</h1>
          <p class="mt-3 max-w-2xl text-[hsl(var(--muted-foreground))]">
            Check each scene, change the narration, and choose a picture for it. Saving keeps your plan. When it looks
            right, make the video below the last scene.
          </p>

          <section class="mt-6 rounded-2xl border border-[hsl(var(--border))] p-4 sm:p-5" aria-labelledby="hook-label">
            <h2 id="hook-label" class="text-sm font-medium text-[hsl(var(--muted-foreground))]">Opening line</h2>
            <p class="serif mt-1 text-2xl leading-snug [overflow-wrap:anywhere]" [attr.lang]="plan.language">{{ plan.hook }}</p>
            <p class="mt-3 text-sm text-[hsl(var(--muted-foreground))]" data-testid="summary">
              {{ plan.scenes.length }} scenes, about {{ roundedSeconds(plan.totalSeconds) }} seconds. The limit is 90.
            </p>
          </section>

          @if (plan.warnings?.length) {
            <section class="mt-4 rounded-2xl border border-[hsl(var(--border))] p-4 text-sm" aria-labelledby="notes-label" data-testid="warnings">
              <h2 id="notes-label" class="flex items-center gap-2 font-semibold">
                <svg lucideIcon="triangle-alert" [size]="16" aria-hidden="true"></svg>
                Notes about this plan
              </h2>
              <ul class="mt-2 list-disc space-y-1 pl-5 text-[hsl(var(--muted-foreground))]">
                @for (w of plan.warnings; track $index) {
                  <li>{{ w }}</li>
                }
              </ul>
            </section>
          }

          <ol class="mt-6 space-y-5">
            @for (scene of plan.scenes; track $index; let i = $index) {
              <li>
                <app-story-scene-card
                  [index]="i"
                  [scene]="scene"
                  [narration]="narrations()[i]"
                  [media]="views()[i]"
                  [language]="plan.language"
                  [disabled]="saving()"
                  [edited]="isEdited(i)"
                  (narrationChange)="setNarration(i, $event)"
                  (choose)="openPicker(i)"
                  (remove)="removePicture(i)"
                />
              </li>
            }
          </ol>

          <div class="mt-6">
            <app-story-attribution [attributions]="rec.mediaAttributions ?? []" />
          </div>

          <app-story-video-panel
            [planId]="rec.planId"
            [revision]="rec.revision"
            [language]="plan.language"
            [blocker]="videoReason()"
            (planChanged)="load()"
            (goToScene)="focusScene($event)"
          />

          <div
            class="sticky bottom-0 z-10 -mx-4 mt-6 border-t border-[hsl(var(--border))] bg-[hsl(var(--background))] px-4 py-3"
          >
            <div class="mx-auto max-h-[50dvh] max-w-5xl space-y-3 overflow-y-auto">
              @if (saveProblem(); as p) {
                <app-story-problem
                  [problem]="p"
                  [actionLabel]="saveActionLabel(p)"
                  (action)="onSaveProblemAction(p)"
                />
              }
              <div class="flex flex-wrap items-center gap-3">
                <p role="status" class="mr-auto text-sm" data-testid="save-status">
                  @if (saving()) {
                    Saving…
                  } @else if (dirty()) {
                    {{ changedCount() }} {{ changedCount() === 1 ? 'scene has' : 'scenes have' }} changes that are not saved
                  } @else if (justSaved()) {
                    <span class="inline-flex items-center gap-2 font-semibold">
                      <svg lucideIcon="check" [size]="16" aria-hidden="true"></svg>
                      Saved
                    </span>
                  } @else {
                    <span class="text-[hsl(var(--muted-foreground))]">No changes yet</span>
                  }
                </p>
                @if (dirty() && !saving()) {
                  <button
                    type="button"
                    (click)="discard()"
                    class="inline-flex min-h-11 items-center rounded-full border border-[hsl(var(--border))] px-4 text-sm font-medium hover:bg-[hsl(var(--muted))]"
                  >
                    Discard changes
                  </button>
                }
                <button
                  type="button"
                  (click)="save()"
                  [disabled]="!dirty() || saving()"
                  class="inline-flex min-h-11 items-center gap-2 rounded-full bg-[#BC5227] px-6 text-sm font-semibold text-white hover:bg-[#a84822] disabled:cursor-not-allowed disabled:opacity-50"
                >
                  @if (saving()) {
                    <svg lucideIcon="loader-2" [size]="16" aria-hidden="true" class="animate-spin motion-reduce:animate-none"></svg>
                  }
                  Save changes
                </button>
              </div>
            </div>
          </div>
        }
      }
    </main>

    <app-story-media-picker (picked)="onPicked($event)" />
  `,
})
export class StoryPlanEditorComponent implements OnInit, OnDestroy {
  private readonly route = inject(ActivatedRoute);
  private readonly service = inject(StoryPlanService);
  private readonly document = inject(DOCUMENT);
  private readonly injector = inject(Injector);
  private readonly picker = viewChild.required(StoryMediaPickerComponent);

  protected readonly loading = signal(true);
  protected readonly loadProblem = signal<StoryPlanProblem | null>(null);
  protected readonly record = signal<StoryPlanRecord | null>(null);
  /** Narration as typed, one entry per scene. */
  protected readonly narrations = signal<string[]>([]);
  /** Picture changes not saved yet, one slot per scene. */
  protected readonly pending = signal<(PendingMedia | undefined)[]>([]);
  protected readonly saving = signal(false);
  protected readonly saveProblem = signal<StoryPlanProblem | null>(null);
  protected readonly justSaved = signal(false);

  protected readonly edits = computed(() => {
    const rec = this.record();
    return rec ? buildEdits(rec, this.narrations(), this.pending()) : [];
  });
  protected readonly changedCount = computed(() => changedSceneCount(this.edits()));
  protected readonly dirty = computed(() => this.changedCount() > 0);
  protected readonly views = computed(() => {
    const rec = this.record();
    const pending = this.pending();
    return (rec?.plan?.scenes ?? []).map((_, i) => sceneMediaView(rec!, i, pending[i]));
  });

  /** Why a video cannot be made from this plan yet; empty when it can. */
  protected readonly videoReason = computed(() => {
    const rec = this.record();
    return rec ? videoBlocker(rec, this.dirty()) : '';
  });

  private heading = viewChild<{ nativeElement: HTMLElement }>('heading');
  private pickerScene = -1;
  private planId = '';
  private routeSub?: Subscription;
  private requestSub?: Subscription;

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Your video plan | Indifferent',
      description: 'Edit the narration and pictures of a planned video.',
      canonical: 'https://indifferent.fun/story',
    });
    setRobots(this.document, ROBOTS_NOINDEX);
    this.routeSub = this.route.paramMap.subscribe((params) => {
      this.planId = params.get('id') ?? '';
      this.load();
    });
  }

  ngOnDestroy(): void {
    this.routeSub?.unsubscribe();
    this.requestSub?.unsubscribe();
  }

  /** Warns before the tab is closed or reloaded with edits that are not saved. */
  @HostListener('window:beforeunload', ['$event'])
  protected onBeforeUnload(event: BeforeUnloadEvent): void {
    if (this.dirty()) event.preventDefault();
  }

  /** Router guard: ask before leaving the page with edits that are not saved. */
  canLeave(): boolean {
    return !this.dirty() || this.document.defaultView!.confirm('You have changes that are not saved. Leave without saving?');
  }

  load(): void {
    this.requestSub?.unsubscribe();
    this.loading.set(true);
    this.loadProblem.set(null);
    this.requestSub = this.service.get(this.planId).subscribe({
      next: (rec) => {
        this.adopt(rec);
        this.loading.set(false);
        if (rec.status !== 'ready' || !rec.plan) {
          this.loadProblem.set(
            rec.status === 'failed'
              ? {
                  kind: 'not_ready',
                  title: "This plan didn't finish",
                  message: "The planner couldn't make a usable plan from that story. Start a new plan.",
                  retryable: false,
                }
              : {
                  kind: 'not_ready',
                  title: 'This plan was never finished',
                  message: 'It was interrupted before it was done. Start a new plan.',
                  retryable: false,
                },
          );
          return;
        }
        afterNextRender(() => this.heading()?.nativeElement.focus(), { injector: this.injector });
      },
      error: (err) => {
        this.loading.set(false);
        this.loadProblem.set(describeProblem(err, 'load'));
      },
    });
  }

  protected setNarration(index: number, text: string): void {
    this.justSaved.set(false);
    this.narrations.update((n) => n.map((v, i) => (i === index ? text : v)));
  }

  protected openPicker(index: number): void {
    const rec = this.record();
    const scene = rec?.plan?.scenes[index];
    if (!rec || !scene) return;
    this.pickerScene = index;
    this.picker().open(scene, index + 1, currentRef(rec, index, this.pending()[index]));
  }

  protected onPicked(asset: StockAsset): void {
    const rec = this.record();
    if (!rec || this.pickerScene < 0) return;
    this.justSaved.set(false);
    this.pending.update((p) => withPick(rec, p, this.pickerScene, asset));
  }

  protected removePicture(index: number): void {
    const rec = this.record();
    if (!rec) return;
    this.justSaved.set(false);
    this.pending.update((p) => withRemoval(rec, p, index));
    // The button that was pressed is about to disappear: keep focus in the scene.
    afterNextRender(() => this.document.getElementById(`scene-${index + 1}-choose`)?.focus(), { injector: this.injector });
  }

  /** Moves to a scene's picture button, for a failure that choosing another picture fixes. */
  protected focusScene(sceneNumber: number): void {
    this.document.getElementById(`scene-${sceneNumber}-choose`)?.focus();
  }

  protected isEdited(index: number): boolean {
    const e = this.edits()[index];
    return !!e && (e.narration !== undefined || e.media !== undefined || !!e.clearMedia);
  }

  protected roundedSeconds(total: number): number {
    return Math.round(total);
  }

  protected save(): void {
    const rec = this.record();
    if (!rec || this.saving() || !this.dirty()) return;
    this.saving.set(true);
    this.saveProblem.set(null);
    this.justSaved.set(false);
    this.requestSub?.unsubscribe();
    this.requestSub = this.service.save(rec.planId, rec.revision, this.edits()).subscribe({
      next: (updated) => {
        this.adopt(updated);
        this.justSaved.set(true);
      },
      error: (err) => {
        // Everything typed and chosen stays, so the user can retry or copy it out.
        this.saveProblem.set(describeProblem(err, 'save'));
        this.saving.set(false);
      },
    });
  }

  protected discard(): void {
    const rec = this.record();
    if (rec) this.adopt(rec);
  }

  protected saveActionLabel(p: StoryPlanProblem): string {
    if (p.kind === 'stale') return 'Reload the latest plan';
    return p.retryable ? 'Try saving again' : '';
  }

  protected onSaveProblemAction(p: StoryPlanProblem): void {
    if (p.kind === 'stale') this.load();
    else this.save();
  }

  /** Makes a plan from the server the page's truth and drops all unsaved edits. */
  private adopt(rec: StoryPlanRecord): void {
    this.record.set(rec);
    this.narrations.set((rec.plan?.scenes ?? []).map((s) => s.narration));
    this.pending.set([]);
    this.saving.set(false);
    this.saveProblem.set(null);
    this.justSaved.set(false);
  }
}
