import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { Router, provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { provideLucideIcons, LucideArrowRight, LucideCheck, LucideLoader2, LucideMic } from '@lucide/angular';
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
        provideLucideIcons(LucideArrowRight, LucideCheck, LucideLoader2, LucideMic),
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
  it('reads the duration of a picked Shorts file through the shared probe, and refuses an unreadable one', async () => {
    const { cmp } = setup();
    cmp.onShortsFileSelected(fileEvent(wav(1.5), 'talk.wav'));
    expect(cmp.shortsProbing).toBeTrue();
    await until(() => !cmp.shortsProbing);
    expect(cmp.shortsDuration).toBeCloseTo(1.5, 1);
    expect(cmp.shortsError).toBe('');
    cmp.onShortsFileSelected(fileEvent(new Blob([new Uint8Array(16)], { type: 'video/mp4' }), 'broken.mp4'));
    await until(() => !cmp.shortsProbing);
    expect(cmp.shortsError).toBe('Could not read this media file.');
    expect(cmp.shortsFile).toBeNull();
  });
});
