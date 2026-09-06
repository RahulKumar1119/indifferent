import { ApplicationConfig, provideBrowserGlobalErrorListeners, provideZoneChangeDetection, APP_INITIALIZER, inject } from '@angular/core';
import { provideRouter } from '@angular/router';
import { provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';
import { firstValueFrom, catchError, of } from 'rxjs';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { provideLucideIcons, LucideVideo, LucideWand2, LucidePalette, LucideMic, LucideZap, LucideUpload, LucidePlay, LucideDownload, LucideSettings, LucideUser, LucideHelpCircle, LucideLogOut, LucidePlus, LucideArrowRight, LucideCheck, LucideX, LucideLoader2, LucideMoon, LucideSun, LucideHome, LucideFolderOpen, LucideChevronRight, LucideCircleCheck, LucideCircleX, LucideClock, LucideFileText } from '@lucide/angular';

import { routes } from './app.routes';
import { AUTH_INTERCEPTOR_PROVIDER, ERROR_INTERCEPTOR_PROVIDER } from './core';
import { AuthService } from './core/services/auth.service';

export function initAuthFactory(): () => Promise<unknown> {
  const auth = inject(AuthService);
  return () => firstValueFrom(auth.initialize().pipe(catchError(() => of(null))));
}

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideZoneChangeDetection({ eventCoalescing: true }),
    provideRouter(routes),
    provideHttpClient(withInterceptorsFromDi()),
    provideAnimationsAsync(),
    AUTH_INTERCEPTOR_PROVIDER,
    ERROR_INTERCEPTOR_PROVIDER,
    { provide: APP_INITIALIZER, multi: true, useFactory: initAuthFactory },
    provideLucideIcons(LucideVideo, LucideWand2, LucidePalette, LucideMic, LucideZap, LucideUpload, LucidePlay, LucideDownload, LucideSettings, LucideUser, LucideHelpCircle, LucideLogOut, LucidePlus, LucideArrowRight, LucideCheck, LucideX, LucideLoader2, LucideMoon, LucideSun, LucideHome, LucideFolderOpen, LucideChevronRight, LucideCircleCheck, LucideCircleX, LucideClock, LucideFileText),
  ],
};
