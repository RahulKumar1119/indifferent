import {
  CoreIntegrityError,
  fetchVerified,
  FF_CORE_JS_SHA384,
  FF_CORE_JS_URL,
  FF_CORE_WASM_SHA384,
  FF_CORE_WASM_URL,
  loadVerifiedCore,
  sha384Base64,
} from './ffmpeg-core';

const originalFetch = globalThis.fetch.bind(globalThis);

describe('verified ffmpeg core', () => {
  const enc = (s: string) => new TextEncoder().encode(s).buffer as ArrayBuffer;

  it('hashes like openssl (known SHA-384 of "abc")', async () => {
    expect(await sha384Base64(enc('abc'))).toBe(
      'ywB1P0WjXou1oD1pmsZQBycsMqsO3tFjGotgWkP/W+2AhgcroefMI1i67KE0yCWn',
    );
  });

  it('pins SHA-384 values of the right length', () => {
    for (const h of [FF_CORE_JS_SHA384, FF_CORE_WASM_SHA384]) {
      expect(atob(h).length).toBe(48);
    }
  });

  function stubFetch(bodies: Record<string, string>): jasmine.Spy {
    return spyOn(globalThis, 'fetch').and.callFake(async (input: RequestInfo | URL) => {
      const url = String(input);
      return new Response(bodies[url] ?? 'x', { status: url in bodies ? 200 : 404 });
    });
  }

  it('refuses a core file whose bytes do not match the pinned hash', async () => {
    stubFetch({ [FF_CORE_JS_URL]: 'tampered', [FF_CORE_WASM_URL]: 'tampered' });
    await expectAsync(loadVerifiedCore()).toBeRejectedWithError(CoreIntegrityError);
  });

  it('refuses when the download fails', async () => {
    stubFetch({});
    await expectAsync(loadVerifiedCore()).toBeRejectedWithError(/Could not download/);
  });

  it('never fetches anything but the two pinned files', async () => {
    const spy = stubFetch({ [FF_CORE_JS_URL]: 'tampered', [FF_CORE_WASM_URL]: 'tampered' });
    await loadVerifiedCore().catch(() => undefined);
    const urls = spy.calls.allArgs().map((a) => String(a[0])).sort();
    expect(urls).toEqual([FF_CORE_JS_URL, FF_CORE_WASM_URL].sort());
  });

  it('accepts bytes that match the hash and returns a blob URL of those bytes', async () => {
    const abc = 'ywB1P0WjXou1oD1pmsZQBycsMqsO3tFjGotgWkP/W+2AhgcroefMI1i67KE0yCWn';
    stubFetch({ 'https://cdn.example/f.js': 'abc' });
    const url = await fetchVerified('https://cdn.example/f.js', abc, 'application/javascript');
    expect(url.startsWith('blob:')).toBeTrue();
    const back = await (await originalFetch(url)).text();
    expect(back).toBe('abc');
    URL.revokeObjectURL(url);
  });
});
