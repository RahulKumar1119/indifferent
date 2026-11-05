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
    const hrefs = Array.from(render().querySelectorAll('nav a')).map((a) => a.getAttribute('href'));
    expect(hrefs).toContain('/shorts');
    expect(hrefs).toContain('/story');
    expect(hrefs).toContain('/shorts/history');
    expect(hrefs).toContain('/dashboard');
  });

  it('labels the Story link so it is not mistaken for the Shorts one', () => {
    const story = Array.from(render().querySelectorAll('nav a')).find((a) => a.getAttribute('href') === '/story');
    expect(story?.textContent?.trim()).toBe('Story');
  });
});
