import { provideRouter } from '@angular/router';
import { TestBed } from '@angular/core/testing';
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
      'How it works',
      'Supported formats',
      'Video specifications',
      'Six designs, six looks',
      'Five narrator voices',
      'Who uses it',
      'Built with',
    ]);

    expect(page.textContent).toContain(
      'Indifferent turns multiple-choice quiz files into engaging, YouTube-ready films.',
    );
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
