import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router, convertToParamMap, provideRouter } from '@angular/router';
import { LucideCheck, LucideCircleCheck, LucideCircleX, LucideLoader2, LucideX, provideLucideIcons } from '@lucide/angular';
import { of } from 'rxjs';
import { ProjectService } from '../../core';
import { PipelineProgress } from '../../shared';
import { ProgressComponent } from './progress.component';

function open(progress: PipelineProgress) {
  TestBed.configureTestingModule({
    imports: [ProgressComponent],
    providers: [
      provideRouter([]),
      provideLucideIcons(LucideCheck, LucideCircleCheck, LucideCircleX, LucideLoader2, LucideX),
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
    expect(alert.textContent).toContain('Processing Failed');
    expect(alert.textContent).toContain('Something went wrong while processing your video');
    expect(alert.querySelector('button')!.textContent!.trim()).toBe('Retry upload');
    expect(alert.querySelector('a')).toBeNull();
  });

  it('shows no failure panel while the video is still being made', () => {
    const { el } = open({ stage: 'rendering', percentage: 75 });
    expect(el.querySelector('[role="alert"]')).toBeNull();
  });
});
