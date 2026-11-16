import { HttpErrorResponse } from '@angular/common/http';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Subject, of, throwError } from 'rxjs';
import {
  provideLucideIcons,
  LucideCheck,
  LucideClock,
  LucideCopy,
  LucideLoader2,
  LucideSparkles,
  LucideTriangleAlert,
} from '@lucide/angular';
import { MIN_SOURCE_CHARS, ThumbnailCopyComponent } from './thumbnail-copy.component';
import { CopyResult, CopyStatus, VideoCopyService } from './video-copy.service';

const ICONS = provideLucideIcons(LucideCheck, LucideClock, LucideCopy, LucideLoader2, LucideSparkles, LucideTriangleAlert);

const STATUS: CopyStatus = {
  enabled: true,
  transcriptAvailable: true,
  remainingToday: 20,
  dailyLimit: 20,
  maxTopicChars: 300,
  maxTranscriptChars: 20000,
  maxRefineChars: 200,
};

function result(over: Partial<CopyResult> = {}): CopyResult {
  return {
    titles: Array.from({ length: 10 }, (_, i) => `Title idea number ${i + 1}`),
    description: 'A keeper climbs the stairs every night.\n\nThe video follows one winter.',
    chapters: [
      { start: 0, time: '0:00', title: 'Opening' },
      { start: 60, time: '1:00', title: 'The stairs' },
      { start: 120, time: '2:00', title: 'The storm' },
    ],
    descriptionWithChapters: 'A keeper climbs the stairs every night.\n\nThe video follows one winter.\n\n0:00 Opening\n1:00 The stairs\n2:00 The storm',
    hasTimestamps: true,
    notes: [],
    remainingToday: 19,
    dailyLimit: 20,
    ...over,
  };
}

function failure(status: number, code = '', message = '', details?: unknown): HttpErrorResponse {
  return new HttpErrorResponse({ status, error: status === 0 ? new ProgressEvent('error') : { code, message, ...(details ? { details } : {}) } });
}

describe('ThumbnailCopyComponent', () => {
  let service: jasmine.SpyObj<VideoCopyService>;
  let fixture: ComponentFixture<ThumbnailCopyComponent>;
  let el: HTMLElement;

  async function create(status: CopyStatus | HttpErrorResponse = STATUS): Promise<void> {
    service.status.and.returnValue(status instanceof HttpErrorResponse ? throwError(() => status) : of(status));
    await TestBed.configureTestingModule({
      imports: [ThumbnailCopyComponent],
      providers: [ICONS, { provide: VideoCopyService, useValue: service }],
    }).compileComponents();
    fixture = TestBed.createComponent(ThumbnailCopyComponent);
    fixture.componentRef.setInput('jobId', 'j1');
    el = fixture.nativeElement;
    await settle();
  }
  const settle = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  const q = <T extends HTMLElement>(testId: string) => el.querySelector<T>(`[data-testid="${testId}"]`)!;
  const text = () => el.textContent!.replace(/\s+/g, ' ');
  const writeButton = () => q<HTMLButtonElement>('copy-write');
  async function typeTopic(value: string): Promise<void> {
    const box = q<HTMLTextAreaElement>('copy-topic');
    box.value = value;
    box.dispatchEvent(new Event('input'));
    await settle();
  }
  async function chooseSource(value: string): Promise<void> {
    const radio = el.querySelector<HTMLInputElement>(`input[name="copy-source"][value="${value}"]`)!;
    radio.click();
    await settle();
  }
  async function writeFromJob(res: CopyResult = result()): Promise<void> {
    service.write.and.returnValue(of(res));
    writeButton().click();
    await settle();
  }

  /**
   * Lets the clipboard promise finish, then redraws. It does not wait for the page to
   * go quiet: that would wait out the two second "Copied" mark and see it gone.
   */
  const copied = async () => {
    await new Promise((r) => setTimeout(r));
    fixture.detectChanges();
  };

  beforeEach(() => {
    service = jasmine.createSpyObj<VideoCopyService>('VideoCopyService', ['status', 'write']);
  });

  describe('whether it shows', () => {
    it('shows nothing while the server has the helper off', async () => {
      await create({ ...STATUS, enabled: false });
      expect(el.querySelector('section')).toBeNull();
      expect(text().trim()).toBe('');
    });

    it('stays out of the way when it cannot find out', async () => {
      await create(failure(500));
      expect(el.querySelector('section')).toBeNull();
    });

    it('asks for the status of the job it was given', async () => {
      await create();
      expect(service.status).toHaveBeenCalledOnceWith('j1');
      expect(el.querySelector('h2')!.textContent).toContain('Title and description');
    });
  });

  describe('choosing what to write from', () => {
    it('starts on the video\'s own transcript when it has one', async () => {
      await create();
      expect(el.querySelector<HTMLInputElement>('input[value="job"]')!.checked).toBeTrue();
      expect(writeButton().disabled).toBeFalse();
      expect(el.querySelector('[data-testid="copy-topic"]')).toBeNull();
    });

    it('starts on a sentence, with the transcript choice switched off and explained, when there is none', async () => {
      await create({ ...STATUS, transcriptAvailable: false });
      const job = el.querySelector<HTMLInputElement>('input[value="job"]')!;
      expect(job.disabled).toBeTrue();
      expect(el.querySelector<HTMLInputElement>('input[value="topic"]')!.checked).toBeTrue();
      expect(text()).toContain('This video has no transcript yet.');
      expect(writeButton().disabled).toBeTrue();
    });

    it('uses a native radio group with a legend, so arrow keys and screen readers work', async () => {
      await create();
      const group = el.querySelector('fieldset')!;
      expect(group.querySelector('legend')!.textContent).toContain('Write from');
      const radios = group.querySelectorAll<HTMLInputElement>('input[type="radio"]');
      expect(radios.length).toBe(3);
      expect(new Set(Array.from(radios).map((r) => r.name)).size).toBe(1);
      for (const r of Array.from(radios)) expect(r.closest('label')!.textContent!.trim().length).toBeGreaterThan(10);
    });

    it('shows a box for a pasted transcript, with its own limit and a count', async () => {
      await create();
      await chooseSource('paste');
      const box = q<HTMLTextAreaElement>('copy-paste');
      expect(box.getAttribute('maxlength')).toBe('20000');
      expect(el.querySelector('label[for="copy-paste"]')).not.toBeNull();
      expect(text()).toContain('0 of 20000 characters');
      expect(text()).toContain('Start lines with a time');
    });
  });

  describe('writing', () => {
    it('needs enough text from a sentence or a pasted transcript, counting characters as a person sees them', async () => {
      await create({ ...STATUS, transcriptAvailable: false });
      expect(writeButton().disabled).toBeTrue();
      await typeTopic('a'.repeat(MIN_SOURCE_CHARS - 1));
      expect(writeButton().disabled).toBeTrue();
      await typeTopic('a'.repeat(MIN_SOURCE_CHARS));
      expect(writeButton().disabled).toBeFalse();
      await typeTopic('   ' + 'ab'.repeat(5) + '   ');
      expect(writeButton().disabled).toBeTrue();
      expect(q('copy-topic').getAttribute('maxlength')).toBe('300');
      await chooseSource('paste');
      expect(writeButton().disabled).toBeTrue();
      const box = q<HTMLTextAreaElement>('copy-paste');
      box.value = '0:00 Intro to the lighthouse\n1:00 The keeper wakes\n2:00 The stairs';
      box.dispatchEvent(new Event('input'));
      await settle();
      expect(writeButton().disabled).toBeFalse();
    });

    it('sends the video\'s transcript request', async () => {
      await create();
      await writeFromJob();
      expect(service.write).toHaveBeenCalledOnceWith('j1', { useJobTranscript: true });
    });

    it('sends a pasted transcript or a sentence, as typed', async () => {
      await create();
      await chooseSource('topic');
      await typeTopic('A video about a lighthouse keeper');
      service.write.and.returnValue(of(result()));
      writeButton().click();
      await settle();
      expect(service.write.calls.mostRecent().args[1]).toEqual({ topic: 'A video about a lighthouse keeper' });
      await chooseSource('paste');
      const box = q<HTMLTextAreaElement>('copy-paste');
      box.value = '0:00 One two three\n1:00 Four five six\n2:00 Seven eight nine';
      box.dispatchEvent(new Event('input'));
      await settle();
      writeButton().click();
      await settle();
      expect(service.write.calls.mostRecent().args[1]).toEqual({ transcript: '0:00 One two three\n1:00 Four five six\n2:00 Seven eight nine' });
    });

    it('says it is working, locks the form, and does not send twice', async () => {
      await create();
      const pending = new Subject<CopyResult>();
      service.write.and.returnValue(pending);
      writeButton().click();
      await settle();
      expect(writeButton().getAttribute('aria-busy')).toBe('true');
      expect(writeButton().disabled).toBeTrue();
      expect(writeButton().textContent).toContain('Writing');
      expect(q('copy-status').textContent).toContain('Writing the titles and description.');
      el.querySelectorAll<HTMLInputElement>('input[type="radio"]').forEach((r) => expect(r.disabled).toBeTrue());
      writeButton().click();
      expect(service.write).toHaveBeenCalledTimes(1);
      pending.next(result());
      pending.complete();
      await settle();
      expect(writeButton().disabled).toBeFalse();
    });

    it('stops waiting for an answer when the page goes away', async () => {
      await create();
      const pending = new Subject<CopyResult>();
      service.write.and.returnValue(pending);
      writeButton().click();
      await settle();
      expect(pending.observed).toBeTrue();
      fixture.destroy();
      expect(pending.observed).toBeFalse();
    });
  });

  describe('the result', () => {
    it('shows ten titles with their lengths, the description, and the chapters', async () => {
      await create();
      await writeFromJob();
      expect(el.querySelectorAll('ol.titles li').length).toBe(10);
      expect(q('copy-title-3').textContent).toBe('Title idea number 3');
      expect(el.querySelector('ol.titles li:nth-child(3)')!.textContent).toContain('19 of 100 characters');
      expect(q('copy-description').textContent).toContain('A keeper climbs the stairs every night.');
      const chapters = el.querySelectorAll('[data-testid="copy-chapters"] li');
      expect(chapters.length).toBe(3);
      expect(chapters[1].textContent!.replace(/\s+/g, ' ').trim()).toBe('1:00The stairs');
      expect(q('copy-status').textContent).toContain('10 titles, a description and 3 chapters are ready.');
    });

    it('keeps the description on its own lines, and shows the text, never as markup', async () => {
      await create();
      await writeFromJob(result({ titles: ['<img src=x onerror=alert(1)> Free', ...result().titles.slice(1)] }));
      expect(el.querySelector('img')).toBeNull();
      expect(q('copy-title-1').textContent).toBe('<img src=x onerror=alert(1)> Free');
      expect(getComputedStyle(q('copy-description')).whiteSpace).toBe('pre-wrap');
    });

    it('offers copy-with-chapters only when there are chapters, and shows why there are none', async () => {
      await create();
      await writeFromJob(result({ chapters: [], descriptionWithChapters: result().description, hasTimestamps: false, notes: ['There are no timestamps, so there are no chapters.'] }));
      expect(el.querySelector('[data-testid="copy-chapters"]')).toBeNull();
      expect(el.querySelector('[data-testid="copy-full-btn"]')).toBeNull();
      expect(q('copy-note').textContent).toContain('no timestamps');
      expect(q('copy-note').getAttribute('role')).toBe('status');
    });

    it('moves focus to the titles heading so a screen reader starts at the result', async () => {
      await create();
      await writeFromJob();
      await settle();
      expect(document.activeElement!.id).toBe('copy-titles-h');
    });

    it('shows the tries left, from the status first and from each answer after', async () => {
      await create();
      expect(q('copy-left').textContent).toContain('20 of 20 tries left today');
      await writeFromJob(result({ remainingToday: 19 }));
      expect(q('copy-left').textContent).toContain('19 of 20 tries left today');
      expect(q('copy-left').textContent).toContain('Writing new titles uses one too.');
    });

    it('says nothing about tries when the server could not count them', async () => {
      await create({ ...STATUS, remainingToday: undefined });
      expect(el.querySelector('[data-testid="copy-left"]')).toBeNull();
    });

    it('writing again replaces everything', async () => {
      await create();
      await writeFromJob();
      expect(writeButton().textContent).toContain('Write again');
      await writeFromJob(result({ titles: Array.from({ length: 10 }, (_, i) => `Fresh ${i + 1}`), description: 'A new description here.' }));
      expect(q('copy-title-1').textContent).toBe('Fresh 1');
      expect(q('copy-description').textContent).toBe('A new description here.');
    });
  });

  describe('copying', () => {
    afterEach(() => fixture?.destroy());
    let write: jasmine.Spy;
    beforeEach(() => {
      write = spyOn(navigator.clipboard, 'writeText').and.returnValue(Promise.resolve());
    });

    it('copies one title, marks it, and announces it', async () => {
      await create();
      await writeFromJob();
      q<HTMLButtonElement>('copy-title-btn-3').click();
      await copied();
      expect(write).toHaveBeenCalledOnceWith('Title idea number 3');
      expect(q('copy-title-btn-3').textContent).toContain('Copied');
      expect(q('copy-title-btn-4').textContent).toContain('Copy');
      expect(q('copy-title-btn-4').textContent).not.toContain('Copied');
      expect(q('copy-status').textContent).toContain('Title 3 copied.');
    });

    it('takes the Copied mark off by itself after a moment', async () => {
      jasmine.clock().install();
      try {
        await create();
        await writeFromJob();
        q<HTMLButtonElement>('copy-title-btn-2').click();
        await Promise.resolve();
        await Promise.resolve();
        fixture.detectChanges();
        expect(q('copy-title-btn-2').textContent).toContain('Copied');
        jasmine.clock().tick(2100);
        fixture.detectChanges();
        expect(q('copy-title-btn-2').textContent).not.toContain('Copied');
      } finally {
        jasmine.clock().uninstall();
      }
    });

    it('names each copy button for the title it copies', async () => {
      await create();
      await writeFromJob();
      expect(q('copy-title-btn-7').textContent!.replace(/\s+/g, ' ').trim()).toBe('Copy title 7');
    });

    it('copies the description, and the description with its chapters', async () => {
      await create();
      await writeFromJob();
      q<HTMLButtonElement>('copy-description-btn').click();
      await copied();
      expect(write.calls.mostRecent().args[0]).toBe(result().description);
      q<HTMLButtonElement>('copy-full-btn').click();
      await copied();
      expect(write.calls.mostRecent().args[0]).toBe(result().descriptionWithChapters);
      expect(q('copy-full-btn').textContent).toContain('Copied');
      expect(q('copy-description-btn').textContent).not.toContain('Copied');
    });

    it('says so when the browser will not let it copy, and tells the person what to do', async () => {
      write.and.returnValue(Promise.reject(new DOMException('no', 'NotAllowedError')));
      spyOn(document, 'execCommand').and.returnValue(false);
      await create();
      await writeFromJob();
      q<HTMLButtonElement>('copy-title-btn-1').click();
      await copied();
      expect(q('copy-problem').textContent).toContain("Couldn't copy");
      expect(q('copy-problem').textContent).toContain('Select the text');
      expect(q('copy-title-btn-1').textContent).not.toContain('Copied');
    });
  });

  describe('asking for different titles', () => {
    it('sends the instruction and the titles it is changing, and keeps the description and chapters', async () => {
      await create();
      await writeFromJob();
      const first = result();
      const box = q<HTMLTextAreaElement>('copy-refine');
      expect(box.getAttribute('maxlength')).toBe('200');
      box.value = 'shorter';
      box.dispatchEvent(new Event('input'));
      await settle();
      const fresh = Array.from({ length: 10 }, (_, i) => `Different ${i + 1}`);
      service.write.and.returnValue(of(result({ titles: fresh, description: 'IGNORED', chapters: [], remainingToday: 18 })));
      q<HTMLButtonElement>('copy-refine-go').click();
      await settle();
      expect(service.write.calls.mostRecent().args[1]).toEqual({ useJobTranscript: true, refine: 'shorter', previousTitles: first.titles });
      expect(q('copy-title-1').textContent).toBe('Different 1');
      expect(q('copy-description').textContent).toBe(first.description);
      expect(el.querySelectorAll('[data-testid="copy-chapters"] li').length).toBe(3);
      expect(q('copy-left').textContent).toContain('18 of 20 tries left today');
      expect(q<HTMLTextAreaElement>('copy-refine').value).toBe('');
      expect(q('copy-status').textContent).toContain('10 new titles are ready.');
    });

    it('works with no instruction at all', async () => {
      await create();
      await writeFromJob();
      service.write.and.returnValue(of(result()));
      q<HTMLButtonElement>('copy-refine-go').click();
      await settle();
      expect(service.write.calls.mostRecent().args[1]).toEqual({ useJobTranscript: true, refine: '', previousTitles: result().titles });
    });

    it('keeps the old titles when it fails', async () => {
      await create();
      await writeFromJob();
      service.write.and.returnValue(throwError(() => failure(503, 'COPY_UNAVAILABLE')));
      q<HTMLButtonElement>('copy-refine-go').click();
      await settle();
      expect(q('copy-title-1').textContent).toBe('Title idea number 1');
      expect(q('copy-problem').getAttribute('data-kind')).toBe('unavailable');
    });
  });

  describe('when it goes wrong', () => {
    const cases: [string, HttpErrorResponse, string, string][] = [
      ['too thin', failure(400, 'VALIDATION_ERROR', 'give a topic or a transcript of at least 15 characters'), 'invalid', 'alert'],
      ['busy', failure(503, 'COPY_UNAVAILABLE'), 'unavailable', 'alert'],
      ['slow', failure(504, 'COPY_TIMEOUT'), 'timeout', 'alert'],
      ['unusable', failure(502, 'COPY_FAILED'), 'failed', 'alert'],
      ['offline', failure(0), 'offline', 'alert'],
      ['over the limit', failure(429, 'QUOTA_EXCEEDED', '', { retryAfter: 3600 }), 'rate_limited', 'status'],
      ['switched off meanwhile', failure(503, 'UNAVAILABLE'), 'disabled', 'status'],
    ];
    for (const [name, err, kind, role] of cases) {
      it(`${name}: shows it as ${role}, and keeps what the person typed`, async () => {
        await create({ ...STATUS, transcriptAvailable: false });
        await typeTopic('A video about a lighthouse keeper');
        service.write.and.returnValue(throwError(() => err));
        writeButton().click();
        await settle();
        const box = q('copy-problem');
        expect(box.getAttribute('data-kind')).toBe(kind);
        expect(box.getAttribute('role')).toBe(role);
        expect(q<HTMLTextAreaElement>('copy-topic').value).toBe('A video about a lighthouse keeper');
        expect(writeButton().disabled).toBeFalse();
        expect(el.querySelector('[data-testid="copy-result"]')).toBeNull();
        expect(q('copy-status').textContent!.trim()).toBe('');
      });
    }

    it('keeps the earlier result on screen when a later try fails, and clears the message on the next try', async () => {
      await create();
      await writeFromJob();
      service.write.and.returnValue(throwError(() => failure(504, 'COPY_TIMEOUT')));
      writeButton().click();
      await settle();
      expect(q('copy-problem')).not.toBeNull();
      expect(q('copy-title-1').textContent).toBe('Title idea number 1');
      service.write.and.returnValue(of(result()));
      writeButton().click();
      await settle();
      expect(el.querySelector('[data-testid="copy-problem"]')).toBeNull();
    });

    it('shows the limit as used up, so the note stops saying there are tries left', async () => {
      await create();
      service.write.and.returnValue(throwError(() => failure(429, 'QUOTA_EXCEEDED', '', { retryAfter: 3600 })));
      writeButton().click();
      await settle();
      expect(q('copy-left').textContent).toContain('0 of 20 tries left today');
      expect(q('copy-problem').textContent).toContain('in 1 hour');
    });
  });

  it('uses no em dashes or emoji, and no hard-coded colours, in anything it shows', async () => {
    await create();
    await writeFromJob(result({ notes: ['A note.'] }));
    expect(el.textContent).not.toMatch(/\u2014|\p{Extended_Pictographic}/u);
    expect(el.innerHTML).not.toMatch(/#[0-9A-Fa-f]{6}\b/);
  });
});
