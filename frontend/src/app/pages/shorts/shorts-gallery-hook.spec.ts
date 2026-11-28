import { TestBed, discardPeriodicTasks, fakeAsync, tick } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { of } from 'rxjs';
import {
  provideLucideIcons, LucideCaptions, LucideCheck, LucideCircleX, LucideClapperboard, LucideCopy, LucideDownload, LucideFilm, LucideImage,
  LucideLoader2, LucideMoon, LucidePencil, LucidePlay, LucideRefreshCw, LucideShare2, LucideSparkles, LucideSun, LucideTrash2, LucideUser, LucideX, LucideZap,
} from '@lucide/angular';
import { Clip, ShortsService } from './shorts.service';
import { PostCopyService } from '../../shared/post-copy/post-copy.service';
import { ShortsGalleryComponent } from './shorts-gallery.component';
import { ClipHookService } from './clip-hook.service';
import { ClipRenderView, HookView, defaultHook } from './clip-hook.models';

const ICONS = provideLucideIcons(
  LucideCaptions, LucideCheck, LucideCircleX, LucideClapperboard, LucideCopy, LucideDownload, LucideFilm, LucideImage, LucideLoader2,
  LucideMoon, LucidePencil, LucidePlay, LucideRefreshCw, LucideShare2, LucideSparkles, LucideSun, LucideTrash2, LucideUser, LucideX, LucideZap,
);
const clips: Clip[] = [1, 2].map((r) => ({ clipId: 'c' + r, s3Key: 'k', rank: r, score: 0.9, duration: 30, hookText: 'Hook ' + r, start: r * 60, end: r * 60 + 30 }));
const hookView: HookView = {
  hook: defaultHook(), saved: false, ranker: 'Hook 1', suggestionsEnabled: false, renderEnabled: true,
  suggestDailyLimit: 20, renderRemainingToday: 15, renderDailyLimit: 15, maxChars: 80,
};

describe('ShortsGalleryComponent hook (#329)', () => {
  let renders: Record<string, ClipRenderView>[];
  let urlCalls: string[];

  function setup() {
    urlCalls = [];
    const shorts = {
      listClips: () => of(clips),
      getClipUrl: (_j: string, c: string) => {
        urlCalls.push(c);
        return of({});
      },
      getStatus: () => of({ jobId: 'j1', status: 'completed', fileType: 'mp4', sourceDuration: 600, clipRenders: renders.shift() ?? {} }),
      getClipCaptions: () => of({ start: 0, end: 1, words: [] }),
    };
    const pc = jasmine.createSpyObj<PostCopyService>('PostCopyService', ['view', 'list', 'write', 'save']);
    pc.list.and.returnValue(of({ enabled: false, platforms: [], saved: {}, dailyLimit: 20 }));
    const hooks = jasmine.createSpyObj<ClipHookService>('ClipHookService', ['view', 'save', 'reset', 'suggest', 'render']);
    hooks.view.and.returnValue(of(hookView));
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(), provideHttpClientTesting(), provideRouter([]), ICONS,
        { provide: ShortsService, useValue: shorts },
        { provide: PostCopyService, useValue: pc },
        { provide: ClipHookService, useValue: hooks },
        { provide: ActivatedRoute, useValue: { snapshot: { paramMap: convertToParamMap({ id: 'j1' }) } } },
      ],
    });
    const fixture = TestBed.createComponent(ShortsGalleryComponent);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, hooks };
  }

  it('opens the hook panel and previews the draft over the clip, labelled approximate', fakeAsync(() => {
    renders = [{}];
    const { fixture, el } = setup();
    tick();
    fixture.detectChanges();
    const card = el.querySelector('[data-clip="c1"]') as HTMLElement;
    (card.querySelector('[data-testid="hook-toggle"]') as HTMLButtonElement).click();
    fixture.detectChanges();
    tick();
    fixture.detectChanges();
    expect(card.querySelector('[data-testid="clip-hook"]')).not.toBeNull();
    // Off by default: nothing over the clip.
    expect(card.querySelector('[data-testid="hook-preview"]')).toBeNull();
    (card.querySelector('[data-testid="hook-enabled"]') as HTMLButtonElement).click();
    const input = card.querySelector('[data-testid="hook-text"]') as HTMLInputElement;
    input.value = 'यह गलती मत करना';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    tick();
    fixture.detectChanges();
    const ov = card.querySelector('[data-testid="hook-preview"]') as HTMLElement;
    expect(ov.textContent).toBe('यह गलती मत करना');
    expect(ov.getAttribute('lang')).toBe('hi');
    expect(ov.classList).toContain('box');
    expect(ov.classList).toContain('top');
    expect(card.textContent).toContain('Hook preview, approximate');
    // Closing the panel takes the preview away.
    (card.querySelector('[data-testid="hook-toggle"]') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(card.querySelector('[data-testid="hook-preview"]')).toBeNull();
    tick(50); // the preview loop ends on its next frame
    discardPeriodicTasks();
  }));

  it('shows Rendering again on the card, and loads the new file when it is done', fakeAsync(() => {
    renders = [{ c2: { state: 'rendering' } }, { c2: { state: 'rendering' } }, { c2: { state: 'done', renders: 1 } }];
    const { fixture, el } = setup();
    tick();
    fixture.detectChanges();
    const card = el.querySelector('[data-clip="c2"]') as HTMLElement;
    expect(card.querySelector('[data-testid="clip-rendering"]')?.textContent).toContain('Rendering again');
    expect(el.querySelector('[data-clip="c1"] [data-testid="clip-rendering"]')).toBeNull();
    tick(10000);
    fixture.detectChanges();
    expect(card.querySelector('[data-testid="clip-rendering"]')).not.toBeNull();
    const before = urlCalls.filter((c) => c === 'c2').length;
    tick(10000);
    fixture.detectChanges();
    expect(card.querySelector('[data-testid="clip-rendering"]')).toBeNull();
    expect(urlCalls.filter((c) => c === 'c2').length).toBe(before + 1);
    // Nothing is rendering any more, so the page stops asking.
    tick(30000);
    expect(renders.length).toBe(0);
    discardPeriodicTasks();
  }));

  it('starts following a render the panel started', fakeAsync(() => {
    renders = [{}, { c1: { state: 'done', renders: 1 } }];
    const { fixture } = setup();
    tick();
    fixture.detectChanges();
    const gallery = fixture.componentInstance;
    gallery.onRenderStarted(clips[0], { state: 'rendering' });
    fixture.detectChanges();
    expect(gallery.clipRenders['c1'].state).toBe('rendering');
    tick(10000);
    fixture.detectChanges();
    expect(gallery.clipRenders['c1'].state).toBe('done');
    discardPeriodicTasks();
  }));
});
