import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { DEFAULT_SHORTS_LIMITS, ShortsService, minSourceSeconds, tooShortMessage } from './shorts.service';
import { environment } from '../../../environments/environment';

describe('ShortsService.createJob', () => {
  let service: ShortsService;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    service = TestBed.inject(ShortsService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('sends the file size so the API can sign it into the upload URL', () => {
    service.createJob('mp4', 60, 'auto', 1048576).subscribe();
    const req = http.expectOne(`${environment.apiUrl}/shorts`);
    expect(req.request.body).toEqual({ fileType: 'mp4', duration: 60, language: 'auto', size: 1048576 });
    req.flush({});
  });

  it('leaves size out when it is not known', () => {
    service.createJob('mp4', 60).subscribe();
    const req = http.expectOne(`${environment.apiUrl}/shorts`);
    expect(req.request.body).toEqual({ fileType: 'mp4', duration: 60, language: 'auto' });
    req.flush({});
  });
});

// Issue #286: the shortest source the API accepts.
describe('Shorts minimum length', () => {
  it('uses the minimum the server sends, and 30 seconds when it is missing or unusable', () => {
    expect(minSourceSeconds({ minDurationSeconds: 45 })).toBe(45);
    for (const bad of [undefined, 0, -5, NaN, Infinity]) {
      expect(minSourceSeconds({ minDurationSeconds: bad })).toBe(30);
    }
    expect(minSourceSeconds({})).toBe(30);
    expect(DEFAULT_SHORTS_LIMITS.minDurationSeconds).toBe(30);
  });

  it('accepts exactly the minimum and refuses anything shorter, never rounding up to it', () => {
    expect(tooShortMessage(30, {})).toBe('');
    expect(tooShortMessage(3600, {})).toBe('');
    expect(tooShortMessage(10, {})).toBe('This file is 10 seconds long. AI Shorts needs at least 30 seconds to cut a clip from.');
    expect(tooShortMessage(29.9, {})).toContain('29 seconds long');
    expect(tooShortMessage(1.4, {})).toContain('1 second long');
    expect(tooShortMessage(0.3, {})).toContain('under 1 second long');
    expect(tooShortMessage(40, { minDurationSeconds: 45 })).toContain('at least 45 seconds');
  });
});
