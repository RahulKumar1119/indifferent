import { Injectable } from '@angular/core';
import { BehaviorSubject } from 'rxjs';

export interface Toast {
  id: number;
  message: string;
  kind: 'success' | 'error' | 'info';
}

/** Minimal top-right toast bus: slide-in 300ms, auto-dismiss 4s (in component). */
@Injectable({ providedIn: 'root' })
export class ToastService {
  private readonly toasts$ = new BehaviorSubject<Toast[]>([]);
  private nextId = 1;

  readonly toasts = this.toasts$.asObservable();

  show(message: string, kind: Toast['kind'] = 'info'): void {
    const id = this.nextId++;
    this.toasts$.next([...this.toasts$.getValue(), { id, message, kind }]);
    setTimeout(() => this.dismiss(id), 4000);
  }

  dismiss(id: number): void {
    this.toasts$.next(this.toasts$.getValue().filter((t) => t.id !== id));
  }
}
