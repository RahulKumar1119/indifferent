import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { of, throwError } from 'rxjs';
import {
  provideLucideIcons,
  LucideCheck,
  LucideChevronRight,
  LucideCircleX,
  LucideDownload,
  LucideLoader2,
  LucideMoon,
  LucideRefreshCw,
  LucideSparkles,
  LucideSun,
  LucideUser,
  LucideX,
} from '@lucide/angular';
import { Thumbnail, ThumbnailService, ThumbnailSet } from './thumbnail.service';
import { ThumbnailGeneratorComponent } from './thumbnail-generator.component';
import { ThumbnailProgressComponent } from './thumbnail-progress.component';
import { ThumbnailStyleSelectorComponent } from './thumbnail-style-selector.component';

const ICONS = provideLucideIcons(
  LucideCheck, LucideChevronRight, LucideCircleX, LucideDownload, LucideLoader2, LucideMoon, LucideRefreshCw, LucideSparkles, LucideSun, LucideUser, LucideX,
);

const thumbs: Thumbnail[] = [1, 2, 3].map(
  (i) => ({ id: 't' + i, concept: (['face', 'subject', 'creative'] as const)[i - 1], style: 'viral', format: '16:9', headline: 'Headline ' + i, frameTime: i * 10, width: 1280, height: 720, url: 'blob:x' + i, selected: i === 2 }) as Thumbnail,
);
const set = (over: Partial<ThumbnailSet>): ThumbnailSet =>
  ({ status: 'completed', supported: true, jobStatus: 'completed', thumbnails: thumbs, thumbnail: null, ...over }) as ThumbnailSet;

const tidy = (el: HTMLElement) => {
  expect(el.innerHTML).not.toContain('var(--primary');
  expect(el.innerHTML).not.toContain('glass-card');
  expect(el.innerHTML).not.toMatch(/#[0-9A-Fa-f]{6}\b/);
  expect(el.textContent).not.toContain('\u00b7');
};

describe('ThumbnailGeneratorComponent', () => {
  function setup(data: ThumbnailSet | 'error', over: Record<string, unknown> = {}) {
    const service = {
      get: () => (data === 'error' ? throwError(() => new Error('x')) : of(data)),
      start: jasmine.createSpy('start').and.returnValue(of({ status: 'running' })),
      select: jasmine.createSpy('select').and.returnValue(of(set({}))),
      downloadUrl: jasmine.createSpy('downloadUrl').and.returnValue(of({ url: 'blob:dl', filename: 'thumb.jpg' })),
      ...over,
    };
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
        ICONS,
        { provide: ThumbnailService, useValue: service },
        { provide: ActivatedRoute, useValue: { snapshot: { paramMap: convertToParamMap({ id: 'j1' }) } } },
      ],
    });
    const fixture = TestBed.createComponent(ThumbnailGeneratorComponent);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, el, service, cmp: fixture.componentInstance };
  }

  it('shows the suggestions with select and download, one h1, and the link back to the clips', () => {
    const { fixture, el } = setup(set({}));
    fixture.detectChanges();
    expect(el.querySelectorAll('h1').length).toBe(1);
    expect(el.querySelector('h1')!.textContent).toContain('AI thumbnails');
    expect(el.querySelector('a[href="/shorts/j1/clips"]')!.textContent).toContain('Back to your clips');
    expect(el.textContent).toContain('created 3 thumbnail suggestions');
    expect(el.querySelectorAll('app-thumbnail-card').length).toBe(3);
    expect(el.querySelector('button[aria-label="Download thumbnail 1"]')).toBeTruthy();
    expect(el.querySelector('button[aria-label^="Preview thumbnail 1"]')).toBeTruthy();
    expect(el.querySelector('app-navbar')).toBeTruthy();
    tidy(el);
  });

  it('selects a thumbnail and saves it with Use selected thumbnail', () => {
    const { fixture, el, cmp } = setup(set({}));
    fixture.detectChanges();
    const select = Array.from(el.querySelectorAll('app-thumbnail-card button[aria-pressed]'))[0] as HTMLButtonElement;
    select.click();
    fixture.detectChanges();
    expect(cmp.chosenId()).toBe('t1');
    expect(select.getAttribute('aria-pressed')).toBe('true');
    const use = el.querySelector('[data-testid="use-selected"]') as HTMLButtonElement;
    expect(use.disabled).toBeFalse();
    const spy = spyOn(cmp, 'useSelected').and.callThrough();
    use.click();
    expect(spy).toHaveBeenCalled();
  });

  it('marks the thumbnail already saved with the video', () => {
    const { fixture, el } = setup(set({}));
    fixture.detectChanges();
    const cards = Array.from(el.querySelectorAll('app-thumbnail-card'));
    expect(cards[1].textContent).toContain('Your thumbnail');
    expect(cards[0].textContent).not.toContain('Your thumbnail');
  });

  it('opens the style options with Regenerate, and starts a new run', () => {
    const { fixture, el, service } = setup(set({}));
    fixture.detectChanges();
    const regen = Array.from(el.querySelectorAll('button')).find((b) => b.textContent!.trim() === 'Regenerate') as HTMLButtonElement;
    regen.click();
    fixture.detectChanges();
    expect(regen.getAttribute('aria-expanded')).toBe('true');
    expect(el.querySelector('app-thumbnail-style-selector')).toBeTruthy();
    (el.querySelector('[data-testid="generate"]') as HTMLButtonElement).click();
    expect(service.start).toHaveBeenCalled();
  });

  it('offers Generate thumbnails when there are none yet', () => {
    const { fixture, el } = setup(set({ status: 'none' as ThumbnailSet['status'], thumbnails: [] }));
    fixture.detectChanges();
    expect(el.textContent).toContain('Create thumbnails for this video');
    expect(el.querySelector('[data-testid="generate"]')!.textContent).toContain('Generate thumbnails');
  });

  it('shows progress while a run is going', () => {
    const { fixture, el } = setup(set({ status: 'running' as ThumbnailSet['status'], stage: 'concepts' as ThumbnailSet['stage'], thumbnails: [] }));
    fixture.detectChanges();
    expect(el.textContent).toContain('Making your thumbnails');
    expect(el.querySelector('app-thumbnail-progress [aria-current="step"]')).toBeTruthy();
  });

  it('explains audio uploads have no picture', () => {
    const { fixture, el } = setup(set({ status: 'none' as ThumbnailSet['status'], supported: false, thumbnails: [] }));
    fixture.detectChanges();
    expect(el.textContent).toContain('No video to work from');
  });

  it('shows a failure with its help text, and the load error with a retry', () => {
    const { fixture, el } = setup(set({ status: 'failed' as ThumbnailSet['status'] }));
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')!.textContent).toContain("Thumbnail generation didn't finish");
    TestBed.resetTestingModule();
    const second = setup('error');
    second.fixture.detectChanges();
    const alert = second.el.querySelector('[role="alert"]')!;
    expect(alert.textContent).toContain("We couldn't load your thumbnails.");
    expect(alert.querySelector('button')!.textContent).toContain('Try again');
  });
});

describe('ThumbnailProgressComponent', () => {
  it('marks done, active and waiting steps for screen readers', () => {
    TestBed.configureTestingModule({ providers: [ICONS] });
    const fixture = TestBed.createComponent(ThumbnailProgressComponent);
    fixture.componentRef.setInput('stage', 'concepts');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const states = Array.from(el.querySelectorAll('li[data-state]')).map((l) => l.getAttribute('data-state'));
    expect(states).toContain('done');
    expect(states.filter((s) => s === 'active').length).toBe(1);
    expect(el.textContent).toContain('(in progress)');
  });
});

describe('ThumbnailStyleSelectorComponent', () => {
  it('uses labelled radio groups for style and shape', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(ThumbnailStyleSelectorComponent);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(Array.from(el.querySelectorAll('legend')).map((l) => l.textContent!.trim())).toEqual(['Thumbnail style', 'Shape']);
    const styles = el.querySelectorAll('input[name="thumb-style"]');
    expect(styles.length).toBeGreaterThan(1);
    (styles[1] as HTMLInputElement).click();
    fixture.detectChanges();
    expect(fixture.componentInstance.style()).not.toBe('');
    expect(el.querySelectorAll('input[name="thumb-format"]').length).toBe(3);
  });
});
