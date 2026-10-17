import { Routes } from '@angular/router';
import { authGuard } from '../../auth';
import { ProfileComponent } from './profile.component';
import { SettingsComponent } from '../settings/settings.component';

export const profileRoutes: Routes = [
  { path: '', component: ProfileComponent, canActivate: [authGuard] },
  { path: 'settings', component: SettingsComponent, canActivate: [authGuard] },
];

export { ProfileComponent } from './profile.component';
export { ProfileService, SOCIAL_PLATFORMS } from './services/profile.service';
export type { SocialConnection, SocialPlatform } from './services/profile.service';
