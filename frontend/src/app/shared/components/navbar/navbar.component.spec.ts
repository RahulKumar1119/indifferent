import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { LucideMoon, LucideSun, LucideUser, provideLucideIcons } from '@lucide/angular';
import { NavbarComponent } from './navbar.component';

function render(): HTMLElement {
  TestBed.configureTestingModule({
    imports: [NavbarComponent],
    providers: [provideRouter([]), provideLucideIcons(LucideUser, LucideSun, LucideMoon)],
  });
  const fixture = TestBed.createComponent(NavbarComponent);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('NavbarComponent', () => {
  it('links to every signed-in section, including the Story tool', () => {
    const links = Array.from(render().querySelectorAll('nav a'));
    const hrefs = links.map((a) => a.getAttribute('href'));
    // New opens the tool picker (#256); Shorts is one of the tools on it.
    expect(links.find((a) => a.textContent!.trim() === 'New')!.getAttribute('href')).toBe('/new');
    expect(hrefs).toContain('/story');
    expect(hrefs).toContain('/shorts/history');
    expect(hrefs).toContain('/dashboard');
  });

  it('labels the Story link so it is not mistaken for the Shorts one', () => {
    const story = Array.from(render().querySelectorAll('nav a')).find((a) => a.getAttribute('href') === '/story');
    expect(story?.textContent?.trim()).toBe('Story');
  });
});

describe('NavbarComponent controls', () => {
  it('names the theme toggle for what it will do and keeps the brand linked home', () => {
    const el = render();
    expect(el.querySelector('button')?.getAttribute('aria-label')).toMatch(/^Switch to (light|dark) mode$/);
    expect(el.querySelector('a.brand')?.getAttribute('href')).toBe('/');
  });
});
