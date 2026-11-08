import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { of, throwError } from 'rxjs';
import {
  provideLucideIcons,
  LucideCaptions,
  LucideCheck,
  LucideCircleX,
  LucideClapperboard,
  LucideCopy,
  LucideDownload,
  LucideFilm,
  LucideImage,
  LucideLoader2,
  LucideMoon,
  LucidePencil,
  LucidePlay,
  LucideShare2,
  LucideSparkles,
  LucideSun,
  LucideTrash2,
  LucideUser,
} from '@lucide/angular';
import { Clip, ShortsService } from './shorts.service';
import { ShortsGalleryComponent } from './shorts-gallery.component';
import { ShortsEffectsPanelComponent } from './shorts-effects-panel.component';

const ICONS = provideLucideIcons(
  LucideCaptions, LucideCheck, LucideCircleX, LucideClapperboard, LucideCopy, LucideDownload, LucideFilm, LucideImage,
  LucideLoader2, LucideMoon, LucidePencil, LucidePlay, LucideShare2, LucideSparkles, LucideSun, LucideTrash2, LucideUser,
);

const clips: Clip[] = [2, 1, 3].map((r) => ({ clipId: 'c' + r, s3Key: 'k', rank: r, score: 0.95 - r / 10, duration: 30, hookText: 'Hook ' + r, start: r * 60, end: r * 60 + 30 }));
const job = (status: string, extra: Record<string, unknown> = {}) => ({ jobId: 'j1', status, fileType: 'mp4', sourceDuration: 600, clipEdits: {}, ...extra });

function base(service: Record<string, unknown>) {
  TestBed.configureTestingModule({
    providers: [
      provideHttpClient(),
      provideHttpClientTesting(),
      provideRouter([]),
      ICONS,
      { provide: ShortsService, useValue: service },
      { provide: ActivatedRoute, useValue: { snapshot: { paramMap: convertToParamMap({ id: 'j1' }) } } },
    ],
  });
}

const tidy = (el: HTMLElement) => {
  expect(el.innerHTML).not.toContain('var(--primary');
  expect(el.innerHTML).not.toContain('gradient');
  expect(el.innerHTML).not.toContain('glass-card');
  expect(el.textContent).not.toContain('\u00b7');
};

describe('ShortsGalleryComponent', () => {
  function setup(over: Record<string, unknown> = {}) {
    const service = {
      listClips: () => of(clips),
      getClipUrl: () => of({}),
      getStatus: () => of(job('completed')),
      getClipCaptions: () => of({ start: 60, end: 90, words: [{ text: 'Hello', start: 60, end: 60.5 }, { text: 'there', start: 60.5, end: 61 }] }),
      updateClipEdit: jasmine.createSpy('updateClipEdit').and.returnValue(of({ droppedWords: 0 })),
      updateCaption: jasmine.createSpy('updateCaption').and.returnValue(of({})),
      deleteJob: jasmine.createSpy('deleteJob').and.returnValue(of({})),
      updateEffects: () => of({}),
      ...over,
    };
    base(service);
    const fixture = TestBed.createComponent(ShortsGalleryComponent);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const btn = (label: string) => el.querySelector(`button[aria-label="${label}"]`) as HTMLButtonElement;
    return { fixture, el, service, btn };
  }

  it('is titled Clips and shows one card per clip, ranked, with its player and every action', () => {
    const { el } = setup();
    expect(el.querySelectorAll('h1').length).toBe(1);
    expect(el.querySelector('h1')!.textContent!.trim()).toBe('Clips');
    const cards = Array.from(el.querySelectorAll('[data-clip]'));
    expect(cards.map((c) => c.getAttribute('data-clip'))).toEqual(['c1', 'c2', 'c3']);
    const first = cards[0];
    expect(first.querySelector('.player')).toBeTruthy();
    expect(first.querySelector('button[aria-label="Preview clip 1"]')).toBeTruthy();
    for (const label of ['Download clip', 'Copy caption', 'Share clip', 'Choose thumbnail frames for clip 1', 'Edit boundaries and captions for clip 1']) {
      expect(first.querySelector(`button[aria-label="${label}"]`)).withContext(label).toBeTruthy();
    }
    expect(first.textContent).toContain('Hook 1');
    expect(first.querySelector('[aria-label^="Engagement score"]')).toBeTruthy();
    tidy(el);
  });

  it('selects clips and counts them for bulk download', () => {
    const { fixture, el, btn } = setup();
    btn('Select clip 2').click();
    fixture.detectChanges();
    expect(btn('Deselect clip 2').getAttribute('aria-checked')).toBe('true');
    expect(el.textContent).toContain('1 of 3 selected');
  });

  it('opens the caption style panel and saves the style for a job that can still change', () => {
    const { fixture, el, btn, service } = setup({ getStatus: () => of(job('failed')) });
    btn('Caption styles').click();
    fixture.detectChanges();
    expect(el.querySelector('[aria-label="Caption presets"]')!.querySelectorAll('button').length).toBe(4);
    const save = Array.from(el.querySelectorAll('button')).find((b) => b.textContent!.trim() === 'Save style')!;
    expect(save).toBeTruthy();
    fixture.componentInstance.saveStyle();
    expect(service.updateCaption).toHaveBeenCalled();
  });

  it('says a rendered job keeps its style for future renders', () => {
    const { fixture, el, btn } = setup();
    btn('Caption styles').click();
    fixture.detectChanges();
    expect(el.querySelector('[role="note"]')!.textContent).toContain('already rendered');
  });

  it('edits a clip: loads captions, saves the edit, and shows a server conflict message', () => {
    const { fixture, el, btn, service } = setup({
      updateClipEdit: jasmine.createSpy('updateClipEdit').and.returnValue(throwError(() => ({ error: { message: 'This clip changed in another tab. Reload to see it.' } }))),
    });
    btn('Edit boundaries and captions for clip 1').click();
    fixture.detectChanges();
    const editor = el.querySelector('[aria-label="Clip editor"]')!;
    expect(editor.querySelector('input[aria-label="Clip start time in seconds"]')).toBeTruthy();
    expect(editor.querySelectorAll('input[type="text"]').length).toBe(2);
    fixture.componentInstance.saveEdit();
    fixture.detectChanges();
    expect(service.updateClipEdit).toHaveBeenCalled();
    expect(editor.querySelector('[role="alert"]')!.textContent).toContain('changed in another tab');
  });

  it('asks before deleting the whole short', () => {
    const { fixture, btn } = setup();
    btn('Delete this short and all its assets').click();
    fixture.detectChanges();
    expect(fixture.componentInstance.pendingDelete).toBeTrue();
  });

  it('shows the error state with a retry when clips fail to load', () => {
    const { el } = setup({ listClips: () => throwError(() => ({ error: { message: 'Not ready yet.' } })) });
    const alert = el.querySelector('[role="alert"]')!;
    expect(alert.textContent).toContain('Not ready yet.');
    expect(alert.querySelector('button')!.textContent).toContain('Try again');
  });
});

describe('ShortsEffectsPanelComponent', () => {
  function setup(status: string, effects: Record<string, unknown> = {}) {
    const service = {
      getStatus: () => of(job(status, { effects })),
      updateEffects: jasmine.createSpy('updateEffects').and.returnValue(of(job(status))),
    };
    base(service);
    const fixture = TestBed.createComponent(ShortsEffectsPanelComponent);
    fixture.componentInstance.jobId = 'j1';
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, service, cmp: fixture.componentInstance };
  }

  it('shows every effect group with labelled on switches and saves', () => {
    const { el, cmp, service } = setup('failed');
    for (const name of ['Quick presets', 'Animated captions', 'Dynamic zoom', 'Color grading', 'Freeze frame and impact text', 'Fade in and out', 'Speed', 'Film look', 'Audio visualizer', 'Camera shake']) {
      expect(el.textContent).withContext(name).toContain(name);
    }
    expect(el.querySelectorAll('input[type="checkbox"].sw').length).toBe(9);
    expect(el.querySelector('input[aria-label="Turn on dynamic zoom"]')).toBeTruthy();
    cmp.save();
    expect(service.updateEffects).toHaveBeenCalled();
    tidy(el);
  });

  it('blocks saving while speed and freeze are both on', () => {
    const { fixture, el, cmp } = setup('failed');
    cmp.draft.speed.enabled = true;
    cmp.draft.freeze.enabled = true;
    fixture.detectChanges();
    expect(el.textContent).toContain("Speed and freeze frame can't both be on");
    expect((el.querySelector('[data-testid="fx-save"]') as HTMLButtonElement).disabled).toBeTrue();
  });

  it('flags a blank number and blocks saving', () => {
    const { fixture, el, cmp } = setup('failed');
    cmp.addPunchIn();
    cmp.draft.zoom.punchIns[0].at = null as unknown as number;
    fixture.detectChanges();
    expect(el.textContent).toContain('Enter a start time in seconds.');
    expect((el.querySelector('[data-testid="fx-save"]') as HTMLButtonElement).disabled).toBeTrue();
  });

  it('locks the panel for a rendered job', () => {
    const { el } = setup('completed');
    expect(el.querySelector('[role="note"]')!.textContent).toContain('This job is locked.');
    expect((el.querySelector('fieldset') as HTMLFieldSetElement).disabled).toBeTrue();
  });
});
