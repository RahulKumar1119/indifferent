import { Injectable } from '@angular/core';
import {
  HttpContextToken,
  HttpEvent,
  HttpHandler,
  HttpInterceptor,
  HttpRequest,
  HttpErrorResponse,
  HTTP_INTERCEPTORS,
} from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { ErrorService } from '../services/error.service';

/**
 * Opt out of the global error toast for requests whose failures are handled
 * locally (polling loops, expected 404s): pass a context with this token set.
 * Usage: `api.get(path, params, true, new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true))`.
 */
export const SUPPRESS_GLOBAL_ERROR = new HttpContextToken<boolean>(() => false);

@Injectable()
export class ErrorInterceptor implements HttpInterceptor {
  constructor(private readonly errorService: ErrorService) {}

  intercept(req: HttpRequest<unknown>, next: HttpHandler): Observable<HttpEvent<unknown>> {
    return next.handle(req).pipe(
      catchError((error) => {
        if (error instanceof HttpErrorResponse) {
          // Skip 401 errors: handled by AuthInterceptor. Skip requests that
          // handle their own failures (polling, expected 404s).
          if (error.status !== 401 && !req.context.get(SUPPRESS_GLOBAL_ERROR)) {
            this.errorService.handleError(error);
          }
        }
        return throwError(() => error);
      }),
    );
  }
}

export const ERROR_INTERCEPTOR_PROVIDER = {
  provide: HTTP_INTERCEPTORS,
  useClass: ErrorInterceptor,
  multi: true,
};
