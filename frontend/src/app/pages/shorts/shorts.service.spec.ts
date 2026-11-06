import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { ShortsService } from './shorts.service';
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
