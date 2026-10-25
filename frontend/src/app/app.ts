import { Component, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NavigationStart, Router, RouterOutlet } from '@angular/router';
import { filter } from 'rxjs';
import { clearJsonLd, robotsForUrl, setRobots } from './shared/seo';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet],
  templateUrl: './app.html',
  styleUrl: './app.css',
})
export class App {
  constructor() {
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
