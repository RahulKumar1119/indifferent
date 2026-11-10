import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { LucideCheck, LucideCircleX, LucideFileText, LucideLoader2, LucideMoon, LucidePlay, LucideSun, LucideUpload, LucideUser, LucideX, provideLucideIcons } from '@lucide/angular';
import { ApiService } from '../../core';
import { UploadComponent } from './upload.component';

describe('UploadComponent: the format hint', () => {
  function open(): HTMLElement {
    return setup().el;
  }
  function setup() {
    TestBed.configureTestingModule({
      imports: [UploadComponent],
      providers: [
        provideRouter([]),
        provideLucideIcons(LucideUpload, LucideFileText, LucideCircleX, LucideX, LucidePlay, LucideCheck, LucideLoader2, LucideSun, LucideMoon, LucideUser),
        { provide: ActivatedRoute, useValue: { snapshot: { paramMap: convertToParamMap({ id: 'p1' }) } } },
        { provide: ApiService, useValue: {} },
      ],
    });
    const fixture = TestBed.createComponent(UploadComponent);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, cmp: fixture.componentInstance };
  }

  it('shows a working example before the drop zone, so people see the format before they upload', () => {
    const el = open();
    const hint = el.querySelector('section[aria-labelledby="format-hint-title"]')!;
    expect(hint.querySelector('h2')!.textContent).toContain('What a working file looks like');
    const example = hint.querySelector('pre')!.textContent!;
    expect(example).toMatch(/^1\. .+\?/);
    expect(example).toContain('A. London');
    expect(example).toContain('Answer: B');
    const zone = el.querySelector('[role="button"][aria-label^="Drop zone"]')!;
    expect(hint.compareDocumentPosition(zone) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('links the format guide and a sample file that downloads', () => {
    const hint = open().querySelector('section[aria-labelledby="format-hint-title"]')!;
    expect(hint.querySelector('a[href="/blog/quiz-file-format-guide"]')).not.toBeNull();
    const sample = hint.querySelector('a[href="/samples/quiz-sample.txt"]')!;
    expect(sample.getAttribute('download')).toBe('quiz-sample.txt');
  });

  it('lets a keyboard user scroll the example', () => {
    expect(open().querySelector('section pre')!.getAttribute('tabindex')).toBe('0');
  });
  it('points people with a story to Story video (#274)', () => {
    const el = open();
    expect(el.querySelector('[data-testid="story-alt"] a[href="/story"]')!.textContent).toContain('Use Story video');
  });

  it('is step 2 of 2, Questions (#251)', () => {
    const now = open().querySelector('app-step-indicator [aria-current="step"]')!;
    expect(now.textContent).toContain('Questions');
    expect(now.textContent).toContain('Step 2 of 2');
  });

  it('keeps the drop zone a single control with no button nested inside it (#251)', () => {
    const zone = open().querySelector('[role="button"][aria-label^="Drop zone"]')!;
    expect(zone.getAttribute('tabindex')).toBe('0');
    expect(zone.querySelector('button, a, input, [tabindex="0"]')).toBeNull();
    expect(zone.textContent).toContain('Browse files');
  });

  it('enables "Start processing" once a valid file is picked, and shows the file with Remove (#251)', async () => {
    const { fixture, el, cmp } = setup();
    const start = () => Array.from(el.querySelectorAll('button')).find((b) => b.textContent!.includes('Start processing')) as HTMLButtonElement;
    expect(start().disabled).toBeTrue();
    const input = document.createElement('input');
    const dt = new DataTransfer();
    dt.items.add(new File(['1. Q?\nA. a\nB. b\nAnswer: A\n'], 'quiz.txt', { type: 'text/plain' }));
    input.type = 'file';
    input.files = dt.files;
    cmp.onFileSelected({ target: input } as unknown as Event);
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="file-name"]')!.textContent).toBe('quiz.txt');
    expect(Array.from(el.querySelectorAll('button')).some((b) => b.textContent!.trim() === 'Remove')).toBeTrue();
    expect(start().disabled).toBeFalse();
    expect(el.querySelector('[role="button"][aria-label^="Drop zone"]')).toBeNull();
  });

  it('refuses a file that is not TXT and says why', () => {
    const { fixture, el, cmp } = setup();
    const input = document.createElement('input');
    const dt = new DataTransfer();
    dt.items.add(new File(['x'], 'quiz.pdf', { type: 'application/pdf' }));
    input.type = 'file';
    input.files = dt.files;
    cmp.onFileSelected({ target: input } as unknown as Event);
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')!.textContent).toContain('Invalid file type');
  });
});
