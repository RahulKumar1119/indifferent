import { ApplicationConfig, provideBrowserGlobalErrorListeners, provideZoneChangeDetection, isDevMode } from '@angular/core';
import { provideRouter } from '@angular/router';
import { provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { provideServiceWorker } from '@angular/service-worker';
import { provideLucideIcons, LucideVideo, LucideWand2, LucidePalette, LucideMic, LucideZap, LucideUpload, LucidePlay, LucideDownload, LucideSettings, LucideUser, LucideHelpCircle, LucideLogOut, LucidePlus, LucideArrowRight, LucideCheck, LucideX, LucideLoader2, LucideMoon, LucideSun, LucideHome, LucideFolderOpen, LucideChevronRight, LucideCircleCheck, LucideCircleX, LucideClock, LucideFileText } from '@lucide/angular';

import { routes } from './app.routes';
import { AUTH_INTERCEPTOR_PROVIDER, ERROR_INTERCEPTOR_PROVIDER } from './core';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideZoneChangeDetection({ eventCoalescing: true }),
    provideRouter(routes),
    provideHttpClient(withInterceptorsFromDi()),
    provideAnimationsAsync(),
    provideServiceWorker('ngsw-worker.js', {
      enabled: !isDevMode(),
      registrationStrategy: 'registerWhenStable:30000',
    }),
    AUTH_INTERCEPTOR_PROVIDER,
    ERROR_INTERCEPTOR_PROVIDER,
    provideLucideIcons(LucideVideo, LucideWand2, LucidePalette, LucideMic, LucideZap, LucideUpload, LucidePlay, LucideDownload, LucideSettings, LucideUser, LucideHelpCircle, LucideLogOut, LucidePlus, LucideArrowRight, LucideCheck, LucideX, LucideLoader2, LucideMoon, LucideSun, LucideHome, LucideFolderOpen, LucideChevronRight, LucideCircleCheck, LucideCircleX, LucideClock, LucideFileText),
  ],
};
