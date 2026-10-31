import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { environment } from '../../../environments/environment';
import { SUPPRESS_GLOBAL_ERROR } from '../../core/interceptors/error.interceptor';
import { StoryPlanService } from './story-plan.service';

describe('StoryPlanService', () => {
  let service: StoryPlanService;
  let http: HttpTestingController;
  const base = environment.apiUrl;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    service = TestBed.inject(StoryPlanService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('reads the limits', () => {
    service.getLimits().subscribe();
    const req = http.expectOne(`${base}/story-plans/limits`);
    expect(req.request.method).toBe('GET');
    req.flush({});
  });

  it('plans from a story', () => {
    service.create('A short story about a lighthouse keeper.').subscribe();
    const req = http.expectOne(`${base}/story-plans`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({ story: 'A short story about a lighthouse keeper.' });
    req.flush({});
  });

  it('reads a plan back, escaping the id', () => {
    service.get('a/b c').subscribe();
    const req = http.expectOne(`${base}/story-plans/a%2Fb%20c`);
    expect(req.request.method).toBe('GET');
    req.flush({});
  });

  it('saves with the revision and one entry per scene', () => {
    service.save('p1', 3, [{ narration: 'New words' }, { media: { provider: 'pixabay', type: 'photo', id: '42' } }, { clearMedia: true }]).subscribe();
    const req = http.expectOne(`${base}/story-plans/p1`);
    expect(req.request.method).toBe('PUT');
    expect(req.request.body).toEqual({
      revision: 3,
      scenes: [{ narration: 'New words' }, { media: { provider: 'pixabay', type: 'photo', id: '42' } }, { clearMedia: true }],
    });
    req.flush({});
  });

  it('sends only the search parameters that were given', () => {
    service.search({ q: 'old lighthouse', type: 'photo' }).subscribe();
    const req = http.expectOne((r) => r.url === `${base}/stock-media/search`);
    expect(req.request.params.keys().sort()).toEqual(['q', 'type']);
    expect(req.request.params.get('q')).toBe('old lighthouse');
    expect(req.request.params.get('type')).toBe('photo');
    req.flush({});
  });

  it('passes paging, provider and orientation through', () => {
    service.search({ q: 'sea', type: 'video', provider: 'pixabay', orientation: 'landscape', page: 2, perPage: 12 }).subscribe();
    const req = http.expectOne((r) => r.url === `${base}/stock-media/search`);
    expect(req.request.params.get('provider')).toBe('pixabay');
    expect(req.request.params.get('orientation')).toBe('landscape');
    expect(req.request.params.get('page')).toBe('2');
    expect(req.request.params.get('perPage')).toBe('12');
    req.flush({});
  });

  it('never puts a provider key or provider host in a request', () => {
    service.search({ q: 'sea', type: 'photo' }).subscribe();
    service.get('p1').subscribe();
    for (const req of http.match(() => true)) {
      expect(req.request.url.startsWith(base)).toBeTrue();
      expect(req.request.urlWithParams).not.toMatch(/pexels|pixabay|key=/i);
      req.flush({});
    }
  });

  it('keeps the global error toast quiet and sends cookies, on every call', () => {
    service.getLimits().subscribe({ error: () => undefined });
    service.create('x').subscribe({ error: () => undefined });
    service.get('p1').subscribe({ error: () => undefined });
    service.save('p1', 1, []).subscribe({ error: () => undefined });
    service.search({ q: 'x', type: 'photo' }).subscribe({ error: () => undefined });
    const reqs = http.match(() => true);
    expect(reqs.length).toBe(5);
    for (const req of reqs) {
      expect(req.request.context.get(SUPPRESS_GLOBAL_ERROR)).toBeTrue();
      expect(req.request.withCredentials).toBeTrue();
      req.flush({}, { status: 500, statusText: 'Server Error' });
    }
  });
});
