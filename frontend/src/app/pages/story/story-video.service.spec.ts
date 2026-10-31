import { TestBed } from '@angular/core/testing';
import { Observable } from 'rxjs';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { environment } from '../../../environments/environment';
import { SUPPRESS_GLOBAL_ERROR } from '../../core/interceptors/error.interceptor';
import { StoryVideoService } from './story-video.service';

describe('StoryVideoService', () => {
  let service: StoryVideoService;
  let http: HttpTestingController;
  const base = environment.apiUrl;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    service = TestBed.inject(StoryVideoService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('reads the limits', () => {
    service.getLimits().subscribe();
    const req = http.expectOne(`${base}/story-videos/limits`);
    expect(req.request.method).toBe('GET');
    req.flush({});
  });

  it('starts a video from the plan, its revision, the voice and the caption look', () => {
    service.create({ planId: 'p1', revision: 4, voice: 'Matthew', caption: { preset: 'karaoke' } }).subscribe();
    const req = http.expectOne(`${base}/story-videos`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({ planId: 'p1', revision: 4, voice: 'Matthew', caption: { preset: 'karaoke' } });
    req.flush({});
  });

  it('reads one video, escaping the id', () => {
    service.get('a/b c').subscribe();
    const req = http.expectOne(`${base}/story-videos/a%2Fb%20c`);
    expect(req.request.method).toBe('GET');
    req.flush({});
  });

  it('lists the videos of one plan', () => {
    service.listForPlan('plan 1').subscribe();
    const req = http.expectOne((r) => r.url === `${base}/story-videos`);
    expect(req.request.method).toBe('GET');
    expect(req.request.params.get('planId')).toBe('plan 1');
    req.flush({ videos: [], hasMore: false });
  });

  it('cancels with a POST', () => {
    service.cancel('j1').subscribe();
    const req = http.expectOne(`${base}/story-videos/j1/cancel`);
    expect(req.request.method).toBe('POST');
    req.flush({});
  });

  it('removes with a DELETE', () => {
    service.remove('j1').subscribe();
    const req = http.expectOne(`${base}/story-videos/j1`);
    expect(req.request.method).toBe('DELETE');
    req.flush(null);
  });

  it('asks for the download link of one video', () => {
    service.download('j1').subscribe();
    const req = http.expectOne(`${base}/story-videos/j1/download`);
    expect(req.request.method).toBe('GET');
    req.flush({});
  });

  it('keeps the global error toast out of every request, since the page reports failures itself', () => {
    const calls: (() => Observable<unknown>)[] = [
      () => service.getLimits(),
      () => service.create({ planId: 'p', revision: 1 }),
      () => service.get('j'),
      () => service.listForPlan('p'),
      () => service.cancel('j'),
      () => service.remove('j'),
      () => service.download('j'),
    ];
    for (const call of calls) {
      call().subscribe({ error: () => undefined });
      const req = http.expectOne(() => true);
      expect(req.request.context.get(SUPPRESS_GLOBAL_ERROR)).withContext(req.request.url).toBeTrue();
      req.flush(null);
    }
  });
});
