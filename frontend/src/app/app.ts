import { Component, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NavigationStart, Router, RouterOutlet } from '@angular/router';
import { filter } from 'rxjs';
import { ThemeService } from './core/services/theme.service';
import { clearJsonLd, robotsForUrl, setRobots } from './shared/seo';
import { CookieBannerComponent } from './shared/components/cookie-consent/cookie-banner.component';
import { CookiePreferencesComponent } from './shared/components/cookie-consent/cookie-preferences.component';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, CookieBannerComponent, CookiePreferencesComponent],
  template: '<router-outlet /><app-cookie-banner /><app-cookie-preferences />',
})
export class App {
  constructor() {
    // Created at startup so a theme saved before the banner is used again as soon as Functional is allowed, on any page.
    inject(ThemeService);
    const doc = inject(DOCUMENT);
    // The head is shared for the whole session. Reset what the previous page
    // set before the next page runs, so a noindex or a JSON-LD block never
    // carries over to a page that should be indexed.
    inject(Router)
      .events.pipe(
        filter((e): e is NavigationStart => e instanceof NavigationStart),
        takeUntilDestroyed(),
      )
      .subscribe((e) => {
        setRobots(doc, robotsForUrl(e.url));
        clearJsonLd(doc);
      });
  }
}
