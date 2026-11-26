import { ApplicationConfig, provideBrowserGlobalErrorListeners, provideZoneChangeDetection, APP_INITIALIZER, inject } from '@angular/core';
import { UrlSerializer, provideRouter, withInMemoryScrolling } from '@angular/router';
import { TrailingSlashUrlSerializer } from './shared/site-url';
import { provideHttpClient, withFetch, withInterceptorsFromDi } from '@angular/common/http';
import { firstValueFrom, catchError, of } from 'rxjs';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { provideLucideIcons, LucideVolume2, LucideSquare, LucideVideo, LucideWand2, LucidePalette, LucideMic, LucideZap, LucideUpload, LucidePlay, LucideDownload, LucideSettings, LucideUser, LucideHelpCircle, LucideLogOut, LucidePlus, LucideArrowRight, LucideCheck, LucideX, LucideLoader2, LucideMoon, LucideSun, LucideHome, LucideFolderOpen, LucideChevronRight, LucideCircleCheck, LucideCircleX, LucideClock, LucideFileText, LucideScissors, LucideStamp, LucideCaptions, LucideFocus, LucideFrame, LucideGauge, LucideShare2, LucideCopy, LucideClapperboard, LucideSparkles, LucidePencil, LucideImage, LucideStar, LucideSquarePlay, LucideCamera, LucideMusic2, LucideTrash2, LucideMoreVertical, LucideCreditCard, LucideArrowUpRight, LucideRefreshCw, LucideFilm, LucideSearch, LucideTriangleAlert, LucideLayoutDashboard, LucidePause, LucideLock, LucideShieldCheck, LucideUserX, LucideRotateCcw, LucideWandSparkles, LucideUndo2, LucideRedo2, LucideChevronUp, LucideChevronDown, LucideGripVertical, LucideChartColumn, LucideMerge } from '@lucide/angular';

import { routes } from './app.routes';
import { AUTH_INTERCEPTOR_PROVIDER, ERROR_INTERCEPTOR_PROVIDER } from './core';
import { AuthService } from './core/services/auth.service';
import { anchorOffsetInitializer } from './core/config/anchor-offset';
import { ConsentIntegrations } from './core/consent/consent-integrations';

export function initAuthFactory(): () => Promise<unknown> {
  const auth = inject(AuthService);
  return () => firstValueFrom(auth.initialize().pipe(catchError(() => of(null))));
}

/** Reads the saved consent and starts what it allows, before the app renders and before sign-in restores (#321). */
export function startConsentFactory(): () => void {
  const integrations = inject(ConsentIntegrations);
  return () => integrations.start();
}

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideZoneChangeDetection({ eventCoalescing: true }),
    provideRouter(
      routes,
      withInMemoryScrolling({ anchorScrolling: 'enabled', scrollPositionRestoration: 'enabled' }),
    ),
    // One address per page: every router URL ends in a slash, as Amplify serves it (#361).
    { provide: UrlSerializer, useClass: TrailingSlashUrlSerializer },
    provideHttpClient(withInterceptorsFromDi(), withFetch()),
    provideAnimationsAsync(),
    AUTH_INTERCEPTOR_PROVIDER,
    ERROR_INTERCEPTOR_PROVIDER,
    { provide: APP_INITIALIZER, multi: true, useFactory: startConsentFactory },
    { provide: APP_INITIALIZER, multi: true, useFactory: initAuthFactory },
    { provide: APP_INITIALIZER, multi: true, useFactory: anchorOffsetInitializer },
    provideLucideIcons(LucideVolume2, LucideSquare, LucideVideo, LucideWand2, LucidePalette, LucideMic, LucideZap, LucideUpload, LucidePlay, LucideDownload, LucideSettings, LucideUser, LucideHelpCircle, LucideLogOut, LucidePlus, LucideArrowRight, LucideCheck, LucideX, LucideLoader2, LucideMoon, LucideSun, LucideHome, LucideFolderOpen, LucideChevronRight, LucideCircleCheck, LucideCircleX, LucideClock, LucideFileText, LucideScissors, LucideStamp, LucideCaptions, LucideFocus, LucideFrame, LucideGauge, LucideShare2, LucideCopy, LucideClapperboard, LucideSparkles, LucidePencil, LucideImage, LucideStar, LucideSquarePlay, LucideCamera, LucideMusic2, LucideTrash2, LucideMoreVertical, LucideCreditCard, LucideArrowUpRight, LucideRefreshCw, LucideFilm, LucideSearch, LucideTriangleAlert, LucideLayoutDashboard, LucidePause, LucideLock, LucideShieldCheck, LucideUserX, LucideRotateCcw, LucideWandSparkles, LucideUndo2, LucideRedo2, LucideChevronUp, LucideChevronDown, LucideGripVertical, LucideChartColumn, LucideMerge),
  ],
};
