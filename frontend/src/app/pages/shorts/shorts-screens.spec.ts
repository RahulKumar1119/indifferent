import { TestBed, fakeAsync, tick, discardPeriodicTasks } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { ActivatedRoute, Router, convertToParamMap, provideRouter } from '@angular/router';
import { of, throwError } from 'rxjs';
import {
  provideLucideIcons,
  LucideCheck,
  LucideChevronRight,
  LucideCircleCheck,
  LucideCircleX,
  LucideClock,
  LucideFileText,
  LucideLoader2,
  LucideMic,
  LucideMoon,
  LucidePlay,
  LucidePlus,
  LucideScissors,
  LucideSun,
  LucideTrash2,
  LucideUpload,
  LucideUser,
  LucideVideo,
  LucideX,
} from '@lucide/angular';
import { ShortsService, ShortsJobSummary, DEFAULT_SHORTS_LIMITS } from './shorts.service';
import { ShortsUploadComponent } from './shorts-upload.component';
import { ShortsHistoryComponent } from './shorts-history.component';
import { ShortsProgressComponent } from './shorts-progress.component';

const ICONS = provideLucideIcons(
  LucideCheck, LucideChevronRight, LucideCircleCheck, LucideCircleX, LucideClock, LucideFileText, LucideLoader2, LucideMic,
  LucideMoon, LucidePlay, LucidePlus, LucideScissors, LucideSun, LucideTrash2, LucideUpload, LucideUser, LucideVideo, LucideX,
);

const job = (over: Partial<ShortsJobSummary>): ShortsJobSummary =>
  ({ jobId: 'abcdef1234', status: 'completed', fileType: 'mp4', clipCount: 3, createdAt: '2026-11-01T10:00:00Z', updatedAt: '2026-11-01T10:00:00Z', ...over }) as ShortsJobSummary;

function base(service: Partial<ShortsService>, id = 'abcdef1234') {
  TestBed.configureTestingModule({
    providers: [
      provideHttpClient(),
      provideHttpClientTesting(),
      provideRouter([]),
      ICONS,
      { provide: ShortsService, useValue: service },
      { provide: ActivatedRoute, useValue: { snapshot: { paramMap: convertToParamMap({ id }) } } },
    ],
  });
}

const noHex = (el: HTMLElement) => {
  expect(el.innerHTML).not.toMatch(/#[0-9A-Fa-f]{6}\b/);
  expect(el.innerHTML).not.toContain('uppercase');
  expect(el.innerHTML).not.toContain('var(--primary');
  expect(el.textContent).not.toContain('\u00b7');
};

describe('ShortsUploadComponent', () => {
  function setup() {
    base({ getLimits: () => of(DEFAULT_SHORTS_LIMITS) } as unknown as Partial<ShortsService>);
    const fixture = TestBed.createComponent(ShortsUploadComponent);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('shows the requirements, the drop zone and a disabled Generate until a file is chosen', () => {
    const { el } = setup();
    expect(el.querySelectorAll('h1').length).toBe(1);
    expect(el.querySelector('h1')!.textContent).toContain('Create AI Shorts');
    expect(el.querySelector('[aria-label="Upload requirements"]')!.textContent).toContain('MP4 or MOV');
    expect(el.querySelector('[data-testid="dropzone"]')!.getAttribute('role')).toBe('button');
    expect((el.querySelector('[data-testid="generate"]') as HTMLButtonElement).disabled).toBeTrue();
    expect(el.querySelector('a[href="/shorts/history"]')).toBeTruthy();
    expect(el.querySelector('app-navbar')).toBeTruthy();
    noHex(el);
  });

  it('labels the transcript language select', () => {
    const { el } = setup();
    const select = el.querySelector('#shorts-lang') as HTMLSelectElement;
    expect(el.querySelector('label[for="shorts-lang"]')!.textContent).toContain('Transcript language');
    expect(select.options.length).toBeGreaterThan(1);
  });

  it('shows upload progress as a labelled progress bar with its facts, and the retry note', () => {
    const { fixture, el } = setup();
    const cmp = fixture.componentInstance;
    cmp.uploadProgress = 42;
    cmp.progress = { status: 'retrying', bytesDone: 42, bytesTotal: 100, bytesPerSecond: 10, partsDone: 2, partsTotal: 5, retry: { part: 3, attempt: 2, of: 5 } } as never;
    fixture.detectChanges();
    const bar = el.querySelector('[role="progressbar"]')!;
    expect(bar.getAttribute('aria-valuenow')).toBe('42');
    expect(el.textContent).toContain('2 of 5 parts uploaded');
    expect(el.textContent).toContain('Trying again (attempt 2 of 5)');
    expect(el.textContent).toContain('Retrying');
  });

  it('shows an upload error with a retry button', () => {
    const { fixture, el } = setup();
    fixture.componentInstance.uploadError = 'Upload failed.';
    fixture.detectChanges();
    const alert = el.querySelector('[role="alert"]')!;
    expect(alert.textContent).toContain('Upload failed.');
    expect(alert.querySelector('button')!.textContent).toContain('Try again');
  });
});

describe('ShortsHistoryComponent', () => {
  function setup(list: ShortsJobSummary[] | 'error', del = of(void 0)) {
    const service = {
      listJobs: () => (list === 'error' ? throwError(() => new Error('x')) : of(list)),
      deleteJob: jasmine.createSpy('deleteJob').and.returnValue(del),
      getStatus: () => of({ status: 'uploaded' }),
    };
    base(service as unknown as Partial<ShortsService>);
    const fixture = TestBed.createComponent(ShortsHistoryComponent);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, service };
  }

  it('is titled "Your shorts" and lists jobs newest first with links to clips or progress', () => {
    const { el } = setup([
      job({ jobId: 'old0000001', createdAt: '2026-10-01T00:00:00Z' }),
      job({ jobId: 'new0000001', status: 'rendering', clipCount: 0, createdAt: '2026-11-05T00:00:00Z' }),
    ]);
    expect(el.querySelector('h1')!.textContent!.trim()).toBe('Your shorts');
    const links = Array.from(el.querySelectorAll('li a.open')).map((a) => a.getAttribute('href'));
    expect(links).toEqual(['/shorts/new0000001/progress', '/shorts/old0000001/clips']);
    expect(el.textContent).toContain('Rendering');
    expect(el.textContent).toContain('3 clips');
    noHex(el);
  });

  it('offers Start render for an uploaded job and opens the effects step', () => {
    const { fixture, el } = setup([job({ status: 'uploaded', clipCount: 0 })]);
    const start = el.querySelector('button[aria-label^="Start render"]') as HTMLButtonElement;
    expect(start).toBeTruthy();
    start.click();
    fixture.detectChanges();
    expect(el.querySelector('app-shorts-effects-panel')).toBeTruthy();
  });

  it('asks before deleting, then deletes', () => {
    const { fixture, el, service } = setup([job({})]);
    (el.querySelector('button[aria-label^="Delete short"]') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(fixture.componentInstance.pendingDelete).not.toBeNull();
    expect(fixture.componentInstance.deleteMessage()).toContain('Permanently delete Short abcdef12, MP4, 3 rendered clips');
    fixture.componentInstance.confirmDelete();
    expect(service.deleteJob).toHaveBeenCalledWith('abcdef1234');
    fixture.detectChanges();
    expect(el.querySelectorAll('li.item').length).toBe(0);
  });

  it('shows the empty state and the error state', () => {
    expect(setup([]).el.textContent).toContain('No shorts yet.');
    TestBed.resetTestingModule();
    const { el } = setup('error');
    expect(el.querySelector('[role="alert"]')!.textContent).toContain('Failed to load shorts');
  });
});

describe('ShortsProgressComponent', () => {
  function setup(status: string, extra: Record<string, unknown> = {}) {
    const service = {
      getStatus: () => of({ jobId: 'abcdef1234', status, fileType: 'mp4', progress: 40, sourceDuration: 120, ...extra }),
      cancelJob: jasmine.createSpy('cancelJob').and.returnValue(of({})),
      retryJob: jasmine.createSpy('retryJob').and.returnValue(of({})),
      updateEffects: () => of({}),
    };
    base(service as unknown as Partial<ShortsService>);
    const fixture = TestBed.createComponent(ShortsProgressComponent);
    spyOn(TestBed.inject(Router), 'navigate').and.resolveTo(true);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, service };
  }

  it('marks finished, current and later steps and offers cancel', fakeAsync(() => {
    const { fixture, el, service } = setup('ranking');
    tick();
    fixture.detectChanges();
    expect(el.querySelector('h1')!.textContent!.trim()).toBe('Making your shorts');
    const states = Array.from(el.querySelectorAll('[data-testid^="stage-"]')).map((s) => s.getAttribute('data-state'));
    expect(states).toEqual(['done', 'done', 'active', 'pending']);
    expect(el.querySelector('[data-testid="stage-ranking"]')!.textContent).toContain('40%');
    const cancel = Array.from(el.querySelectorAll('button')).find((b) => b.textContent!.includes('Cancel processing'))!;
    cancel.click();
    expect(service.cancelJob).toHaveBeenCalled();
    noHex(el);
    fixture.destroy();
    discardPeriodicTasks();
  }));

  it('says the shorts are ready when the job completes', fakeAsync(() => {
    const { fixture, el } = setup('completed');
    tick();
    fixture.detectChanges();
    expect(el.textContent).toContain('Your shorts are ready');
    fixture.destroy();
    discardPeriodicTasks();
  }));

  it('shows the failure with its reason, the effects panel and Retry', fakeAsync(() => {
    const { fixture, el, service } = setup('failed', { error: 'Rendering stopped.', errorCode: 'RENDER_FAILED' });
    tick();
    fixture.detectChanges();
    const alert = el.querySelector('[role="alert"]')!;
    expect(alert.textContent).toContain('Processing failed');
    expect(alert.querySelector('app-shorts-effects-panel')).toBeTruthy();
    const retry = Array.from(alert.querySelectorAll('button')).find((b) => b.textContent!.includes('Retry processing'));
    if (retry) {
      retry.click();
      expect(service.retryJob).toHaveBeenCalled();
    } else {
      expect(alert.textContent).toContain("can't be processed as-is");
    }
    fixture.destroy();
    discardPeriodicTasks();
  }));
});
