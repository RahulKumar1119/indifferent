import { Component } from '@angular/core';
import { MatCardModule } from '@angular/material/card';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatIconModule } from '@angular/material/icon';
import { MatDividerModule } from '@angular/material/divider';

@Component({
  selector: 'app-help',
  standalone: true,
  imports: [MatCardModule, MatExpansionModule, MatIconModule, MatDividerModule],
  template: `
    <div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-6 text-2xl font-bold">Help</h1>

      <!-- FAQ Section -->
      <section class="mb-8">
        <h2 class="mb-4 text-lg font-semibold">Frequently Asked Questions</h2>
        <mat-accordion>
          <mat-expansion-panel>
            <mat-expansion-panel-header>
              <mat-panel-title>What file formats are supported?</mat-panel-title>
            </mat-expansion-panel-header>
            <p>
              We support TXT files with questions formatted using numbered lists, bulleted lists, or
              tab-indented structures. Each question should include the question text and answer
              options.
            </p>
          </mat-expansion-panel>

          <mat-expansion-panel>
            <mat-expansion-panel-header>
              <mat-panel-title>How long does processing take?</mat-panel-title>
            </mat-expansion-panel-header>
            <p>
              Processing typically takes 2-5 minutes depending on the number of questions in your
              file. Larger files with more questions will take longer to generate slides, narration,
              and final video output.
            </p>
          </mat-expansion-panel>

          <mat-expansion-panel>
            <mat-expansion-panel-header>
              <mat-panel-title>What templates are available?</mat-panel-title>
            </mat-expansion-panel-header>
            <p>
              Currently we offer the Classic template, which provides a clean quiz-show style layout.
              More templates are coming soon.
            </p>
          </mat-expansion-panel>

          <mat-expansion-panel>
            <mat-expansion-panel-header>
              <mat-panel-title>What voices can I use?</mat-panel-title>
            </mat-expansion-panel-header>
            <p>
              We offer 5 Amazon Polly neural voices to narrate your quiz videos. You can select your
              preferred voice during project creation.
            </p>
          </mat-expansion-panel>
        </mat-accordion>
      </section>

      <mat-divider class="my-6" />

      <!-- Usage Instructions -->
      <section class="mb-8">
        <h2 class="mb-4 text-lg font-semibold">How to Use</h2>
        <mat-card>
          <mat-card-content class="p-6">
            <ol class="list-decimal space-y-3 pl-5">
              <li>Create a new project from the dashboard</li>
              <li>Upload a TXT file containing your quiz questions</li>
              <li>Select a template and voice for your video</li>
              <li>Click "Generate" and wait for processing to complete</li>
              <li>Download your finished quiz video</li>
            </ol>
          </mat-card-content>
        </mat-card>
      </section>

      <mat-divider class="my-6" />

      <!-- Contact Information -->
      <section>
        <h2 class="mb-4 text-lg font-semibold">Contact Us</h2>
        <mat-card>
          <mat-card-content class="p-6">
            <div class="flex items-center gap-3">
              <mat-icon>email</mat-icon>
              <a href="mailto:support@indifferent.app" class="text-blue-600 hover:underline dark:text-blue-400">
                support&#64;indifferent.app
              </a>
            </div>
          </mat-card-content>
        </mat-card>
      </section>
    </div>
  `,
})
export class HelpComponent {}
