import { TestBed } from '@angular/core/testing';
import { provideLucideIcons, LucideSun, LucideMoon, LucideUser, LucideTriangleAlert, LucidePlay, LucideUpload } from '@lucide/angular';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { ProjectDetailComponent } from './project-detail.component';
import { ProjectService } from '../../core/services/project.service';
import { ShortsService } from '../shorts/shorts.service';

describe('ProjectDetailComponent: the next step (#274)', () => {
  function open(status: string) {
    TestBed.configureTestingModule({
      imports: [ProjectDetailComponent],
      providers: [
        provideRouter([]),
        provideLucideIcons(LucideSun, LucideMoon, LucideUser, LucideTriangleAlert, LucidePlay, LucideUpload),
        { provide: ActivatedRoute, useValue: { snapshot: { paramMap: convertToParamMap({ id: 'p1' }) } } },
        { provide: ProjectService, useValue: { getProject: () => of({ id: 'p1', name: 'Capitals', status, template: 'classic', voice: 'Joanna', createdAt: '2026-11-09T00:00:00Z' }), logoUrl: () => of({}) } },
        { provide: ShortsService, useValue: { getStatus: () => of({}), listJobs: () => of([]) } },
      ],
    });
    const fixture = TestBed.createComponent(ProjectDetailComponent);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('makes uploading the questions the main action while nothing is uploaded', () => {
    const el = open('created');
    const next = el.querySelector('[data-testid="next-upload"]')!;
    expect(next.textContent).toContain('Next: upload your questions.');
    expect(next.querySelector('a')!.getAttribute('href')).toBe('/projects/p1/upload');
  });

  it('drops the prompt once the project has started', () => {
    for (const status of ['parsing', 'completed', 'failed']) {
      TestBed.resetTestingModule();
      expect(open(status).querySelector('[data-testid="next-upload"]')).withContext(status).toBeNull();
    }
  });
});
