import { HttpErrorResponse } from '@angular/common/http';
import { ComponentFixture, TestBed, fakeAsync, tick } from '@angular/core/testing';
import { Observable, of, throwError } from 'rxjs';
import { provideLucideIcons, LucideCheck, LucideClock, LucideCopy, LucideLoader2, LucideSparkles, LucideTriangleAlert } from '@lucide/angular';
import { PostCopyComponent } from './post-copy.component';
import { PlatformSpec, PostCopyResult, PostCopySaved, PostCopyTarget, PostCopyView } from './post-copy.models';
import { PostCopyService } from './post-copy.service';

const ICONS = provideLucideIcons(LucideCheck, LucideClock, LucideCopy, LucideLoader2, LucideSparkles, LucideTriangleAlert);

const SPECS: PlatformSpec[] = [
  { id: 'youtube', label: 'YouTube', hasTitle: true, titleMax: 100, titleUnit: 'characters', textLabel: 'Description', textMax: 5000, textUnit: 'bytes', aim: 700, hashtagsMax: 5, hashtagsMin: 3 },
  { id: 'tiktok', label: 'TikTok', hasTitle: false, textLabel: 'Caption', textMax: 2200, textUnit: 'utf16', aim: 220, hashtagsMax: 5, hashtagsMin: 3 },
  { id: 'instagram', label: 'Instagram Reels', hasTitle: false, textLabel: 'Caption', textMax: 2200, textUnit: 'utf16', aim: 400, hashtagsMax: 5, hashtagsMin: 3 },
];

const TAGS = ['#BrassLamp', '#Restoration', '#Wiring'];

function copy(over: Partial<Record<string, { title?: string; text: string; hashtags: string[] }>> = {}) {
  return {
    youtube: { title: 'Fixing a brass lamp', text: 'A brass lamp gets a new cable.', hashtags: TAGS },
    tiktok: { text: 'New cable, old lamp.', hashtags: TAGS },
    instagram: { text: 'A brass lamp, rewired.', hashtags: TAGS },
    ...over,
  };
}

function saved(over: Partial<PostCopySaved> = {}): PostCopySaved {
  return { copy: copy(), notes: [], source: 'generated', generatedAt: '2026-11-24T10:00:00Z', updatedAt: '2026-11-24T10:00:00Z', ...over };
}

function view(over: Partial<PostCopyView> = {}): PostCopyView {
  return { enabled: true, platforms: SPECS, saved: null, remainingToday: 20, dailyLimit: 20, ...over };
}

function result(over: Partial<PostCopyResult> = {}): PostCopyResult {
  return { ...saved(), remainingToday: 19, dailyLimit: 20, replayed: false, ...over };
}

function failure(status: number, code = '', message = '', details?: unknown): HttpErrorResponse {
  return new HttpErrorResponse({ status, error: status === 0 ? new ProgressEvent('error') : { code, message, ...(details ? { details } : {}) } });
}

describe('PostCopyComponent', () => {
  let service: jasmine.SpyObj<PostCopyService>;
  let fixture: ComponentFixture<PostCopyComponent>;
  let el: HTMLElement;
  let clipboard: string[];
  let emitted: PostCopySaved[];

  const target: PostCopyTarget = { kind: 'clip', jobId: 'job1', clipId: 'clip-1' };

  async function create(v: PostCopyView | HttpErrorResponse = view(), t: PostCopyTarget = target): Promise<void> {
    service.view.and.returnValue(v instanceof HttpErrorResponse ? throwError(() => v) : of(v));
    await TestBed.configureTestingModule({
      imports: [PostCopyComponent],
      providers: [ICONS, { provide: PostCopyService, useValue: service }],
    }).compileComponents();
    fixture = TestBed.createComponent(PostCopyComponent);
    fixture.componentRef.setInput('target', t);
    emitted = [];
    fixture.componentInstance.savedChange.subscribe((s) => emitted.push(s));
    el = fixture.nativeElement;
    await settle();
  }

  const settle = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  const q = <T extends HTMLElement>(id: string) => el.querySelector<T>(`[data-testid="${id}"]`)!;
  const has = (id: string) => !!el.querySelector(`[data-testid="${id}"]`);
  const text = () => el.textContent!.replace(/\s+/g, ' ');
  async function type(id: string, value: string): Promise<void> {
    const box = q<HTMLInputElement | HTMLTextAreaElement>(id);
    box.value = value;
    box.dispatchEvent(new Event('input'));
    await settle();
  }
  async function click(id: string): Promise<void> {
    q<HTMLButtonElement>(id).click();
    await settle();
  }

  beforeEach(() => {
    service = jasmine.createSpyObj<PostCopyService>('PostCopyService', ['view', 'list', 'write', 'save']);
    clipboard = [];
    spyOn(navigator.clipboard, 'writeText').and.callFake((t: string) => {
      clipboard.push(t);
      return Promise.resolve();
    });
  });

  describe('when it is switched off or cannot be read', () => {
    it('shows nothing at all when the server has it off', async () => {
      await create(view({ enabled: false }));
      expect(has('post-copy')).toBeFalse();
      expect(text()).not.toContain('Post copy');
    });
    it('says so when it cannot be read, with a way to try again', async () => {
      await create(failure(0));
      expect(el.querySelector('[data-testid="problem"]')).toBeTruthy();
      service.view.and.returnValue(of(view()));
      el.querySelector<HTMLButtonElement>('[data-testid="problem"] button')!.click();
      await settle();
      expect(has('post-copy')).toBeTrue();
    });
  });

  describe('before any copy is written', () => {
    beforeEach(() => create());
    it('offers to write, and shows the uses left', () => {
      expect(q('pc-write').textContent).toContain('Write for all three platforms');
      expect(q('pc-left').textContent).toContain('20 of 20 tries left today');
      expect(has('pc-youtube')).toBeFalse();
    });
    it('says nothing is posted and that it can be wrong', () => {
      expect(text()).toContain('Nothing is posted for you');
      expect(text()).toContain('can still get things wrong');
    });
    it('names the source by what it is', () => {
      expect(text()).toContain('what this clip says');
    });
  });

  describe('writing', () => {
    beforeEach(() => create());
    it('sends the target and a request id the server accepts, and shows three platforms', async () => {
      service.write.and.returnValue(of(result()));
      await click('pc-write');
      expect(service.write).toHaveBeenCalledTimes(1);
      const [t, id] = service.write.calls.mostRecent().args;
      expect(t).toEqual(target);
      expect(id).toMatch(/^[A-Za-z0-9_-]{16,64}$/);
      for (const p of ['youtube', 'tiktok', 'instagram']) expect(has('pc-' + p)).toBeTrue();
      expect(q<HTMLInputElement>('pc-youtube-title').value).toBe('Fixing a brass lamp');
      expect(q<HTMLTextAreaElement>('pc-tiktok-text').value).toBe('New cable, old lamp.');
      expect(q<HTMLTextAreaElement>('pc-instagram-tags').value).toBe('#BrassLamp #Restoration #Wiring');
      expect(q('pc-left').textContent).toContain('19 of 20 tries left');
      expect(emitted.length).toBe(1);
    });
    it('moves focus to the heading and announces it', async () => {
      service.write.and.returnValue(of(result()));
      await click('pc-write');
      expect(document.activeElement?.tagName).toBe('H3');
      expect(q('pc-status').textContent).toContain('ready');
      expect(q('pc-write').textContent).toContain('Write again');
    });
    it('shows what was changed or left out, for copy as it was written', async () => {
      service.write.and.returnValue(of(result({ notes: ['TikTok: 1 sentence left out because it was not in the video.'] })));
      await click('pc-write');
      expect(q('pc-notes').textContent).toContain('1 sentence left out');
    });
    it('is busy while it writes, and cannot be started twice', async () => {
      let done!: (r: PostCopyResult) => void;
      service.write.and.returnValue(new Observable<PostCopyResult>((s) => { done = (r) => { s.next(r); s.complete(); }; }));
      await click('pc-write');
      expect(q<HTMLButtonElement>('pc-write').disabled).toBeTrue();
      expect(q('pc-write').getAttribute('aria-busy')).toBe('true');
      expect(q('pc-status').textContent).toContain('Writing');
      q<HTMLButtonElement>('pc-write').click();
      expect(service.write).toHaveBeenCalledTimes(1);
      done(result());
      await settle();
      expect(q<HTMLButtonElement>('pc-write').disabled).toBeFalse();
    });
    it('says when the answer was one it already had, and that nothing more was used', async () => {
      service.write.and.returnValue(of(result({ replayed: true })));
      await click('pc-write');
      expect(q('pc-status').textContent).toContain('Nothing more was used');
    });
  });

  describe('a retry is the same request', () => {
    beforeEach(() => create());
    it('reuses the id after a dropped connection, and takes a new one after a finished try', async () => {
      service.write.and.returnValues(throwError(() => failure(0)), of(result()));
      await click('pc-write');
      expect(q('problem').getAttribute('data-kind')).toBe('offline');
      const first = service.write.calls.argsFor(0)[1];
      el.querySelector<HTMLButtonElement>('[data-testid="problem"] button')!.click();
      await settle();
      expect(service.write.calls.argsFor(1)[1]).toBe(first);
      expect(has('pc-youtube')).toBeTrue();
      service.write.and.returnValue(of(result()));
      await click('pc-write');
      const third = service.write.calls.mostRecent().args[1];
      expect(third).not.toBe(first);
    });
    it('keeps the id after a timeout too, since the first try may still be running', async () => {
      service.write.and.returnValues(throwError(() => failure(504)), of(result()));
      await click('pc-write');
      const first = service.write.calls.argsFor(0)[1];
      el.querySelector<HTMLButtonElement>('[data-testid="problem"] button')!.click();
      await settle();
      expect(service.write.calls.argsFor(1)[1]).toBe(first);
    });
    it('takes a new id after an answer the server gave for good', async () => {
      service.write.and.returnValues(throwError(() => failure(502)), of(result()));
      await click('pc-write');
      const first = service.write.calls.argsFor(0)[1];
      el.querySelector<HTMLButtonElement>('[data-testid="problem"] button')!.click();
      await settle();
      expect(service.write.calls.argsFor(1)[1]).not.toBe(first);
    });
  });

  describe('a request that is still being written', () => {
    it('waits and asks again with the same id while the first try is still being written', fakeAsync(() => {
      service.view.and.returnValue(of(view()));
      TestBed.configureTestingModule({ imports: [PostCopyComponent], providers: [ICONS, { provide: PostCopyService, useValue: service }] });
      fixture = TestBed.createComponent(PostCopyComponent);
      fixture.componentRef.setInput('target', target);
      el = fixture.nativeElement;
      fixture.detectChanges();
      service.write.and.returnValues(throwError(() => failure(409, 'IN_PROGRESS')), throwError(() => failure(409, 'IN_PROGRESS')), of(result()));
      q<HTMLButtonElement>('pc-write').click();
      fixture.detectChanges();
      expect(service.write).toHaveBeenCalledTimes(1);
      expect(q<HTMLButtonElement>('pc-write').disabled).toBeTrue();
      tick(2000);
      expect(service.write).toHaveBeenCalledTimes(2);
      tick(2000);
      expect(service.write).toHaveBeenCalledTimes(3);
      fixture.detectChanges();
      const ids = service.write.calls.allArgs().map((a) => a[1]);
      expect(new Set(ids).size).toBe(1);
      expect(el.querySelector('[data-testid="pc-youtube"]')).toBeTruthy();
    }));
    it('gives up waiting after a while and says so', fakeAsync(() => {
      service.view.and.returnValue(of(view()));
      TestBed.configureTestingModule({ imports: [PostCopyComponent], providers: [ICONS, { provide: PostCopyService, useValue: service }] });
      fixture = TestBed.createComponent(PostCopyComponent);
      fixture.componentRef.setInput('target', target);
      el = fixture.nativeElement;
      fixture.detectChanges();
      service.write.and.returnValue(throwError(() => failure(409, 'IN_PROGRESS')));
      q<HTMLButtonElement>('pc-write').click();
      for (let i = 0; i < 12; i++) tick(2000);
      fixture.detectChanges();
      expect(service.write.calls.count()).toBe(9);
      expect(q('problem').textContent).toContain('Still writing');
    }));
  });

  describe('problems writing', () => {
    beforeEach(() => create());
    it('says a limit has been reached, and keeps what is there', async () => {
      service.write.and.returnValue(throwError(() => failure(429, 'QUOTA_EXCEEDED', 'x', { retryAfter: 3600 })));
      await click('pc-write');
      expect(q('problem').getAttribute('data-kind')).toBe('rate_limited');
      expect(q('pc-left').textContent).toContain('0 of 20');
      expect(has('pc-youtube')).toBeFalse();
    });
    it('says it is not switched on', async () => {
      service.write.and.returnValue(throwError(() => failure(503, 'UNAVAILABLE')));
      await click('pc-write');
      expect(q('problem').getAttribute('data-kind')).toBe('disabled');
    });
  });

  describe('with a copy that is saved', () => {
    beforeEach(() => create(view({ saved: saved() })));
    it('shows it without writing anything', () => {
      expect(service.write).not.toHaveBeenCalled();
      expect(q<HTMLTextAreaElement>('pc-youtube-text').value).toBe('A brass lamp gets a new cable.');
      expect(q('pc-write').textContent).toContain('Write again');
      expect(q('pc-state').textContent).toContain('kept with this clip');
    });
    it('does not ask to emit it again', () => expect(emitted.length).toBe(0));
    it('labels each part', () => {
      const labels = Array.from(el.querySelectorAll('label')).map((l) => l.textContent!.trim());
      expect(labels).toEqual(['Title', 'Description', 'Hashtags', 'Caption', 'Hashtags', 'Caption', 'Hashtags']);
      for (const l of Array.from(el.querySelectorAll('label'))) {
        expect(el.querySelector('#' + l.getAttribute('for'))).withContext(l.textContent!).toBeTruthy();
      }
    });
    it('gives every button a tap target of at least 44 pixels', () => {
      for (const b of Array.from(el.querySelectorAll('button'))) expect(b.getBoundingClientRect().height).withContext(b.textContent!).toBeGreaterThanOrEqual(44);
    });
    it('uses no em dash', () => expect(text()).not.toContain('\u2014'));
  });

  describe('for a story video', () => {
    it('says story, and uses the story target', async () => {
      await create(view({ saved: saved() }), { kind: 'story', videoId: 'v1' });
      expect(text()).toContain('what this story says');
      expect(q('pc-state').textContent).toContain('kept with this video');
    });
  });

  describe('counting and limits', () => {
    beforeEach(() => create(view({ saved: saved() })));
    it('counts the YouTube title in characters and the description in bytes, hashtags included', () => {
      expect(q('pc-youtube-title-count').textContent).toContain('19 of 100 characters');
      const full = 'A brass lamp gets a new cable.\n\n#BrassLamp #Restoration #Wiring';
      expect(q('pc-youtube-text-count').textContent).toContain(`${new TextEncoder().encode(full).length} of 5,000 bytes, hashtags included`);
    });
    it('counts a TikTok caption in UTF-16 units', async () => {
      await type('pc-tiktok-text', '😀');
      const full = '😀\n\n#BrassLamp #Restoration #Wiring';
      expect(q('pc-tiktok-text-count').textContent).toContain(`${full.length} of 2,200 characters`);
    });
    it('counts hashtags', () => expect(q('pc-tiktok-tags-count').textContent).toContain('3 of 5 hashtags'));

    it('stops copying and saving a title over its limit, says why, and lets go when it is fixed', async () => {
      await type('pc-youtube-title', 'a'.repeat(101));
      expect(q('pc-youtube-title-count').classList).toContain('over');
      expect(q('pc-youtube-title-why').textContent).toContain('101 characters');
      expect(q('pc-youtube-title').getAttribute('aria-invalid')).toBe('true');
      expect(q<HTMLButtonElement>('pc-youtube-copy-title').disabled).toBeTrue();
      expect(q<HTMLButtonElement>('pc-save').disabled).toBeTrue();
      // The other parts are still copyable.
      expect(q<HTMLButtonElement>('pc-youtube-copy-text').disabled).toBeFalse();
      expect(q<HTMLButtonElement>('pc-tiktok-copy-text').disabled).toBeFalse();
      expect(q('pc-status').textContent).toContain('YouTube');
      await type('pc-youtube-title', 'a'.repeat(100));
      expect(has('pc-youtube-title-why')).toBeFalse();
      expect(q<HTMLButtonElement>('pc-youtube-copy-title').disabled).toBeFalse();
      expect(q<HTMLButtonElement>('pc-save').disabled).toBeFalse();
      expect(q('pc-status').textContent).toContain('within its limit');
    });
    it('stops a Hindi description that is under 5,000 characters but over 5,000 bytes', async () => {
      await type('pc-youtube-text', 'द'.repeat(1700));
      expect(q('pc-youtube-text-why').textContent).toContain('bytes');
      expect(q<HTMLButtonElement>('pc-youtube-copy-text').disabled).toBeTrue();
      expect(q<HTMLButtonElement>('pc-save').disabled).toBeTrue();
    });
    it('stops a caption with its hashtags over 2,200', async () => {
      await type('pc-instagram-text', 'a'.repeat(2190));
      expect(q('pc-instagram-text-why').textContent).toContain('limit is 2,200');
      expect(q<HTMLButtonElement>('pc-instagram-copy-text').disabled).toBeTrue();
    });
    it('stops six hashtags, a repeated one and a bad one', async () => {
      await type('pc-tiktok-tags', '#a1 #b2 #c3 #d4 #e5 #f6');
      expect(q('pc-tiktok-tags-why').textContent).toContain('at most 5');
      expect(q<HTMLButtonElement>('pc-tiktok-copy-text').disabled).toBeTrue();
      await type('pc-tiktok-tags', '#Lamp #lamp');
      expect(q('pc-tiktok-tags-why').textContent).toContain('twice');
      await type('pc-tiktok-tags', '#a1 !!!');
      expect(q('pc-tiktok-tags-why').textContent).toContain('!!! is not a hashtag');
      expect(q<HTMLButtonElement>('pc-save').disabled).toBeTrue();
    });
    it('stops an empty title or caption', async () => {
      await type('pc-youtube-title', '   ');
      expect(q('pc-youtube-title-why').textContent).toContain('needs a title');
      await type('pc-youtube-title', 'Fine');
      await type('pc-tiktok-text', '');
      expect(q('pc-tiktok-text-why').textContent).toContain('needs its caption');
    });
    it('does not announce on every key, only when a part crosses its limit', async () => {
      await type('pc-youtube-title', 'a'.repeat(50));
      expect(q('pc-status').textContent!.trim()).toBe('');
      await type('pc-youtube-title', 'a'.repeat(101));
      const said = q('pc-status').textContent;
      await type('pc-youtube-title', 'a'.repeat(102));
      expect(q('pc-status').textContent).toBe(said);
    });
    it('cleans tabs and control characters out of what is typed', async () => {
      await type('pc-tiktok-text', 'a\tb\u0000c');
      expect(q<HTMLTextAreaElement>('pc-tiktok-text').value).toBe('a bc');
    });
    it('tidies the hashtag box on leaving it', async () => {
      await type('pc-tiktok-tags', 'lamp  wiring,#Vintage');
      q<HTMLTextAreaElement>('pc-tiktok-tags').dispatchEvent(new Event('blur'));
      await settle();
      expect(q<HTMLTextAreaElement>('pc-tiktok-tags').value).toBe('#lamp #wiring #Vintage');
    });
    it('marks Hindi text as Hindi', async () => {
      await type('pc-tiktok-text', 'दीपक');
      expect(q('pc-tiktok-text').getAttribute('lang')).toBe('hi');
      expect(q('pc-youtube-title').getAttribute('lang')).toBeNull();
    });
  });

  describe('copying', () => {
    beforeEach(() => create(view({ saved: saved() })));
    // A copied mark clears itself after two seconds, and whenStable would wait for that, so
    // these clicks only let the clipboard promise settle.
    async function click(id: string): Promise<void> {
      q<HTMLButtonElement>(id).click();
      for (let i = 0; i < 5; i++) await Promise.resolve();
      fixture.detectChanges();
    }
    it('copies the title, the description with its hashtags, and the hashtags alone', async () => {
      await click('pc-youtube-copy-title');
      await click('pc-youtube-copy-text');
      await click('pc-youtube-copy-tags');
      await click('pc-tiktok-copy-text');
      expect(clipboard).toEqual([
        'Fixing a brass lamp',
        'A brass lamp gets a new cable.\n\n#BrassLamp #Restoration #Wiring',
        '#BrassLamp #Restoration #Wiring',
        'New cable, old lamp.\n\n#BrassLamp #Restoration #Wiring',
      ]);
    });
    it('copies what is on the page, edits included', async () => {
      await type('pc-tiktok-text', 'My own caption.');
      await click('pc-tiktok-copy-text');
      expect(clipboard[0]).toBe('My own caption.\n\n#BrassLamp #Restoration #Wiring');
    });
    it('says Copied, announces it, and goes back', async () => {
      await click('pc-tiktok-copy-text');
      expect(q('pc-tiktok-copy-text').textContent).toContain('Copied');
      expect(q('pc-status').textContent).toContain('TikTok caption copied');
    });
    it('names the platform for a screen reader on each button', () => {
      expect(q('pc-tiktok-copy-text').textContent).toContain('for TikTok');
      expect(q('pc-youtube-copy-title').textContent).toContain('for YouTube');
    });
    it('says so when the browser will not copy', async () => {
      (navigator.clipboard.writeText as jasmine.Spy).and.returnValue(Promise.reject(new Error('no')));
      spyOn(document, 'execCommand').and.returnValue(false);
      await click('pc-tiktok-copy-text');
      expect(q('problem').textContent).toContain("Couldn't copy");
    });
    it('has nothing to copy for hashtags when there are none', async () => {
      await type('pc-tiktok-tags', '');
      expect(q<HTMLButtonElement>('pc-tiktok-copy-tags').disabled).toBeTrue();
    });
  });

  describe('saving what was edited', () => {
    beforeEach(() => create(view({ saved: saved() })));
    it('cannot save until something changes', () => {
      expect(q<HTMLButtonElement>('pc-save').disabled).toBeTrue();
      expect(has('pc-undo')).toBeFalse();
    });
    it('says there are changes, and sends exactly what is typed', async () => {
      await type('pc-youtube-title', 'My own title');
      await type('pc-youtube-text', 'First line.\n\nSecond paragraph with a link https://example.com/a kept whole.');
      await type('pc-tiktok-tags', '#lamp #दीपक');
      expect(q('pc-state').textContent).toContain('changes that are not saved');
      service.save.and.returnValue(of(saved({ source: 'edited' })));
      await click('pc-save');
      expect(service.save).toHaveBeenCalledOnceWith(target, {
        youtube: { title: 'My own title', text: 'First line.\n\nSecond paragraph with a link https://example.com/a kept whole.', hashtags: TAGS },
        tiktok: { text: 'New cable, old lamp.', hashtags: ['#lamp', '#दीपक'] },
        instagram: { text: 'A brass lamp, rewired.', hashtags: TAGS },
      });
    });
    it('sends no title for a platform that has none', async () => {
      await type('pc-tiktok-text', 'x');
      service.save.and.returnValue(of(saved({ source: 'edited' })));
      await click('pc-save');
      const sent = service.save.calls.mostRecent().args[1];
      expect('title' in sent['tiktok']).toBeFalse();
      expect('title' in sent['youtube']).toBeTrue();
    });
    it('says Saved, tells the page, and is clean again', async () => {
      await type('pc-tiktok-text', 'x');
      service.save.and.returnValue(of(saved({ source: 'edited', copy: copy({ tiktok: { text: 'x', hashtags: TAGS } }) })));
      await click('pc-save');
      expect(q('pc-state').textContent).toContain('Saved');
      expect(q<HTMLButtonElement>('pc-save').disabled).toBeTrue();
      expect(emitted.length).toBe(1);
      expect(emitted[0].source).toBe('edited');
      expect(q('pc-status').textContent).toBe('Saved.');
    });
    it('keeps the edits and offers to try again when saving fails', async () => {
      await type('pc-tiktok-text', 'my edit');
      service.save.and.returnValues(throwError(() => failure(400, 'VALIDATION_ERROR', 'TikTok caption with its hashtags is 2,300 characters; the limit is 2,200 characters.')), of(saved({ source: 'edited' })));
      await click('pc-save');
      expect(q('problem').textContent).toContain('2,300 characters');
      expect(q<HTMLTextAreaElement>('pc-tiktok-text').value).toBe('my edit');
      expect(q<HTMLButtonElement>('pc-save').disabled).toBeFalse();
      service.save.and.returnValue(of(saved({ source: 'edited' })));
      await click('pc-save');
      expect(service.save).toHaveBeenCalledTimes(2);
      expect(has('problem')).toBeFalse();
    });
    it('repeats a save, not a write, when Try again is pressed after a failed save', async () => {
      await type('pc-tiktok-text', 'my edit');
      service.save.and.returnValues(throwError(() => failure(0)), of(saved({ source: 'edited' })));
      await click('pc-save');
      el.querySelector<HTMLButtonElement>('[data-testid="problem"] button')!.click();
      await settle();
      expect(service.save).toHaveBeenCalledTimes(2);
      expect(service.write).not.toHaveBeenCalled();
    });
    it('undoes changes', async () => {
      await type('pc-tiktok-text', 'my edit');
      await click('pc-undo');
      expect(q<HTMLTextAreaElement>('pc-tiktok-text').value).toBe('New cable, old lamp.');
      expect(has('pc-undo')).toBeFalse();
    });
  });

  describe('writing again over something', () => {
    it('asks first when there are unsaved changes, and keeps them if told to', async () => {
      await create(view({ saved: saved() }));
      await type('pc-tiktok-text', 'my edit');
      await click('pc-write');
      expect(q('pc-confirm').textContent).toContain('including changes you have not saved');
      expect(service.write).not.toHaveBeenCalled();
      await click('pc-confirm-no');
      expect(has('pc-confirm')).toBeFalse();
      expect(q<HTMLTextAreaElement>('pc-tiktok-text').value).toBe('my edit');
    });
    it('asks first when the saved copy was edited, and replaces it when told to', async () => {
      await create(view({ saved: saved({ source: 'edited' }) }));
      await click('pc-write');
      expect(has('pc-confirm')).toBeTrue();
      service.write.and.returnValue(of(result({ copy: copy({ tiktok: { text: 'Fresh.', hashtags: TAGS } }) })));
      await click('pc-confirm-yes');
      expect(service.write).toHaveBeenCalledTimes(1);
      expect(q<HTMLTextAreaElement>('pc-tiktok-text').value).toBe('Fresh.');
    });
    it('does not ask when what is there is as it was written', async () => {
      await create(view({ saved: saved() }));
      service.write.and.returnValue(of(result()));
      await click('pc-write');
      expect(has('pc-confirm')).toBeFalse();
      expect(service.write).toHaveBeenCalledTimes(1);
    });
    it('Escape closes the question', async () => {
      await create(view({ saved: saved({ source: 'edited' }) }));
      await click('pc-write');
      q('pc-confirm').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      await settle();
      expect(has('pc-confirm')).toBeFalse();
    });
  });

  describe('writing it myself', () => {
    beforeEach(() => create());
    it('opens empty boxes with the same limits, and saves what is typed', async () => {
      await click('pc-blank');
      expect(q<HTMLTextAreaElement>('pc-tiktok-text').value).toBe('');
      expect(q<HTMLButtonElement>('pc-save').disabled).toBeTrue();
      await type('pc-youtube-title', 'Mine');
      await type('pc-youtube-text', 'My description.');
      await type('pc-tiktok-text', 'My caption.');
      await type('pc-instagram-text', 'My other caption.');
      expect(q<HTMLButtonElement>('pc-save').disabled).toBeFalse();
      service.save.and.returnValue(of(saved({ source: 'edited' })));
      await click('pc-save');
      expect(service.save.calls.mostRecent().args[1]['youtube']).toEqual({ title: 'Mine', text: 'My description.', hashtags: [] });
    });
    it('goes back to the start when undone', async () => {
      await click('pc-blank');
      await type('pc-tiktok-text', 'x');
      await click('pc-undo');
      expect(has('pc-youtube')).toBeFalse();
      expect(has('pc-blank')).toBeTrue();
    });
  });
});
