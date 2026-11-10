import { TestBed } from '@angular/core/testing';
import { provideLucideIcons, LucideSun, LucideMoon, LucideUser, LucideScissors, LucideFileText, LucideClapperboard, LucideFilm, LucideStamp } from '@lucide/angular';
import { provideRouter } from '@angular/router';
import { NewComponent } from './new.component';

describe('NewComponent (#251)', () => {
  function open() {
    TestBed.configureTestingModule({ imports: [NewComponent], providers: [provideRouter([]), provideLucideIcons(LucideSun, LucideMoon, LucideUser, LucideScissors, LucideFileText, LucideClapperboard, LucideFilm, LucideStamp)] });
    const fixture = TestBed.createComponent(NewComponent);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('links the four tools', () => {
    const el = open();
    const href = (id: string) => el.querySelector(`[data-testid="${id}"] a.cta`)!.getAttribute('href');
    expect(href('tool-shorts')).toBe('/shorts');
    expect(href('tool-quiz')).toBe('/projects/new');
    expect(href('tool-story')).toBe('/story');
    expect(href('tool-watermark')).toBe('/tools/add-watermark');
    expect(el.querySelectorAll('h1').length).toBe(1);
  });

  it('says what a quiz video needs before anyone starts one, with the sample and the guide', () => {
    const el = open();
    const needs = el.querySelector('[data-testid="quiz-needs"]')!;
    expect(needs.textContent).toContain("You'll need a TXT file of questions.");
    expect(needs.querySelector('a[download="quiz-sample.txt"]')!.getAttribute('href')).toBe('/samples/quiz-sample.txt');
    expect(needs.querySelector('a[href="/blog/quiz-file-format-guide"]')).toBeTruthy();
  });

  it('points people with a story or notes to Story video', () => {
    const story = open().querySelector('[data-testid="tool-story"]')!;
    expect(story.textContent).toContain('story, a script or notes instead of quiz questions');
    expect(story.querySelector('a.cta')!.textContent).toContain('Use Story video');
  });
});
