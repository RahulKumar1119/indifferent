import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ProjectService } from '../../core/services/project.service';
import { Project } from '../../shared/models/project.model';
import { projectStatusLabel, projectStatusTone } from './project-status';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';

@Component({
  selector: 'app-projects',
  standalone: true,
  imports: [CommonModule, RouterLink, LucideDynamicIcon, NavbarComponent],
  styles: [`
    :host { display: block; }
    .list { display: flex; flex-direction: column; gap: var(--space-xs); }
    .rows { margin: 0; padding: var(--space-2xs) var(--space-md); list-style: none; }
    .rows li + li { border-top: 1px solid var(--color-rule); }
    .open { display: flex; align-items: center; gap: var(--space-sm); min-height: 44px; padding: var(--space-sm) var(--space-xs); margin-inline: calc(-1 * var(--space-xs)); border-radius: var(--radius-control); color: var(--color-ink); text-decoration: none; }
    .open:hover { background: var(--color-paper-3); }
    .main { flex: 1; min-width: 0; }
    .title { display: block; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .meta { display: flex; flex-wrap: wrap; gap: 0 var(--space-sm); font-size: var(--type-xs); color: var(--color-muted); }
    .arrow { flex: none; color: var(--color-muted); }
    .t-ok { color: var(--color-positive); } .t-bad { color: var(--color-negative); } .t-wait { color: var(--color-caution); } .t-new { color: var(--color-ink-2); }
    .ui-empty .ui-btn, .ui-error-card .ui-btn { margin-top: var(--space-xs); }
  `],
  template: `
    <div class="ui-app">
      <app-navbar />
      <main id="main" class="ui-app-main hm">
        <div class="ui-page-head">
          <div>
            <h1 class="ui-h1">Quiz videos</h1>
            <p class="ui-lead">
              @if (!isLoading && !loadError && projects.length > 0) {
                {{ projects.length }} {{ projects.length === 1 ? 'project' : 'projects' }}, newest first.
              } @else {
                Every quiz project you make, newest first.
              }
            </p>
          </div>
          <a routerLink="/projects/new" class="ui-btn ui-btn--primary"><svg lucideIcon="plus" [size]="18" aria-hidden="true"></svg>New quiz video</a>
        </div>

        @if (isLoading && !loadError) {
          <div class="list" role="status" aria-label="Loading projects" data-testid="loading">
            @for (i of [1, 2, 3]; track i) {
              <div class="ui-skeleton" style="height: 4.25rem"></div>
            }
          </div>
        } @else if (loadError) {
          <div class="ui-card ui-error-card" role="alert" data-testid="load-error">
            <p><svg lucideIcon="triangle-alert" [size]="18" aria-hidden="true"></svg>Couldn't load your projects.</p>
            <span class="ui-card-sub">Check your connection and try again.</span>
            <button type="button" class="ui-btn ui-btn--secondary" (click)="reload()">Try again</button>
          </div>
        } @else if (projects.length === 0) {
          <div class="ui-card ui-empty" data-testid="empty">
            <span class="ui-chip" aria-hidden="true"><svg lucideIcon="file-text" [size]="18"></svg></span>
            <p><strong>No quiz videos yet.</strong></p>
            <p>Write your questions in a TXT file and we turn them into a narrated video.</p>
            <a routerLink="/projects/new" class="ui-btn ui-btn--primary">Make your first quiz video</a>
          </div>
        } @else {
          <ul class="ui-card rows" aria-label="Your quiz videos">
            @for (project of projects; track project.id) {
              <li [attr.data-testid]="'project-' + project.id">
                <a [routerLink]="['/projects', project.id]" class="open">
                  <span class="main">
                    <span class="title">{{ project.name }}</span>
                    <span class="meta">
                      <span>{{ project.template | titlecase }}</span>
                      <span>{{ formatDate(project.createdAt) }}</span>
                    </span>
                  </span>
                  <span [class]="'ui-pill ' + tone(project.status)">{{ getStatusLabel(project.status) }}</span>
                  <svg lucideIcon="chevron-right" [size]="16" class="arrow" aria-hidden="true"></svg>
                </a>
              </li>
            }
          </ul>
        }
      </main>
    </div>
  `,
})
export class ProjectsComponent implements OnInit {
  private readonly projectService = inject(ProjectService);

  projects: Project[] = [];
  loadError = false;
  isLoading = true;

  ngOnInit(): void {
    this.reload();
  }

  reload(): void {
    this.loadError = false;
    this.isLoading = true;
    this.projectService.getProjects().subscribe({
      next: (projects) => {
        this.projects = [...projects].sort(
          (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
        );
        this.isLoading = false;
      },
      error: () => {
        this.loadError = true;
        this.isLoading = false;
      },
    });
  }

  readonly getStatusLabel = projectStatusLabel;
  readonly tone = projectStatusTone;

  formatDate(dateStr: string): string {
    const date = new Date(dateStr);
    return isNaN(date.getTime())
      ? dateStr
      : date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
  }
}
