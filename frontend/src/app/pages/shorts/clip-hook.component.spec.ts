import { HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { provideLucideIcons, LucideCheck, LucideCircleX, LucideLoader2, LucideRefreshCw, LucideSparkles, LucideTriangleAlert, LucideClock } from '@lucide/angular';
import { ClipHookComponent } from './clip-hook.component';
import { ClipHook, ClipRenderView, HookView, defaultHook } from './clip-hook.models';
import { ClipHookService } from './clip-hook.service';

const ICONS = provideLucideIcons(LucideCheck, LucideCircleX, LucideLoader2, LucideRefreshCw, LucideSparkles, LucideTriangleAlert, LucideClock);

function view(over: Partial<HookView> = {}): HookView {
  return {
    hook: defaultHook(), saved: false, ranker: 'I lost three clients in one week', suggestionsEnabled: true, renderEnabled: true,
    suggestRemainingToday: 20, suggestDailyLimit: 20, renderRemainingToday: 15, renderDailyLimit: 15, maxChars: 80, ...over,
  };
}

function failure(status: number, code = '', message = '', details?: unknown): HttpErrorResponse {
  return new HttpErrorResponse({ status, headers: new HttpHeaders(), error: { code, message, ...(details ? { details } : {}) } });
}

describe('ClipHookComponent', () => {
  let service: jasmine.SpyObj<ClipHookService>;
  let fixture: ComponentFixture<ClipHookComponent>;
  let el: HTMLElement;
  let drafts: ClipHook[];
  let started: ClipRenderView[];

  async function create(v: HookView | HttpErrorResponse = view(), render: ClipRenderView | null = null): Promise<void> {
    service.view.and.returnValue(v instanceof HttpErrorResponse ? throwError(() => v) : of(v));
    await TestBed.configureTestingModule({ imports: [ClipHookComponent], providers: [ICONS, { provide: ClipHookService, useValue: service }] }).compileComponents();
    fixture = TestBed.createComponent(ClipHookComponent);
    fixture.componentRef.setInput('jobId', 'job1');
    fixture.componentRef.setInput('clipId', 'clip-1');
    fixture.componentRef.setInput('render', render);
    drafts = [];
    started = [];
    fixture.componentInstance.draftChange.subscribe((d) => drafts.push(d));
    fixture.componentInstance.renderStarted.subscribe((r) => started.push(r));
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
  }

  const q = <T extends Element>(sel: string) => el.querySelector(sel) as T;
  const typeText = (text: string) => {
    const input = q<HTMLInputElement>('[data-testid="hook-text"]');
    input.value = text;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  };
  const button = (label: string) =>
    Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.trim().includes(label)) as HTMLButtonElement;

  beforeEach(() => {
    service = jasmine.createSpyObj<ClipHookService>('ClipHookService', ['view', 'save', 'reset', 'suggest', 'render']);
  });

  it('starts from the saved hook and offers the ranking line for free', async () => {
    await create(view({ saved: true, hook: { enabled: true, text: 'Saved line', style: 'band', position: 'middle', seconds: 3 } }));
    expect(q<HTMLInputElement>('[data-testid="hook-text"]').value).toBe('Saved line');
    expect(q('[data-testid="hook-enabled"]').getAttribute('aria-checked')).toBe('true');
    expect(q('[data-testid="hook-ideas"]').textContent).toContain('I lost three clients in one week');
    expect(q('[data-testid="hook-ideas"]').textContent).toContain('From the ranking');
    expect(el.textContent).toContain('approximate');
    expect(drafts.at(-1)?.style).toBe('band');
  });

  it('counts Hindi the way the server does and blocks saving past 80', async () => {
    await create();
    typeText('क्षमा');
    expect(q('.count').textContent).toContain('2 / 80');
    typeText('क्ष'.repeat(81));
    expect(q('.count').classList).toContain('over');
    expect(el.textContent).toContain('The limit is 80');
    expect(q<HTMLButtonElement>('[data-testid="hook-save"]').disabled).toBeTrue();
  });

  it('uses an idea, saves it, and asks to render to see it', async () => {
    await create();
    service.save.and.callFake((_j, _c, h) => of({ hook: { ...h, updatedAt: 'now' } }));
    (el.querySelector('.ideas button') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(drafts.at(-1)).toEqual(jasmine.objectContaining({ enabled: true, text: 'I lost three clients in one week' }));
    // Unsaved: the render waits for a save.
    expect(q<HTMLButtonElement>('[data-testid="hook-render-again"]').disabled).toBeTrue();
    expect(el.textContent).toContain('Save the hook first');
    q<HTMLButtonElement>('[data-testid="hook-save"]').click();
    fixture.detectChanges();
    expect(service.save).toHaveBeenCalledWith('job1', 'clip-1', jasmine.objectContaining({ enabled: true, style: 'box', position: 'top', seconds: 2.5 }));
    expect(el.textContent).toContain('Render the clip again to see it');
    expect(q<HTMLButtonElement>('[data-testid="hook-render-again"]').disabled).toBeFalse();
  });

  it('shows the server reason when a hook cannot be drawn', async () => {
    await create();
    service.save.and.returnValue(throwError(() => failure(400, 'VALIDATION_ERROR', 'This hook is too long to fit on two lines. Shorten it.')));
    q<HTMLButtonElement>('[data-testid="hook-enabled"]').click();
    typeText('A line');
    q<HTMLButtonElement>('[data-testid="hook-save"]').click();
    fixture.detectChanges();
    expect(el.textContent).toContain('too long to fit on two lines');
    expect(q<HTMLInputElement>('[data-testid="hook-text"]').value).toBe('A line');
  });

  it('suggests lines, and keeps the request id across a retryable failure', async () => {
    await create();
    service.suggest.and.returnValues(
      throwError(() => failure(504, 'COPY_TIMEOUT')),
      of({ suggestions: ['One line', 'Two line'], ranker: 'x', dropped: 1, remainingToday: 18, dailyLimit: 20, replayed: false }),
    );
    q<HTMLButtonElement>('[data-testid="hook-suggest"]').click();
    fixture.detectChanges();
    expect(el.textContent).toContain("didn't use up");
    button('Try again').click();
    fixture.detectChanges();
    const ids = service.suggest.calls.allArgs().map((a) => a[2]);
    expect(ids[0]).toBe(ids[1]);
    expect(q('[data-testid="hook-ideas"]').textContent).toContain('Two line');
    expect(el.textContent).toContain('18 of 20 left today');
    expect(el.textContent).toContain('1 more line was left out');
  });

  it('keeps the ranking line when suggestions fail', async () => {
    await create();
    service.suggest.and.returnValue(throwError(() => failure(502, 'COPY_FAILED')));
    q<HTMLButtonElement>('[data-testid="hook-suggest"]').click();
    fixture.detectChanges();
    expect(q('[data-testid="hook-ideas"]').textContent).toContain('I lost three clients in one week');
  });

  it('reports used-up suggestions clearly', async () => {
    await create();
    service.suggest.and.returnValue(throwError(() => failure(429, 'QUOTA_EXCEEDED', '', { retryAfter: 120 })));
    q<HTMLButtonElement>('[data-testid="hook-suggest"]').click();
    fixture.detectChanges();
    expect(el.textContent).toContain("You've used today's suggestions");
    expect(el.textContent).toContain('in 2 minutes');
  });

  it('renders again once, tells the page, and keeps the id across a retryable failure', async () => {
    await create(view({ saved: true, hook: { enabled: true, text: 'x', style: 'box', position: 'top', seconds: 2.5 } }));
    service.render.and.returnValues(
      throwError(() => failure(0)),
      of({ render: { state: 'rendering' as const, requestedAt: 'now' }, replayed: false, remainingToday: 14, dailyLimit: 15 }),
    );
    q<HTMLButtonElement>('[data-testid="hook-render-again"]').click();
    fixture.detectChanges();
    expect(el.textContent).toContain("You're offline");
    button('Try again').click();
    fixture.detectChanges();
    const ids = service.render.calls.allArgs().map((a) => a[2]);
    expect(ids[0]).toBe(ids[1]);
    expect(started.length).toBe(1);
    expect(el.textContent).toContain('Rendering this clip again');
    expect(el.textContent).toContain('14 of 15 left today');
    expect(q<HTMLButtonElement>('[data-testid="hook-render-again"]').disabled).toBeTrue();
  });

  it('reports used-up renders clearly and says the clip stays', async () => {
    await create();
    service.render.and.returnValue(throwError(() => failure(429, 'QUOTA_EXCEEDED')));
    q<HTMLButtonElement>('[data-testid="hook-render-again"]').click();
    fixture.detectChanges();
    expect(el.textContent).toContain("You've used today's renders");
    expect(el.textContent).toContain('stays as it is');
    // A fresh request id next time: the refused one was never accepted.
    q<HTMLButtonElement>('[data-testid="hook-render-again"]').click();
    const ids = service.render.calls.allArgs().map((a) => a[2]);
    expect(ids[0]).not.toBe(ids[1]);
  });

  it('shows the state the page passes in', async () => {
    await create(view(), { state: 'failed', error: 'This clip took too long to render.' });
    expect(el.textContent).toContain('This clip took too long to render.');
    fixture.componentRef.setInput('render', { state: 'done', renders: 1 });
    fixture.detectChanges();
    expect(el.textContent).toContain('Rendered again');
  });

  it('hides suggestions and the render when they are switched off', async () => {
    await create(view({ suggestionsEnabled: false, renderEnabled: false, ranker: '' }));
    expect(q('[data-testid="hook-suggest"]')).toBeNull();
    expect(q('[data-testid="hook-render"]')).toBeNull();
    expect(q('[data-testid="hook-text"]')).not.toBeNull();
  });

  it('offers a retry when it cannot load', async () => {
    await create(failure(0));
    expect(el.textContent).toContain("You're offline");
    service.view.and.returnValue(of(view()));
    button('Try again').click();
    fixture.detectChanges();
    expect(q('[data-testid="hook-text"]')).not.toBeNull();
  });

  it('removes a saved hook', async () => {
    await create(view({ saved: true, hook: { enabled: true, text: 'x', style: 'box', position: 'top', seconds: 2.5 } }));
    service.reset.and.returnValue(of({ reset: true, hook: defaultHook() }));
    button('Remove hook').click();
    fixture.detectChanges();
    expect(service.reset).toHaveBeenCalledWith('job1', 'clip-1');
    expect(q<HTMLInputElement>('[data-testid="hook-text"]').value).toBe('');
    expect(el.textContent).toContain('Hook removed');
  });

  it('has no em dash in its copy', async () => {
    await create();
    expect(el.textContent).not.toContain('\u2014');
  });
});
