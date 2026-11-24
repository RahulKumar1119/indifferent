import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { environment } from '../../../environments/environment';
import { SUPPRESS_GLOBAL_ERROR } from '../../core/interceptors/error.interceptor';
import { PostCopyService, postCopyPath } from './post-copy.service';

describe('PostCopyService', () => {
  let service: PostCopyService;
  let http: HttpTestingController;
  const base = environment.apiUrl;
  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    service = TestBed.inject(PostCopyService);
    http = TestBed.inject(HttpTestingController);
  });
  afterEach(() => http.verify());

  it('builds the path for a clip and for a story video, escaping the ids', () => {
    expect(postCopyPath({ kind: 'clip', jobId: 'j1', clipId: 'clip-1' })).toBe('/shorts/j1/clips/clip-1/post-copy');
    expect(postCopyPath({ kind: 'clip', jobId: 'a/b', clipId: 'c d' })).toBe('/shorts/a%2Fb/clips/c%20d/post-copy');
    expect(postCopyPath({ kind: 'story', videoId: 'abc' })).toBe('/story-videos/abc/post-copy');
  });

  it('reads one clip, and the list for a job', () => {
    service.view({ kind: 'clip', jobId: 'j1', clipId: 'c1' }).subscribe();
    http.expectOne(`${base}/shorts/j1/clips/c1/post-copy`).flush({});
    service.list('j1').subscribe();
    const req = http.expectOne(`${base}/shorts/j1/post-copy`);
    expect(req.request.method).toBe('GET');
    req.flush({});
  });

  it('writes with only a request id', () => {
    service.write({ kind: 'story', videoId: 'v1' }, 'request-id-0123456789').subscribe();
    const req = http.expectOne(`${base}/story-videos/v1/post-copy`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({ requestId: 'request-id-0123456789' });
    req.flush({});
  });

  it('saves exactly the copy it was given', () => {
    const copy = { youtube: { title: 'T', text: 'x', hashtags: ['#a1'] } };
    service.save({ kind: 'clip', jobId: 'j1', clipId: 'c1' }, copy).subscribe();
    const req = http.expectOne(`${base}/shorts/j1/clips/c1/post-copy`);
    expect(req.request.method).toBe('PUT');
    expect(req.request.body).toEqual({ copy });
    req.flush({});
  });

  it('sends the sign-in cookie and keeps the global error toast quiet', () => {
    service.view({ kind: 'story', videoId: 'v1' }).subscribe({ error: () => undefined });
    service.list('j1').subscribe({ error: () => undefined });
    service.write({ kind: 'story', videoId: 'v1' }, 'request-id-0123456789').subscribe({ error: () => undefined });
    service.save({ kind: 'story', videoId: 'v1' }, {}).subscribe({ error: () => undefined });
    const reqs = http.match(() => true);
    expect(reqs.length).toBe(4);
    for (const r of reqs) {
      expect(r.request.withCredentials).toBeTrue();
      expect(r.request.context.get(SUPPRESS_GLOBAL_ERROR)).toBeTrue();
      r.flush({});
    }
  });
});
