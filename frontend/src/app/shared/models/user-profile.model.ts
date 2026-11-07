export interface UserProfile {
  email: string;
  name: string;
  avatarUrl: string;
  createdAt: string;
  /** False for a password account whose address has not been confirmed with the emailed link. */
  emailVerified?: boolean;
}
