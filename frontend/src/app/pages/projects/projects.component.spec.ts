import { TestBed } from '@angular/core/testing';
import { provideLucideIcons, LucideSun, LucideMoon, LucideUser, LucidePlus, LucideTriangleAlert, LucideFileText, LucideChevronRight } from '@lucide/angular';
import { provideRouter } from '@angular/router';
import { Observable, Subject, of, throwError } from 'rxjs';
import { ProjectsComponent } from './projects.component';
import { ProjectService } from '../../core/services/project.service';
import { Project } from '../../shared/models/project.model';

const project = (id: string, status: string, createdAt: string): Project =>
  ({ id, name: 'Quiz ' + id, status, template: 'classic', voice: 'Joanna', createdAt }) as unknown as Project;

describe('ProjectsComponent (#251)', () => {
  function open(source: Observable<Project[]>) {
    TestBed.configureTestingModule({
      imports: [ProjectsComponent],
      providers: [provideRouter([]), provideLucideIcons(LucideSun, LucideMoon, LucideUser, LucidePlus, LucideTriangleAlert, LucideFileText, LucideChevronRight), { provide: ProjectService, useValue: { getProjects: () => source } }],
    });
    const fixture = TestBed.createComponent(ProjectsComponent);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('shows a loading state until the list arrives', () => {
    const pending = new Subject<Project[]>();
    const { fixture, el } = open(pending);
    expect(el.querySelector('[data-testid="loading"]')).toBeTruthy();
    pending.next([]);
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="loading"]')).toBeNull();
  });

  it('offers the first quiz video when there are none', () => {
    const { el } = open(of([]));
    const empty = el.querySelector('[data-testid="empty"]')!;
    expect(empty.textContent).toContain('No quiz videos yet.');
    expect(empty.querySelector('a')!.getAttribute('href')).toBe('/projects/new');
  });

  it('shows an error with a retry that loads again', () => {
    let calls = 0;
    TestBed.configureTestingModule({
      imports: [ProjectsComponent],
      providers: [
        provideRouter([]),
        provideLucideIcons(LucideSun, LucideMoon, LucideUser, LucidePlus, LucideTriangleAlert, LucideFileText, LucideChevronRight),
        { provide: ProjectService, useValue: { getProjects: () => (++calls === 1 ? throwError(() => new Error('down')) : of([])) } },
      ],
    });
    const fixture = TestBed.createComponent(ProjectsComponent);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const err = el.querySelector('[data-testid="load-error"]')!;
    expect(err.getAttribute('role')).toBe('alert');
    (err.querySelector('button') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(calls).toBe(2);
    expect(el.querySelector('[data-testid="empty"]')).toBeTruthy();
  });

  it('lists projects newest first as links, with plain status words', () => {
    const { el } = open(of([project('a', 'completed', '2026-10-01T00:00:00Z'), project('b', 'created', '2026-10-20T00:00:00Z')]));
    const rows = Array.from(el.querySelectorAll('li[data-testid^="project-"]'));
    expect(rows.map((r) => r.getAttribute('data-testid'))).toEqual(['project-b', 'project-a']);
    expect(rows[0].querySelector('a')!.getAttribute('href')).toBe('/projects/b');
    expect(rows[0].querySelector('.ui-pill')!.textContent!.trim()).toBe('Needs questions');
    expect(rows[1].querySelector('.ui-pill')!.textContent!.trim()).toBe('Ready');
    expect(el.querySelectorAll('h1').length).toBe(1);
    expect(el.textContent).not.toContain('\u2014');
  });
});
