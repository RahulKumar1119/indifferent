import { DOCUMENT, NgTemplateOutlet } from '@angular/common';
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
  SceneMediaView,
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
type ScenePicture = 'chosen' | 'picked' | 'missing' | 'problem';

@Component({
  selector: 'app-story-plan-editor',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [
    NgTemplateOutlet,
    RouterLink,
    LucideDynamicIcon,
    NavbarComponent,
    StoryAttributionComponent,
    StoryMediaPickerComponent,
    StoryProblemComponent,
    StorySceneCardComponent,
    StoryVideoPanelComponent,
  ],
  styles: [`
    :host { display: block; }
    .back { display: inline-flex; align-items: center; min-height: 44px; font-size: var(--type-sm); align-self: flex-start; }
    .loading { margin: 0; display: flex; align-items: center; gap: var(--space-sm); color: var(--color-ink-2); }
    .stack { display: flex; flex-direction: column; gap: var(--space-md); max-width: 36rem; }
    .editor { display: grid; gap: var(--space-lg); grid-template-columns: minmax(0, 1fr); }
    .intro, .work { display: flex; flex-direction: column; gap: var(--space-md); min-width: 0; }
    /* The heading takes focus on load so screen readers start here; it is not a control, so no ring. */
    h1:focus, h1:focus-visible { outline: none; }
    .hook-label { margin: 0; font-size: var(--type-sm); font-weight: 600; color: var(--color-ink-2); }
    .hook { margin: var(--space-2xs) 0 0; font-family: var(--font-display); font-size: var(--type-xl); line-height: 1.3; overflow-wrap: anywhere; }
    .summary { margin: var(--space-sm) 0 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .notes h2 { margin: 0; display: flex; align-items: center; gap: var(--space-xs); font-size: var(--type-md); font-weight: 600; }
    .notes ul { margin: var(--space-xs) 0 0; padding-left: var(--space-lg); list-style: disc; color: var(--color-ink-2); font-size: var(--type-sm); }
    .notes li + li { margin-top: var(--space-3xs); }
    .scenes { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: var(--space-md); }
    .rail { display: flex; flex-direction: column; gap: var(--space-md); min-width: 0; }
    .idx-title { margin: 0 0 var(--space-2xs); font-size: var(--type-sm); font-weight: 600; color: var(--color-ink-2); }
    .idx { margin: 0; padding: 0 0 var(--space-3xs); list-style: none; display: flex; gap: var(--space-xs); overflow-x: auto; overscroll-behavior-x: contain; }
    .idx button { position: relative; display: inline-flex; align-items: center; gap: var(--space-xs); min-height: 44px; padding: 0 var(--space-sm); white-space: nowrap; border: 1px solid var(--color-rule); border-radius: var(--radius-pill); background: var(--app-card); color: var(--color-ink); font: 500 var(--type-sm)/1.2 var(--font-body); cursor: pointer; transition: background-color var(--dur-short) var(--ease-out), border-color var(--dur-short) var(--ease-out); }
    .idx button:hover { background: var(--color-paper-3); }
    .idx button[aria-current='true'] { border-color: var(--color-accent); box-shadow: inset 0 0 0 1px var(--color-accent); font-weight: 600; }
    .idx .ok { color: var(--color-positive); }
    .idx .miss { color: var(--color-ink-2); }
    .idx .auto { color: var(--color-accent); }
    .idx .bad { color: var(--color-negative); }
    .word, .ed > span { position: absolute; width: 1px; height: 1px; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
    .ed { display: inline-flex; align-items: center; }
    .ed::before { content: ''; width: 0.5rem; height: 0.5rem; border-radius: 50%; background: var(--color-caution); }
    .save { display: flex; flex-direction: column; gap: var(--space-sm); }
    .save-row { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-sm); }
    .save-status { margin: 0 auto 0 0; font-size: var(--type-sm); }
    .saved { display: inline-flex; align-items: center; gap: var(--space-xs); font-weight: 600; color: var(--color-positive); }
    .muted { color: var(--color-ink-2); }
    .bar { position: sticky; bottom: 0; z-index: 10; margin-inline: calc(-1 * var(--space-md)); padding: var(--space-sm) var(--space-md); border-top: 1px solid var(--color-rule); background: var(--app-page); }
    @media (min-width: 40rem) { .bar { margin-inline: calc(-1 * var(--space-lg)); padding-inline: var(--space-lg); } }
    .bar .save { max-height: 50dvh; overflow-y: auto; }
    @media (min-width: 64rem) {
      .editor { grid-template-columns: minmax(0, 1fr) 17rem; grid-template-areas: 'intro rail' 'work rail'; align-items: start; }
      .intro { grid-area: intro; }
      .work { grid-area: work; }
      .rail { grid-area: rail; position: sticky; top: calc(56px + var(--space-md)); max-height: calc(100dvh - 56px - 2 * var(--space-md)); }
      .rail nav { display: flex; flex-direction: column; min-height: 0; flex: 1 1 auto; }
      .idx { flex-direction: column; overflow-x: visible; overflow-y: auto; padding: 2px; }
      .idx button { width: 100%; border-radius: var(--radius-control); }
      .word { position: static; width: auto; height: auto; margin: 0 0 0 auto; overflow: visible; clip: auto; font-size: var(--type-xs); font-weight: 400; color: var(--color-ink-2); }
      .rail .save { flex: none; }
      .rail .save .ui-btn { flex: 1 1 auto; }
    }
    @media (prefers-reduced-motion: reduce) { .idx button { transition: none; } }
  `],
  template: `
    <div class="ui-app">
      <app-navbar></app-navbar>
      <main id="main" class="ui-app-main hm">
        <a routerLink="/story" class="ui-link back">Plan another video</a>

        @if (loading()) {
          <p role="status" class="loading">
            <svg lucideIcon="loader-2" [size]="18" aria-hidden="true" class="ui-spin"></svg>
            Loading your plan…
          </p>
        } @else if (loadProblem(); as p) {
          <div class="stack">
            <h1 class="ui-h1">Your video plan</h1>
            <app-story-problem [problem]="p" [actionLabel]="p.retryable ? 'Try again' : ''" (action)="load()" />
            @if (!p.retryable) {
              <a routerLink="/story" class="ui-btn ui-btn--primary" style="align-self: flex-start">Plan a new video</a>
            }
          </div>
        } @else if (record(); as rec) {
          @if (rec.plan; as plan) {
            <div class="editor">
              <div class="intro">
                <div>
                  <h1 #heading tabindex="-1" class="ui-h1">Your video plan</h1>
                  <p class="ui-lead">
                    Check each scene, change the narration, and choose a picture for it. Saving keeps your plan. When it looks
                    right, make the video below the last scene.
                  </p>
                </div>

                <section class="ui-card" aria-labelledby="hook-label">
                  <h2 id="hook-label" class="hook-label">Opening line</h2>
                  <p class="hook" [attr.lang]="plan.language">{{ plan.hook }}</p>
                  <p class="summary" data-testid="summary">
                    {{ plan.scenes.length }} scenes, about {{ roundedSeconds(plan.totalSeconds) }} seconds. The limit is 90.
                  </p>
                </section>

                @if (plan.warnings?.length) {
                  <section class="ui-alert ui-alert--warn notes" style="margin: 0" aria-labelledby="notes-label" data-testid="warnings">
                    <h2 id="notes-label">
                      <svg lucideIcon="triangle-alert" [size]="16" aria-hidden="true"></svg>
                      Notes about this plan
                    </h2>
                    <ul>
                      @for (w of plan.warnings; track $index) {
                        <li>{{ w }}</li>
                      }
                    </ul>
                  </section>
                }
              </div>

              <aside class="rail" aria-label="Scenes and saving">
                <nav aria-labelledby="index-title" data-testid="scene-index">
                  <h2 id="index-title" class="idx-title">Scenes</h2>
                  <ol class="idx">
                    @for (item of indexItems(); track item.n) {
                      <li>
                        <button
                          type="button"
                          (click)="goToScene(item.n)"
                          [attr.aria-current]="currentScene() === item.n ? 'true' : null"
                          [attr.data-testid]="'index-scene-' + item.n"
                          [attr.data-picture]="item.picture"
                          [attr.data-edited]="item.edited"
                        >
                          @switch (item.picture) {
                            @case ('chosen') { <svg lucideIcon="check" [size]="16" aria-hidden="true" class="ok"></svg> }
                            @case ('picked') { <svg lucideIcon="wand-sparkles" [size]="16" aria-hidden="true" class="auto"></svg> }
                            @case ('missing') { <svg lucideIcon="image" [size]="16" aria-hidden="true" class="miss"></svg> }
                            @default { <svg lucideIcon="triangle-alert" [size]="16" aria-hidden="true" class="bad"></svg> }
                          }
                          Scene {{ item.n }}
                          <span class="word">{{ pictureWords[item.picture] }}</span>
                          @if (item.edited) {
                            <span class="ed" data-testid="index-edited"><span>, edited, not saved</span></span>
                          }
                        </button>
                      </li>
                    }
                  </ol>
                </nav>
                @if (wide()) {
                  <div class="ui-card">
                    <ng-container [ngTemplateOutlet]="saveBlock" />
                  </div>
                }
              </aside>

              <div class="work">
                <ol class="scenes">
                  @for (scene of plan.scenes; track $index; let i = $index) {
                    <li (focusin)="currentScene.set(i + 1)">
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

                <app-story-attribution [attributions]="rec.mediaAttributions ?? []" />

                <app-story-video-panel
                  [planId]="rec.planId"
                  [revision]="rec.revision"
                  [language]="plan.language"
                  [blocker]="videoReason()"
                  (planChanged)="load()"
                  (goToScene)="focusScene($event)"
                />
              </div>
            </div>

            @if (!wide()) {
              <div class="bar">
                <ng-container [ngTemplateOutlet]="saveBlock" />
              </div>
            }
          }
        }
      </main>
    </div>

    <ng-template #saveBlock>
      <div class="save">
        @if (saveProblem(); as p) {
          <app-story-problem [problem]="p" [actionLabel]="saveActionLabel(p)" (action)="onSaveProblemAction(p)" />
        }
        <div class="save-row">
          <p role="status" class="save-status" data-testid="save-status">
            @if (saving()) {
              Saving…
            } @else if (dirty()) {
              {{ changedCount() }} {{ changedCount() === 1 ? 'scene has' : 'scenes have' }} changes that are not saved
            } @else if (justSaved()) {
              <span class="saved">
                <svg lucideIcon="check" [size]="16" aria-hidden="true"></svg>
                Saved
              </span>
            } @else {
              <span class="muted">No changes yet</span>
            }
          </p>
          @if (dirty() && !saving()) {
            <button type="button" (click)="discard()" class="ui-btn ui-btn--secondary">Discard changes</button>
          }
          <button type="button" (click)="save()" [disabled]="!dirty() || saving()" [attr.aria-busy]="saving()" class="ui-btn ui-btn--primary">
            @if (saving()) {
              <svg lucideIcon="loader-2" [size]="16" aria-hidden="true" class="ui-spin"></svg>
            }
            Save changes
          </button>
        </div>
      </div>
    </ng-template>

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

  /**
   * The scene index: one entry per scene, read from what the page already holds
   * (the picture each scene shows and whether it has unsaved edits).
   */
  protected readonly indexItems = computed(() =>
    this.views().map((v, i) => ({
      n: i + 1,
      picture: this.pictureState(v),
      edited: this.isEdited(i),
    })),
  );
  protected readonly pictureWords: Record<ScenePicture, string> = {
    chosen: 'Picture chosen',
    picked: 'Picked for you',
    missing: 'No picture',
    problem: 'Picture problem',
  };
  /** The scene last moved to from the index or edited; 0 when none yet. */
  protected readonly currentScene = signal(0);
  /** Wide screens put the index and the save controls in a sticky rail; narrow ones keep the bottom bar. */
  protected readonly wide = signal(false);
  private wideQuery?: MediaQueryList;
  private readonly onWideChange = (e: MediaQueryListEvent) => this.wide.set(e.matches);

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
    const view = this.document.defaultView;
    if (view && typeof view.matchMedia === 'function') {
      this.wideQuery = view.matchMedia('(min-width: 64rem)');
      this.wide.set(this.wideQuery.matches);
      this.wideQuery.addEventListener('change', this.onWideChange);
    }
    this.routeSub = this.route.paramMap.subscribe((params) => {
      this.planId = params.get('id') ?? '';
      this.load();
    });
  }

  ngOnDestroy(): void {
    this.wideQuery?.removeEventListener('change', this.onWideChange);
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
    this.currentScene.set(sceneNumber);
    this.document.getElementById(`scene-${sceneNumber}-choose`)?.focus();
  }

  /** From the scene index: move to the scene's heading, which scrolls it into view. */
  protected goToScene(sceneNumber: number): void {
    this.currentScene.set(sceneNumber);
    // Bring the scene's top under the navbar (its scroll-margin), then focus the heading without a second jump.
    this.document.getElementById(`scene-${sceneNumber}`)?.scrollIntoView({ block: 'start' });
    this.document.getElementById(`scene-${sceneNumber}-title`)?.focus({ preventScroll: true });
  }

  /** What the index says about a scene's picture. An auto-picked one is told apart so the user can see which to check. */
  private pictureState(v: SceneMediaView): ScenePicture {
    if (v.state === 'none') return 'missing';
    if (v.state !== 'asset') return 'problem';
    return v.auto ? 'picked' : 'chosen';
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
