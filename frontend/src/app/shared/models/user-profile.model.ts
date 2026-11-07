export interface UserProfile {
  email: string;
  name: string;
  avatarUrl: string;
  createdAt: string;
  /** False for a password account whose address has not been confirmed with the emailed link. */
  emailVerified?: boolean;
  /** Confirmation is switched on, so the link can be sent again. */
  canConfirmEmail?: boolean;
  /** Creating projects and Shorts waits for the link (new password sign-ups). Older accounts can confirm but are not held back. */
  emailConfirmationRequired?: boolean;
}
