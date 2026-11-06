/**
 * Protects "Continue with Google" against login CSRF (SEC-07).
 *
 * Without this, the callback page accepted any `?code=` it was given. An attacker
 * could start a sign-in with their own Google account, stop before the callback,
 * and send a victim a link to /auth/callback with the attacker's code. The
 * victim's browser would redeem it and be signed in to the attacker's account,
 * and whatever the victim then uploaded would land there.
 *
 * Two values close that hole. Both are made in the browser before it leaves for
 * Google and are checked when it comes back:
 *
 * - `state`: a random value Google returns untouched. The callback refuses to
 *   continue unless it equals the value this browser stored, so a link built by
 *   someone else carries no matching state.
 * - PKCE (RFC 7636): the browser keeps a secret `verifier` and sends Google only
 *   its SHA-256 `challenge`. Google will give the code only to whoever can
 *   present the verifier, so a code made in another browser cannot be redeemed
 *   here, even if the state check were bypassed.
 */

/** sessionStorage key for the flow in progress. Per tab, gone when the tab closes. */
const FLOW_KEY = 'indifferent.oauthFlow';

/** A sign-in that takes longer than this between the click and the callback is refused. */
export const OAUTH_FLOW_MAX_AGE_MS = 10 * 60 * 1000;

export interface OAuthFlow {
  /** Sent to Google as `state`. */
  state: string;
  /** Kept in the browser; sent to the API with the code. Empty when PKCE is unavailable. */
  verifier: string;
  /** Sent to Google as `code_challenge` (S256). Empty when PKCE is unavailable. */
  challenge: string;
}

function base64Url(bytes: Uint8Array): string {
  let binary = '';
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function randomUrlSafe(byteLength: number): string {
  const bytes = new Uint8Array(byteLength);
  crypto.getRandomValues(bytes);
  return base64Url(bytes);
}

/** The S256 code challenge for a verifier: base64url(SHA-256(verifier)). */
export async function pkceChallenge(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier));
  return base64Url(new Uint8Array(digest));
}

/**
 * Makes the values for one sign-in. It never throws: a browser without
 * `crypto.subtle` (an insecure page) still gets a random `state`, which is the
 * check that matters most, and simply goes without PKCE.
 */
export async function createOAuthFlow(): Promise<OAuthFlow> {
  const state = randomUrlSafe(32);
  try {
    // 48 random bytes make a 64 character verifier (the allowed range is 43 to 128).
    const verifier = randomUrlSafe(48);
    return { state, verifier, challenge: await pkceChallenge(verifier) };
  } catch {
    return { state, verifier: '', challenge: '' };
  }
}

/** Remembers the flow across the round trip to Google. */
export function rememberOAuthFlow(flow: OAuthFlow, now: number = Date.now()): void {
  sessionStorage.setItem(FLOW_KEY, JSON.stringify({ state: flow.state, verifier: flow.verifier, createdAt: now }));
}

/**
 * Reads the remembered flow and forgets it, so one sign-in can only be finished
 * once. Returns null when there is none, when it is unreadable, or when it is
 * older than OAUTH_FLOW_MAX_AGE_MS.
 */
export function takeOAuthFlow(now: number = Date.now()): { state: string; verifier: string } | null {
  let raw: string | null = null;
  try {
    raw = sessionStorage.getItem(FLOW_KEY);
    sessionStorage.removeItem(FLOW_KEY);
  } catch {
    return null;
  }
  if (!raw) return null;
  try {
    const f = JSON.parse(raw) as Partial<{ state: unknown; verifier: unknown; createdAt: unknown }>;
    if (typeof f.state !== 'string' || f.state.length < 16) return null;
    if (typeof f.verifier !== 'string') return null;
    if (typeof f.createdAt !== 'number' || now - f.createdAt > OAUTH_FLOW_MAX_AGE_MS || f.createdAt > now + 60_000) return null;
    return { state: f.state, verifier: f.verifier };
  } catch {
    return null;
  }
}
