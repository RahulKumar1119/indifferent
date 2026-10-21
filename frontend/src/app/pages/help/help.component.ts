import { Component } from '@angular/core';
import { MatExpansionModule } from '@angular/material/expansion';
import { LucideDynamicIcon } from '@lucide/angular';

@Component({
  selector: 'app-help',
  standalone: true,
  imports: [MatExpansionModule, LucideDynamicIcon],
  template: `
    <div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-6 text-2xl font-bold">Help</h1>

      <!-- FAQ Section -->
      <section class="mb-8">
        <h2 class="mb-4 text-lg font-semibold flex items-center gap-2">
          <svg lucideIcon="help-circle" [size]="20" class="text-[hsl(var(--primary))]"></svg>
          Frequently Asked Questions
        </h2>
        <mat-accordion>
          <mat-expansion-panel>
            <mat-expansion-panel-header>
              <mat-panel-title>What file formats are supported?</mat-panel-title>
            </mat-expansion-panel-header>
            <p class="text-[hsl(var(--muted-foreground))]">
              We support TXT files with questions formatted using numbered lists, bulleted lists, or
              tab-indented structures. Each question should include the question text and answer
              options.
            </p>
          </mat-expansion-panel>

          <mat-expansion-panel>
            <mat-expansion-panel-header>
              <mat-panel-title>How long does processing take?</mat-panel-title>
            </mat-expansion-panel-header>
            <p class="text-[hsl(var(--muted-foreground))]">
              Processing typically takes 2-5 minutes depending on the number of questions in your
              file. Larger files with more questions will take longer to generate slides, narration,
              and final video output.
            </p>
          </mat-expansion-panel>

          <mat-expansion-panel>
            <mat-expansion-panel-header>
              <mat-panel-title>What designs are available?</mat-panel-title>
            </mat-expansion-panel-header>
            <p class="text-[hsl(var(--muted-foreground))]">
              Six: Classic, Modern, Education, Dark, Minimal and Neon. A design sets the colours, type scale and
              layout of your slides, including the background, answer cards and the highlight on the correct answer.
              The project form shows a real preview of each one.
            </p>
          </mat-expansion-panel>

          <mat-expansion-panel>
            <mat-expansion-panel-header>
              <mat-panel-title>What voices can I use?</mat-panel-title>
            </mat-expansion-panel-header>
            <p class="text-[hsl(var(--muted-foreground))]">
              We offer 5 Amazon Polly neural voices to narrate your quiz videos. You can select your
              preferred voice during project creation.
            </p>
          </mat-expansion-panel>
        </mat-accordion>
      </section>

      <!-- Usage Instructions -->
      <section class="mb-8">
        <h2 class="mb-4 text-lg font-semibold flex items-center gap-2">
          <svg lucideIcon="play" [size]="20" class="text-[hsl(var(--primary))]"></svg>
          How to Use
        </h2>
        <div class="glass-card p-6">
          <ol class="list-decimal space-y-3 pl-5 text-[hsl(var(--muted-foreground))]">
            <li>Create a new project from the dashboard</li>
            <li>Upload a TXT file containing your quiz questions</li>
            <li>Select a design and voice for your video</li>
            <li>Click "Generate" and wait for processing to complete</li>
            <li>Download your finished quiz video</li>
          </ol>
        </div>
      </section>

      <!-- Contact Information -->
      <section>
        <h2 class="mb-4 text-lg font-semibold flex items-center gap-2">
          <svg lucideIcon="settings" [size]="20" class="text-[hsl(var(--primary))]"></svg>
          Contact Us
        </h2>
        <div class="glass-card p-6">
          <div class="flex items-center gap-3">
            <svg lucideIcon="settings" [size]="20" class="text-[hsl(var(--muted-foreground))]"></svg>
            <a href="mailto:support@indifferent.fun" class="text-[hsl(var(--primary))] hover:underline">
              support&#64;indifferent.fun
            </a>
          </div>
        </div>
      </section>
    </div>
  `,
})
export class HelpComponent {}
