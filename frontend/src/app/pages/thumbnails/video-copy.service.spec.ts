import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { environment } from '../../../environments/environment';
import { SUPPRESS_GLOBAL_ERROR } from '../../core/interceptors/error.interceptor';
import { VideoCopyService } from './video-copy.service';

describe('VideoCopyService', () => {
  let service: VideoCopyService;
  let http: HttpTestingController;
  const base = environment.apiUrl;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    service = TestBed.inject(VideoCopyService);
    http = TestBed.inject(HttpTestingController);
  });
  afterEach(() => http.verify());

  it('reads the status of a job, escaping the id', () => {
    service.status('a/b c').subscribe();
    const req = http.expectOne(`${base}/shorts/a%2Fb%20c/copy/status`);
    expect(req.request.method).toBe('GET');
    req.flush({});
  });

  it('writes with exactly the body it was given', () => {
    const body = { topic: 'A video about a lighthouse keeper', refine: 'shorter', previousTitles: ['One'] };
    service.write('j1', body).subscribe();
    const req = http.expectOne(`${base}/shorts/j1/copy`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual(body);
    req.flush({});
  });

  it('sends the sign-in cookie and keeps the global error toast quiet, since the page explains every failure', () => {
    service.status('j1').subscribe({ error: () => undefined });
    service.write('j1', { useJobTranscript: true }).subscribe({ error: () => undefined });
    const reqs = http.match(() => true);
    expect(reqs.length).toBe(2);
    for (const req of reqs) {
      expect(req.request.context.get(SUPPRESS_GLOBAL_ERROR)).toBeTrue();
      expect(req.request.withCredentials).toBeTrue();
      req.flush({}, { status: 500, statusText: 'Server Error' });
    }
  });
});
