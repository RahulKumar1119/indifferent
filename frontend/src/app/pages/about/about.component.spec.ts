import { provideRouter } from '@angular/router';
import { TestBed } from '@angular/core/testing';
import { PRODUCT_FACTS, joinAnd, shortsLengthLabel } from '../../core/config/product-facts';
import { AboutComponent } from './about.component';

describe('AboutComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AboutComponent],
      providers: [provideRouter([])],
    }).compileComponents();
  });

  function render(): HTMLElement {
    const fixture = TestBed.createComponent(AboutComponent);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('presents the studio, process and design catalogue as semantic sections', () => {
    const page = render();
    expect(page.querySelectorAll('h1').length).toBe(1);
    expect(page.querySelector('h1')?.textContent?.trim()).toBe('The studio behind the video.');
    expect(Array.from(page.querySelectorAll('h2')).map((h) => h.textContent?.trim())).toEqual([
      'Our mission',
      'Four tools, one studio',
      'How a quiz film is made',
      'Quiz file formats',
      'Quiz film specifications',
      'Six designs, six looks',
      'Five narrator voices',
      'Who uses it',
      'Built with',
    ]);

    expect(page.textContent).toContain('Indifferent is a small studio of four video tools');
    expect(page.textContent).toContain(
      'A design sets the palette, type scale and layout together',
    );
    expect(page.querySelectorAll('ol > li').length).toBe(4);
    expect(page.querySelectorAll('img[src^="/templates/"]').length).toBe(6);
    expect(page.querySelectorAll('img[src^="/templates/"]')[0]?.getAttribute('alt')).toContain(
      'Classic design preview',
    );
    const voiceNames = Array.from(page.querySelectorAll('li .font-semibold')).map((el) =>
      el.textContent?.trim(),
    );
    for (const voice of ['Joanna', 'Matthew', 'Amy', 'Brian', 'Aditi']) {
      expect(voiceNames).toContain(voice);
    }
  });

  it('introduces all four tools, each linked to its own page and quoting the shared product facts', () => {
    const page = render();
    const section = page.querySelector('#tools-heading')!.closest('section')!;
    const hrefs = Array.from(section.querySelectorAll('a')).map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual([
      '/features/txt-to-video-quiz',
      '/features/ai-shorts',
      '/features/story-video',
      '/features/watermark',
    ]);
    const text = section.textContent!;
    const t = PRODUCT_FACTS.story;
    expect(text).toContain(`${t.minScenes} to ${t.maxScenes} scenes`);
    expect(text).toContain(`up to ${t.maxTotalSeconds} seconds`);
    expect(text).toContain(joinAnd(t.stockLibraries));
    expect(text).toContain(`of ${shortsLengthLabel()}`);
    expect(text).toContain(`up to ${PRODUCT_FACTS.shorts.maxClips} ranked vertical clips`);
    expect(text).toContain('nothing is uploaded');
  });

  it('does not describe the whole studio as a quiz tool, and uses no em dashes', () => {
    const text = render().textContent!;
    expect(text).not.toContain('YouTube-ready');
    expect(text).not.toContain('\u2014');
  });

  it('keeps navigation and the issue link named and keyboard reachable', () => {
    const page = render();
    const links = Array.from(page.querySelectorAll('a'));

    for (const link of links) {
      const name =
        link.textContent?.trim() || link.querySelector('img')?.getAttribute('alt') || '';
      expect(name).withContext(`Unnamed link: ${link.outerHTML}`).toBeTruthy();
      expect(link.tabIndex).not.toBe(-1);
    }

    const github = links.find((link) => link.textContent?.includes('Open an issue'));
    expect(github?.getAttribute('href')).toBe(
      'https://github.com/RahulKumar1119/indifferent/issues',
    );
    expect(github?.getAttribute('target')).toBe('_blank');
    expect(github?.getAttribute('rel')?.split(/\s+/)).toContain('noopener');
    expect(links.some((link) => link.textContent?.includes('Start creating'))).toBeTrue();
  });
});
