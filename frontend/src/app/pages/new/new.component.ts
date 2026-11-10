import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { FORMAT_GUIDE_PATH, SAMPLE_FILE_PATH } from '../progress/failure-message';

/** Where people land after signing in: pick one of the four tools. */
@Component({
  selector: 'app-new',
  standalone: true,
  imports: [RouterLink, LucideDynamicIcon, NavbarComponent],
  template: `
    <div class="ui-app">
      <app-navbar />
      <main id="main" class="ui-app-main hm">
        <div>
          <h1 class="ui-h1">What do you want to make?</h1>
          <p class="ui-lead">Pick a tool, or <a routerLink="/dashboard" class="ui-link">go to your dashboard</a>.</p>
        </div>

        <div class="tools">
          <section class="ui-card tool" aria-labelledby="t-shorts" data-testid="tool-shorts">
            <img src="/screenshots/shorts-hero-still.png" alt="" width="640" height="400" loading="eager" />
            <div class="body">
              <p class="kind"><svg lucideIcon="scissors" [size]="16" aria-hidden="true"></svg>Vertical clips</p>
              <h2 id="t-shorts" class="ui-card-title">AI Shorts</h2>
              <p class="ui-card-sub">Get ranked 9:16 clips from a long video.</p>
              <a routerLink="/shorts" class="ui-btn ui-btn--primary cta">Make shorts</a>
            </div>
          </section>

          <section class="ui-card tool" aria-labelledby="t-quiz" data-testid="tool-quiz">
            <img src="/screenshots/quiz-create.png" alt="" width="640" height="400" loading="eager" />
            <div class="body">
              <p class="kind"><svg lucideIcon="file-text" [size]="16" aria-hidden="true"></svg>Quiz videos</p>
              <h2 id="t-quiz" class="ui-card-title">TXT to quiz video</h2>
              <p class="ui-card-sub">A narrated video from your questions.</p>
              <div class="need" data-testid="quiz-needs">
                <p><strong>You'll need a TXT file of questions.</strong></p>
                <p class="links">
                  <a [href]="sampleFilePath" download="quiz-sample.txt" class="ui-link">Download a sample file</a>
                  <a [routerLink]="formatGuidePath" class="ui-link">Read the format guide</a>
                </p>
              </div>
              <a routerLink="/projects/new" class="ui-btn ui-btn--primary cta">Make a quiz video</a>
            </div>
          </section>

          <section class="ui-card tool" aria-labelledby="t-story" data-testid="tool-story">
            <div class="art" aria-hidden="true"><svg lucideIcon="clapperboard" [size]="44"></svg></div>
            <div class="body">
              <p class="kind"><svg lucideIcon="film" [size]="16" aria-hidden="true"></svg>Story videos</p>
              <h2 id="t-story" class="ui-card-title">Story to video</h2>
              <p class="ui-card-sub">Have a story, a script or notes instead of quiz questions? Turn it into a video.</p>
              <a routerLink="/story" class="ui-btn ui-btn--primary cta">Use Story video</a>
            </div>
          </section>

          <section class="ui-card tool" aria-labelledby="t-wm" data-testid="tool-watermark">
            <img src="/screenshots/watermark-tool.png" alt="" width="640" height="400" loading="lazy" />
            <div class="body">
              <p class="kind"><svg lucideIcon="stamp" [size]="16" aria-hidden="true"></svg>Free tool</p>
              <h2 id="t-wm" class="ui-card-title">Watermark tool</h2>
              <p class="ui-card-sub">Mark an image or video in your browser.</p>
              <a routerLink="/tools/add-watermark" class="ui-btn ui-btn--secondary cta">Open the watermark tool</a>
            </div>
          </section>
        </div>
      </main>
    </div>
  `,
  styles: [`
    :host { display: block; }
    .tools { display: grid; gap: var(--space-md); grid-template-columns: minmax(0, 1fr); }
    @media (min-width: 44rem) { .tools { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
    .tool { padding: 0; overflow: hidden; display: flex; flex-direction: column; }
    .tool img, .art { display: block; width: 100%; height: 10rem; object-fit: cover; object-position: top; border-bottom: 1px solid var(--color-rule); background: var(--color-paper-3); }
    .art { display: flex; align-items: center; justify-content: center; color: var(--color-accent-text); }
    .body { flex: 1; display: flex; flex-direction: column; gap: var(--space-2xs); padding: var(--space-lg); }
    @media (max-width: 39.99rem) { .body { padding: var(--space-md); } .tool img, .art { height: 8rem; } }
    .kind { margin: 0; display: inline-flex; align-items: center; gap: var(--space-2xs); font-size: var(--type-xs); font-weight: 600; color: var(--color-accent-text); }
    .need { margin-top: var(--space-xs); padding: var(--space-xs) var(--space-sm); border-left: 3px solid var(--color-accent); background: var(--app-page); border-radius: 0 var(--radius-control) var(--radius-control) 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .need p { margin: 0; }
    .need strong { color: var(--color-ink); }
    .links { display: flex; flex-wrap: wrap; gap: 0 var(--space-md); }
    .links a { display: inline-flex; align-items: center; min-height: 44px; font-weight: 600; }
    .cta { margin-top: auto; align-self: flex-start; }
    .body > .cta { margin-top: var(--space-sm); }
    .body > .ui-card-sub { flex: 1; }
    @media (max-width: 39.99rem) { .cta { align-self: stretch; } }
  `],
})
export class NewComponent {
  readonly sampleFilePath = SAMPLE_FILE_PATH;
  readonly formatGuidePath = FORMAT_GUIDE_PATH;
}
