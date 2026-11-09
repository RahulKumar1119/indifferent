import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { LucideCircleX, LucideFileText, LucidePlay, LucideUpload, LucideX, provideLucideIcons } from '@lucide/angular';
import { ApiService } from '../../core';
import { UploadComponent } from './upload.component';

describe('UploadComponent: the format hint', () => {
  function open() {
    TestBed.configureTestingModule({
      imports: [UploadComponent],
      providers: [
        provideRouter([]),
        provideLucideIcons(LucideUpload, LucideFileText, LucideCircleX, LucideX, LucidePlay),
        { provide: ActivatedRoute, useValue: { snapshot: { paramMap: convertToParamMap({ id: 'p1' }) } } },
        { provide: ApiService, useValue: {} },
      ],
    });
    const fixture = TestBed.createComponent(UploadComponent);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
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
});
