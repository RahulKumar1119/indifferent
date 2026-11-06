import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { ProjectService } from './project.service';
import { environment } from '../../../environments/environment';

describe('ProjectService upload requests', () => {
  let service: ProjectService;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    service = TestBed.inject(ProjectService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('asks for a logo upload URL with the logo size', () => {
    service.logoUploadUrl('p1', 2048).subscribe();
    const req = http.expectOne(`${environment.apiUrl}/projects/p1/logo`);
    expect(req.request.body).toEqual({ size: 2048 });
    req.flush({ uploadUrl: 'u', logoKey: 'k' });
  });

  it('asks for a text upload URL with the file size', () => {
    const file = new File(['hello'], 'a.txt', { type: 'text/plain' });
    service.uploadFile('p1', file).subscribe();
    const req = http.expectOne(`${environment.apiUrl}/projects/p1/upload`);
    expect(req.request.body.size).toBe(5);
    req.flush({ uploadUrl: 'https://s3.example/put' });
    http.expectOne('https://s3.example/put').flush(null);
  });
});
