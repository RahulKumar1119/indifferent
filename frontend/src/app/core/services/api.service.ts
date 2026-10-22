import { Injectable } from '@angular/core';
import { HttpClient, HttpContext, HttpHeaders, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly baseUrl = environment.apiUrl;

  constructor(private readonly http: HttpClient) {}

  get<T>(
    path: string,
    params?: Record<string, string | number | boolean>,
    withCredentials = true,
    context?: HttpContext,
  ): Observable<T> {
    let httpParams = new HttpParams();
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        httpParams = httpParams.set(key, String(value));
      });
    }
    return this.http.get<T>(`${this.baseUrl}${path}`, { params: httpParams, withCredentials, context });
  }

  post<T>(path: string, body: unknown = {}, withCredentials = true): Observable<T> {
    return this.http.post<T>(`${this.baseUrl}${path}`, body, { withCredentials });
  }

  /** POST for binary responses such as the rendered slide preview PNG. */
  postBlob(path: string, body: unknown = {}): Observable<Blob> {
    return this.http.post(`${this.baseUrl}${path}`, body, {
      withCredentials: true,
      responseType: 'blob',
      headers: new HttpHeaders({ Accept: 'image/png' }),
    });
  }

  put<T>(path: string, body: unknown = {}, withCredentials = true, context?: HttpContext): Observable<T> {
    return this.http.put<T>(`${this.baseUrl}${path}`, body, { withCredentials, context });
  }

  /** PUT to an absolute URL (e.g. S3 presigned URL): no cookies/auth headers. */
  putAbsolute<T>(url: string, body: unknown): Observable<T> {
    return this.http.put<T>(url, body, { withCredentials: false });
  }

  delete<T>(path: string): Observable<T> {
    return this.http.delete<T>(`${this.baseUrl}${path}`, { withCredentials: true });
  }
}
