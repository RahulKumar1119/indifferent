import { Injectable } from '@angular/core';
import { Observable, timer, switchMap, takeWhile, EMPTY, of, retry, tap } from 'rxjs';
import { ApiService } from './api.service';
import {
  Project,
  PipelineProgress,
  CreateProjectRequest,
} from '../../shared/models/project.model';

@Injectable({ providedIn: 'root' })
export class ProjectService {
  constructor(private readonly api: ApiService) {}

  getProjects(): Observable<Project[]> {
    return this.api.get<Project[]>('/projects');
  }

  getProject(id: string): Observable<Project> {
    return this.api.get<Project>(`/projects/${id}`);
  }

  getProjectStatus(id: string): Observable<PipelineProgress> {
    return this.api.get<PipelineProgress>(`/projects/${id}/status`);
  }

  createProject(req: CreateProjectRequest): Observable<Project> {
    return this.api.post<Project>('/projects', req);
  }

  deleteProject(id: string): Observable<void> {
    return this.api.delete<void>(`/projects/${id}`);
  }

  uploadFile(projectId: string, file: File): Observable<void> {
    return this.api.post<{ uploadUrl: string }>(`/projects/${projectId}/upload`, {
      fileName: file.name,
      contentType: file.type,
    }).pipe(
      switchMap((res) => this.api.put<void>(res.uploadUrl, file))
    );
  }

  /**
   * Polls project status with adaptive intervals:
   * - 3s when page is visible (active)
   * - 30s when page is hidden (idle)
   * - Exponential backoff on failure (up to 60s)
   *
   * Stops when status reaches 'completed' or 'failed'.
   */
  pollProjectStatus(projectId: string): Observable<PipelineProgress> {
    let failureCount = 0;

    return new Observable<PipelineProgress>((subscriber) => {
      let timeoutId: ReturnType<typeof setTimeout> | null = null;
      let destroyed = false;

      const getInterval = (): number => {
        if (failureCount > 0) {
          return Math.min(3000 * Math.pow(2, failureCount), 60000);
        }
        return document.hidden ? 30000 : 3000;
      };

      const poll = () => {
        if (destroyed) return;

        this.getProjectStatus(projectId).subscribe({
          next: (progress) => {
            failureCount = 0;
            subscriber.next(progress);

            if (progress.stage === 'completed' || progress.stage === 'failed') {
              subscriber.complete();
              return;
            }

            timeoutId = setTimeout(poll, getInterval());
          },
          error: (err) => {
            failureCount++;
            if (failureCount > 10) {
              subscriber.error(err);
              return;
            }
            timeoutId = setTimeout(poll, getInterval());
          },
        });
      };

      const onVisibilityChange = () => {
        if (!document.hidden && timeoutId !== null) {
          clearTimeout(timeoutId);
          timeoutId = setTimeout(poll, 500);
        }
      };

      document.addEventListener('visibilitychange', onVisibilityChange);
      poll();

      return () => {
        destroyed = true;
        if (timeoutId !== null) {
          clearTimeout(timeoutId);
        }
        document.removeEventListener('visibilitychange', onVisibilityChange);
      };
    });
  }
}
