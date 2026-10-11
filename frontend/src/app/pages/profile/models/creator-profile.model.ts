/** Creator identity shown in the profile header. */
export interface CreatorProfile {
  name: string;
  username: string;
  email: string;
  bio: string;
  avatarUrl: string;
  badge: string;
  createdAt: string;
}

/** Derive a URL-safe username from a display name or email. Underscores are
 * kept to match the Edit Profile validator ([a-zA-Z0-9_], 2–24 chars). */
export function toUsername(name: string, fallbackEmail = ''): string {
  const base = (name || fallbackEmail.split('@')[0] || 'creator')
    .toLowerCase()
    .replace(/[^a-z0-9_]+/g, '');
  return base.slice(0, 24) || 'creator';
}

/** Blank profile used before real data loads. */
export function emptyCreatorProfile(): CreatorProfile {
  return {
    name: '',
    username: '',
    email: '',
    bio: '',
    avatarUrl: '',
    badge: 'AI Creator',
    createdAt: new Date().toISOString(),
  };
}
