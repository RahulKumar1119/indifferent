import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { Router, provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { provideLucideIcons, LucideArrowRight, LucideCheck, LucideLoader2, LucideMic, LucideMoon, LucideSun, LucideUser } from '@lucide/angular';
import { CreateProjectComponent } from './create-project.component';
import { ApiService } from '../../core/services/api.service';
import { ShortsService, DEFAULT_SHORTS_LIMITS } from '../shorts/shorts.service';
import { wav } from '../../shared/media-probe.spec';
const fileEvent = (blob: Blob, name: string) => {
  const input = document.createElement('input');
  const dt = new DataTransfer();
  dt.items.add(new File([blob], name, { type: blob.type }));
  input.type = 'file';
  input.files = dt.files;
  return { target: input } as unknown as Event;
};
const until = async (ok: () => boolean) => {
  for (let i = 0; i < 100 && !ok(); i++) await new Promise((r) => setTimeout(r, 20));
};


describe('CreateProjectComponent (#274)', () => {
  function setup() {
    const api = { post: jasmine.createSpy('post').and.returnValue(of({ id: 'p1' })), putAbsolute: () => of(undefined) };
    TestBed.configureTestingModule({
      imports: [CreateProjectComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
        provideNoopAnimations(),
        provideLucideIcons(LucideArrowRight, LucideCheck, LucideLoader2, LucideMic, LucideMoon, LucideSun, LucideUser),
        { provide: ApiService, useValue: api },
        { provide: ShortsService, useValue: { getLimits: () => of(DEFAULT_SHORTS_LIMITS) } },
      ],
    });
    const fixture = TestBed.createComponent(CreateProjectComponent);
    const navigate = spyOn(TestBed.inject(Router), 'navigate').and.resolveTo(true);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, api, navigate, cmp: fixture.componentInstance };
  }

  it('says on step 1 that a TXT file of questions comes next, with the sample, the guide and Story video', () => {
    const { el } = setup();
    const need = el.querySelector('[data-testid="file-needed"]')!;
    expect(need).toBeTruthy();
    expect(need.querySelector('h2')!.textContent).toContain("You'll need a TXT file of questions");
    const sample = need.querySelector('a[download="quiz-sample.txt"]')!;
    expect(sample.getAttribute('href')).toBe('/samples/quiz-sample.txt');
    expect(need.querySelector('a[href="/blog/quiz-file-format-guide"]')).toBeTruthy();
    expect(need.querySelector('a[href="/story"]')!.textContent).toContain('Use Story video');
    // It comes before the form, so it is seen before the long design and voice choices.
    const form = el.querySelector('form')!;
    expect(need.compareDocumentPosition(form) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(need.textContent).not.toContain('\u2014');
  });

  it('goes straight to the upload step after Create, not the project page', () => {
    const { cmp, api, navigate } = setup();
    cmp.projectForm.patchValue({ name: 'Capitals quiz' });
    cmp.createProject();
    expect(api.post).toHaveBeenCalledWith('/projects', jasmine.objectContaining({ name: 'Capitals quiz' }));
    expect(navigate).toHaveBeenCalledWith(['/projects', 'p1', 'upload']);
  });

  it('labels the button with what happens next', () => {
    const { el } = setup();
    const btn = Array.from(el.querySelectorAll('button')).find((b) => b.textContent!.includes('Create and add questions'));
    expect(btn).toBeTruthy();
  });
  it('shows step 1 of 2 with aria-current, and no Material stepper', () => {
    const { el } = setup();
    expect(el.querySelector('mat-stepper, .mat-stepper-horizontal')).toBeNull();
    const now = el.querySelector('app-step-indicator [aria-current="step"]')!;
    expect(now.textContent).toContain('Details');
  });

  it('keeps the optional settings in a closed "More options" panel that says it uses the defaults', () => {
    const { el } = setup();
    const more = el.querySelector('[data-testid="more-options"]') as HTMLDetailsElement;
    expect(more.tagName).toBe('DETAILS');
    expect(more.open).toBeFalse();
    expect(more.querySelector('summary')!.textContent).toContain('More options');
    expect(el.querySelector('[data-testid="more-state"]')!.textContent).toContain('Using the defaults');
    for (const name of ['textSize', 'accent', 'answerLabels', 'showQuestionNumber', 'watermarkText', 'channelName', 'includeShorts']) {
      expect(more.querySelector(`[formcontrolname="${name}"]`)).withContext(name).toBeTruthy();
    }
    // Design and voice stay in view: they are not optional.
    expect(more.querySelector('[formcontrolname="template"]')).toBeNull();
    expect(more.querySelector('[formcontrolname="voice"]')).toBeNull();
  });

  it('keeps choices made in the panel after it closes, says what changed, and sends them', () => {
    const { fixture, el, cmp, api } = setup();
    const more = el.querySelector('[data-testid="more-options"]') as HTMLDetailsElement;
    more.open = true;
    more.dispatchEvent(new Event('toggle'));
    cmp.projectForm.patchValue({ name: 'Q', watermarkText: 'Mine', channelName: '@me', designOptions: { textSize: 'l' } });
    more.open = false;
    more.dispatchEvent(new Event('toggle'));
    fixture.detectChanges();
    expect(cmp.moreOpen).toBeFalse();
    expect(el.querySelector('[data-testid="more-state"]')!.textContent).toContain('Changed: Design options, Watermark, Branding');
    cmp.createProject();
    expect(api.post).toHaveBeenCalledWith('/projects', jasmine.objectContaining({
      designOptions: jasmine.objectContaining({ textSize: 'l' }),
      watermark: { text: 'Mine', opacity: 0.8 },
      branding: { channelName: '@me' },
    }));
  });

  it('sends the defaults when the panel is never opened', () => {
    const { cmp, api } = setup();
    cmp.projectForm.patchValue({ name: 'Q' });
    cmp.createProject();
    const body = api.post.calls.mostRecent().args[1];
    expect(body.designOptions).toEqual({ accent: jasmine.any(String), textSize: 'm', showQuestionNumber: true, answerLabels: 'letters' });
    expect(body.voice).toBe('Joanna');
    expect(body.watermark).toBeUndefined();
    expect(body.branding).toBeUndefined();
    expect(body.shorts).toBeUndefined();
  });

  it('opens the panel and says why when AI Shorts is on without a file', () => {
    const { fixture, el, cmp, api } = setup();
    cmp.projectForm.patchValue({ name: 'Q', includeShorts: true });
    cmp.createProject();
    fixture.detectChanges();
    expect(api.post).not.toHaveBeenCalled();
    expect((el.querySelector('[data-testid="more-options"]') as HTMLDetailsElement).open).toBeTrue();
    expect(el.querySelector('[role="alert"]')!.textContent).toContain('Choose a video or audio file for AI Shorts');
  });

  it('reads the duration of a picked Shorts file through the shared probe, and refuses an unreadable one', async () => {
    const { cmp } = setup();
    cmp.onShortsFileSelected(fileEvent(wav(30), 'talk.wav'));
    expect(cmp.shortsProbing).toBeTrue();
    await until(() => !cmp.shortsProbing);
    expect(cmp.shortsDuration).toBeCloseTo(30, 1);
    expect(cmp.shortsError).toBe('');
    cmp.onShortsFileSelected(fileEvent(new Blob([new Uint8Array(16)], { type: 'video/mp4' }), 'broken.mp4'));
    await until(() => !cmp.shortsProbing);
    expect(cmp.shortsError).toBe('Could not read this media file.');
    expect(cmp.shortsFile).toBeNull();
  });

  // Issue #286: the quiz page's Shorts option refuses a too-short file too.
  it('refuses a Shorts file under the minimum before upload, with the server minimum or 30 seconds', async () => {
    const { cmp } = setup();
    cmp.onShortsFileSelected(fileEvent(wav(10), 'short.wav'));
    await until(() => !cmp.shortsProbing);
    expect(cmp.shortsError).toBe('This file is 10 seconds long. AI Shorts needs at least 30 seconds to cut a clip from.');
    expect(cmp.shortsFile).toBeNull();
    expect(cmp.shortsMinSeconds).toBe(30);

    cmp.shortsLimits = { ...cmp.shortsLimits, minDurationSeconds: 45 };
    cmp.onShortsFileSelected(fileEvent(wav(30), 'talk.wav'));
    await until(() => !cmp.shortsProbing);
    expect(cmp.shortsError).toContain('at least 45 seconds');
    expect(cmp.shortsFile).toBeNull();
  });
});
