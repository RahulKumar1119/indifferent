import { Injectable, inject } from '@angular/core';
import { HttpContext } from '@angular/common/http';
import { Observable } from 'rxjs';
import { ApiService } from '../../core/services/api.service';
import { SUPPRESS_GLOBAL_ERROR } from '../../core/interceptors/error.interceptor';

/** GET/PUT /settings/notifications. */
export interface NotificationSettings {
  /** Address the emails go to ('' when the account has none). */
  email: string;
  /**
   * False when we cannot confirm the address belongs to the account holder.
   * Nothing is sent to such an address, whatever the switches say.
   */
  emailVerified: boolean;
  notifyCompleted: boolean;
  notifyFailed: boolean;
}

export type NotificationPreference = 'notifyCompleted' | 'notifyFailed';

// The Settings page reports failures inline, so skip the global error toast.
const LOCAL_ERRORS = () => new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true);

@Injectable({ providedIn: 'root' })
export class NotificationSettingsService {
  private readonly api = inject(ApiService);

  load(): Observable<NotificationSettings> {
    return this.api.get<NotificationSettings>('/settings/notifications', undefined, true, LOCAL_ERRORS());
  }

  /** Saves one preference and returns the stored settings. */
  save(preference: NotificationPreference, value: boolean): Observable<NotificationSettings> {
    return this.api.put<NotificationSettings>(
      '/settings/notifications',
      { [preference]: value },
      true,
      LOCAL_ERRORS(),
    );
  }
}
