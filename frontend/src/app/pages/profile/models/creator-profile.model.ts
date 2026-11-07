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
