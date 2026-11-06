import {
  OAUTH_FLOW_MAX_AGE_MS,
  createOAuthFlow,
  pkceChallenge,
  rememberOAuthFlow,
  takeOAuthFlow,
} from './oauth-flow';

describe('oauth-flow', () => {
  beforeEach(() => sessionStorage.clear());
  afterEach(() => sessionStorage.clear());

  describe('pkceChallenge', () => {
    it('matches the example in RFC 7636 appendix B', async () => {
      expect(await pkceChallenge('dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk')).toBe(
        'E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM',
      );
    });
  });

  describe('createOAuthFlow', () => {
    it('makes an unguessable state and a verifier Google and our API accept', async () => {
      const flow = await createOAuthFlow();
      expect(flow.state).toMatch(/^[A-Za-z0-9_-]{43}$/); // 32 random bytes
      expect(flow.verifier).toMatch(/^[A-Za-z0-9._~-]{43,128}$/); // RFC 7636, and the API's own check
      expect(flow.verifier.length).toBe(64);
    });

    it('derives the challenge from the verifier it made', async () => {
      const flow = await createOAuthFlow();
      expect(flow.challenge).toBe(await pkceChallenge(flow.verifier));
      expect(flow.challenge).not.toBe(flow.verifier);
    });

    it('makes new values every time', async () => {
      const a = await createOAuthFlow();
      const b = await createOAuthFlow();
      expect(a.state).not.toBe(b.state);
      expect(a.verifier).not.toBe(b.verifier);
    });

    it('still gives a state, and goes without PKCE, when the browser cannot hash', async () => {
      spyOn(crypto.subtle, 'digest').and.rejectWith(new Error('no subtle crypto'));
      const flow = await createOAuthFlow();
      expect(flow.state.length).toBe(43);
      expect(flow.verifier).toBe('');
      expect(flow.challenge).toBe('');
    });
  });

  describe('rememberOAuthFlow / takeOAuthFlow', () => {
    const flow = { state: 's'.repeat(43), verifier: 'v'.repeat(64), challenge: 'c' };

    it('hands back what was remembered, once', () => {
      rememberOAuthFlow(flow);
      expect(takeOAuthFlow()).toEqual({ state: flow.state, verifier: flow.verifier });
      expect(takeOAuthFlow()).toBeNull(); // one sign-in can only be finished once
    });

    it('never stores the challenge, only what the callback needs', () => {
      rememberOAuthFlow(flow);
      expect(sessionStorage.getItem('indifferent.oauthFlow')).not.toContain('"challenge"');
    });

    it('returns null when nothing was remembered', () => {
      expect(takeOAuthFlow()).toBeNull();
    });

    it('refuses a flow that is older than the limit, and forgets it', () => {
      const t0 = 1_000_000;
      rememberOAuthFlow(flow, t0);
      expect(takeOAuthFlow(t0 + OAUTH_FLOW_MAX_AGE_MS + 1)).toBeNull();
      expect(sessionStorage.getItem('indifferent.oauthFlow')).toBeNull();
    });

    it('accepts a flow just inside the limit', () => {
      const t0 = 1_000_000;
      rememberOAuthFlow(flow, t0);
      expect(takeOAuthFlow(t0 + OAUTH_FLOW_MAX_AGE_MS)).not.toBeNull();
    });

    it('refuses a flow dated in the future', () => {
      rememberOAuthFlow(flow, 5_000_000);
      expect(takeOAuthFlow(1_000_000)).toBeNull();
    });

    it('refuses what it cannot read', () => {
      for (const junk of ['not json', '{}', '[]', 'null', '{"state":"short","verifier":"","createdAt":1}',
        JSON.stringify({ state: 's'.repeat(43), verifier: 5, createdAt: Date.now() }),
        JSON.stringify({ state: 's'.repeat(43), verifier: '', createdAt: 'yesterday' })]) {
        sessionStorage.setItem('indifferent.oauthFlow', junk);
        expect(takeOAuthFlow()).withContext(junk).toBeNull();
      }
    });
  });
});
