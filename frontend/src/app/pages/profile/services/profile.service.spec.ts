import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { ProfileService } from './profile.service';
import { UserProfile } from '../../../shared/models/user-profile.model';
import { emptyCreatorProfile } from '../models/creator-profile.model';

const KEYS = [
  'indifferent.creatorProfile.v1',
  'indifferent.creatorDna.v1',
  'indifferent.creatorSocial.v1',
  'indifferent.creatorFavorites.v1',
  'indifferent.creatorPublished.v1',
  'indifferent.creatorActivity.v1',
  'indifferent.creatorAvatarSynced.v1',
];

const remote = (over: Partial<UserProfile> = {}): UserProfile => ({
  email: 'new.person@example.com',
  name: 'New Person',
  avatarUrl: '',
  createdAt: '2026-11-07T10:00:00Z',
  ...over,
});

describe('ProfileService', () => {
  let service: ProfileService;

  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    service = TestBed.inject(ProfileService);
  });

  afterEach(() => localStorage.clear());

  describe('username', () => {
    it('is not made up from the name or the email', () => {
      const merged = service.mergeBackendUser(emptyCreatorProfile(), remote({ name: 'Rahul Prem', email: 'rahulprem512@gmail.com' }));
      expect(merged.username).toBe('');
      expect(merged.name).toBe('Rahul Prem');
    });

    it('stays what the person saved, whatever their name is', () => {
      const saved = service.saveProfile({ name: 'New Person', email: 'new.person@example.com', username: 'my_handle' });
      const merged = service.mergeBackendUser(saved, remote());
      expect(merged.username).toBe('my_handle');
    });
  });

  describe('signing in as someone else on the same browser', () => {
    function previousPersonLeavesData(): void {
      service.saveProfile({
        name: 'Rahul Prem',
        email: 'rahulprem512@gmail.com',
        username: 'rahulprem',
        bio: 'My own bio',
        avatarUrl: 'data:image/png;base64,AAAA',
      });
      for (const key of KEYS.slice(1)) localStorage.setItem(key, JSON.stringify(['x']));
    }

    it('does not hand them the last person\'s handle, bio or avatar', () => {
      previousPersonLeavesData();
      const merged = service.mergeBackendUser(service.loadProfile(), remote());
      expect(merged.username).toBe('');
      expect(merged.bio).not.toBe('My own bio');
      expect(merged.avatarUrl).toBe('');
      expect(merged.name).toBe('New Person');
      expect(merged.email).toBe('new.person@example.com');
    });

    it('forgets everything else kept in the browser too', () => {
      previousPersonLeavesData();
      service.mergeBackendUser(service.loadProfile(), remote());
      for (const key of KEYS) expect(localStorage.getItem(key)).withContext(key).toBeNull();
    });

    it('treats the same address in another case or with spaces as the same person', () => {
      service.saveProfile({ email: 'Rahul.Prem@Gmail.com', username: 'rahulprem', bio: 'Mine' });
      const merged = service.mergeBackendUser(service.loadProfile(), remote({ email: '  rahul.prem@gmail.com ', name: 'Rahul' }));
      expect(merged.username).toBe('rahulprem');
      expect(merged.bio).toBe('Mine');
      expect(localStorage.getItem('indifferent.creatorProfile.v1')).not.toBeNull();
    });

    it('keeps data saved before an address was recorded, for the account that signs in', () => {
      service.saveProfile({ username: 'early_user', bio: 'Early bio' });
      const merged = service.mergeBackendUser(service.loadProfile(), remote());
      expect(merged.username).toBe('early_user');
      expect(merged.bio).toBe('Early bio');
    });

    it('does nothing when the account could not be read', () => {
      previousPersonLeavesData();
      const local = service.loadProfile();
      expect(service.mergeBackendUser(local, null)).toBe(local);
      for (const key of KEYS) expect(localStorage.getItem(key)).withContext(key).not.toBeNull();
    });
  });

  it('clearLocalData removes only this service\'s keys', () => {
    for (const key of KEYS) localStorage.setItem(key, '1');
    localStorage.setItem('app-theme', 'dark');
    service.clearLocalData();
    for (const key of KEYS) expect(localStorage.getItem(key)).withContext(key).toBeNull();
    expect(localStorage.getItem('app-theme')).toBe('dark');
  });
});
