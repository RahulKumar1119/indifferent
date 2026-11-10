import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { LucideClock, LucideDownload, LucideLoader2, LucideMoon, LucideSun, LucideUser, provideLucideIcons } from '@lucide/angular';
import { Observable, Subject, of, throwError } from 'rxjs';
import { PreviewComponent } from './preview.component';
import { ProjectService } from '../../core/services/project.service';
import { ApiService } from '../../core/services/api.service';
import { Project } from '../../shared/models/project.model';

const ready = {
  id: 'p1', name: 'Capitals', status: 'completed', template: 'classic', voice: 'Joanna',
  createdAt: '2026-11-08T00:00:00Z', completedAt: '2026-11-09T00:00:00Z',
  videoUrl: 'https://example.com/v.mp4', thumbnailUrl: 'https://example.com/t.jpg',
} as unknown as Project;

describe('PreviewComponent (#251)', () => {
  function open(source: Observable<Project>, download: Observable<unknown> = of({})) {
    const api = { get: jasmine.createSpy('get').and.returnValue(download) };
    TestBed.configureTestingModule({
      imports: [PreviewComponent],
      providers: [
        provideRouter([]),
        provideLucideIcons(LucideClock, LucideDownload, LucideLoader2, LucideSun, LucideMoon, LucideUser),
        { provide: ActivatedRoute, useValue: { snapshot: { paramMap: convertToParamMap({ id: 'p1' }) } } },
        { provide: ProjectService, useValue: { getProject: () => source } },
        { provide: ApiService, useValue: api },
      ],
    });
    const fixture = TestBed.createComponent(PreviewComponent);
    fixture.detectChanges();
    return { fixture, api, el: fixture.nativeElement as HTMLElement };
  }

  it('shows a loading state until the project arrives', () => {
    const pending = new Subject<Project>();
    const { fixture, el } = open(pending);
    expect(el.querySelector('[data-testid="loading"]')).toBeTruthy();
    pending.next(ready);
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="loading"]')).toBeNull();
  });

  it('puts the player first, then Download, then the thumbnail', () => {
    const { el } = open(of(ready));
    const video = el.querySelector('video')!;
    const download = el.querySelector('[data-testid="download"]')!;
    const thumb = el.querySelector('img')!;
    expect(video.getAttribute('src')).toBe('https://example.com/v.mp4');
    expect(video.compareDocumentPosition(download) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(download.compareDocumentPosition(thumb) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(download.textContent).toContain('Download video');
    expect(el.querySelectorAll('h1').length).toBe(1);
    expect(el.querySelector('h1')!.textContent).toContain('Capitals');
  });

  it('says when a video is not ready, in plain words, with a link to its progress', () => {
    const { el } = open(of({ ...ready, status: 'rendering' } as Project));
    const box = el.querySelector('[data-testid="not-ready"]')!;
    expect(box.querySelector('h1')!.textContent).toContain('not ready yet');
    expect(box.textContent).toContain('Status: Rendering');
    expect(box.querySelector('a')!.getAttribute('href')).toBe('/projects/p1/progress');
    expect(el.querySelector('video')).toBeNull();
  });

  it('says the video was not found when the project cannot load', () => {
    const { el } = open(throwError(() => new Error('404')));
    const box = el.querySelector('[data-testid="not-found"]')!;
    expect(box.getAttribute('role')).toBe('alert');
    expect(box.querySelector('h1')!.textContent).toContain('Video not found');
  });

  it('asks the same download endpoint, and announces a video that is being restored', () => {
    const { fixture, el, api } = open(of(ready), of({ code: 'RESTORING', message: 'Being restored. Check back later.' }));
    (el.querySelector('[data-testid="download"]') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(api.get).toHaveBeenCalledWith('/projects/p1/download');
    const notice = el.querySelector('[data-testid="notice"]')!;
    expect(notice.getAttribute('role')).toBe('status');
    expect(notice.textContent).toContain('Being restored. Check back later.');
  });
});
