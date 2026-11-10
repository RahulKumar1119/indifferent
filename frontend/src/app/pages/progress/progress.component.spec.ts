import { TestBed, fakeAsync, tick } from '@angular/core/testing';
import { ActivatedRoute, Router, convertToParamMap, provideRouter } from '@angular/router';
import { LucideCheck, LucideCircleCheck, LucideCircleX, LucideLoader2, LucideMoon, LucideSun, LucideUser, LucideX, provideLucideIcons } from '@lucide/angular';
import { of } from 'rxjs';
import { ProjectService } from '../../core';
import { PipelineProgress } from '../../shared';
import { ProgressComponent } from './progress.component';

function open(progress: PipelineProgress) {
  TestBed.configureTestingModule({
    imports: [ProgressComponent],
    providers: [
      provideRouter([]),
      provideLucideIcons(LucideCheck, LucideCircleCheck, LucideCircleX, LucideLoader2, LucideX, LucideSun, LucideMoon, LucideUser),
      { provide: ActivatedRoute, useValue: { snapshot: { paramMap: convertToParamMap({ id: 'p1' }) } } },
      { provide: ProjectService, useValue: { pollProjectStatus: () => of(progress) } },
    ],
  });
  const navigate = spyOn(TestBed.inject(Router), 'navigate').and.resolveTo(true);
  const fixture = TestBed.createComponent(ProgressComponent);
  fixture.detectChanges();
  return { fixture, navigate, el: fixture.nativeElement as HTMLElement };
}

describe('ProgressComponent: a failed project', () => {
  it('says the file had no questions, announces it, and offers another file, the guide and a sample', () => {
    const { el, navigate } = open({ stage: 'failed', percentage: 0, failureCode: 'NO_QUESTION_FORMAT' });
    const alert = el.querySelector('[role="alert"]')!;
    expect(alert.textContent).toContain('We could not find any questions in your file');
    expect(alert.textContent).toContain('lettered options');
    const button = alert.querySelector('button')!;
    expect(button.textContent!.trim()).toBe('Choose another file');
    button.click();
    expect(navigate).toHaveBeenCalledWith(['/projects', 'p1', 'upload']);
    expect(alert.querySelector('a[href="/blog/quiz-file-format-guide"]')).not.toBeNull();
    const sample = alert.querySelector('a[href="/samples/quiz-sample.txt"]')!;
    expect(sample.hasAttribute('download')).toBeTrue();
    // A file with no questions may be a story: point to Story video (#274).
    expect(alert.querySelector('[data-testid="story-hint"] a[href="/story"]')!.textContent).toContain('Story video');
  });

  it('keeps the generic message and Retry upload for a failure the file did not cause', () => {
    const { el } = open({ stage: 'failed', percentage: 0 });
    const alert = el.querySelector('[role="alert"]')!;
    expect(alert.textContent).toContain('Processing failed');
    expect(alert.textContent).toContain('Something went wrong while processing your video');
    expect(alert.querySelector('button')!.textContent!.trim()).toBe('Retry upload');
    expect(alert.querySelector('a')).toBeNull();
  });

  it('shows no failure panel while the video is still being made', () => {
    const { el } = open({ stage: 'rendering', percentage: 75 });
    expect(el.querySelector('[role="alert"]')).toBeNull();
  });
});

describe('ProgressComponent: the heading follows the state (#251)', () => {
  const heading = (el: HTMLElement) => el.querySelector('[data-testid="heading"]')!.textContent!.trim();

  it('says "Making your video" while it is being made, marks the running step and announces it', () => {
    const { el } = open({ stage: 'narrating', percentage: 40 });
    expect(heading(el)).toBe('Making your video');
    const now = el.querySelectorAll('[aria-current="step"]');
    expect(now.length).toBe(1);
    expect(now[0].getAttribute('data-testid')).toBe('stage-narrating');
    expect(el.querySelector('[data-testid="stage-parsing"]')!.textContent).toContain('done');
    expect(el.querySelector('[data-testid="live"]')!.textContent).toContain('Narrating, in progress.');
  });

  it('shows the render percentage as a progressbar while rendering', () => {
    const { el } = open({ stage: 'rendering', percentage: 75 });
    expect(el.querySelector('[role="progressbar"]')!.getAttribute('aria-valuenow')).toBe('75');
  });

  it('says "Video ready" when it is done, links the preview and still redirects', fakeAsync(() => {
    const { el, navigate } = open({ stage: 'completed', percentage: 100 });
    expect(heading(el)).toBe('Video ready');
    expect(el.querySelector('[data-testid="ready"] a')!.getAttribute('href')).toBe('/projects/p1/preview');
    tick(2000);
    expect(navigate).toHaveBeenCalledWith(['/projects', 'p1', 'preview']);
  }));

  it('says "Something went wrong" on a failure, never "Making your video"', () => {
    for (const code of [undefined, 'NO_QUESTION_FORMAT']) {
      TestBed.resetTestingModule();
      const { el } = open({ stage: 'failed', percentage: 0, failureCode: code } as PipelineProgress);
      expect(heading(el)).withContext(String(code)).toBe('Something went wrong');
      expect(el.textContent).not.toContain('Making your video');
      expect(el.querySelector('[data-testid="stage-parsing"].failed')).withContext(String(code)).toBeTruthy();
    }
  });
});
