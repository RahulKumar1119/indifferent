import { ANALYTICS_COOKIE_PATTERNS, ADVERTISING_COOKIE_PATTERNS } from './consent.model';
import { clearCookies, cookieDomains, cookieNames } from './cookies';

describe('cookies helper', () => {
  afterEach(() => {
    for (const n of cookieNames(document)) document.cookie = `${n}=; max-age=0; path=/`;
  });

  it('lists the host and each parent domain, and just the host for an IP or single label', () => {
    expect(cookieDomains('www.indifferent.fun')).toEqual([null, 'www.indifferent.fun', '.www.indifferent.fun', 'indifferent.fun', '.indifferent.fun']);
    expect(cookieDomains('localhost')).toEqual([null]);
    expect(cookieDomains('127.0.0.1')).toEqual([null]);
  });

  it('removes matching cookies and leaves the others', () => {
    document.cookie = '_ga=1; path=/';
    document.cookie = '_ga_D2GB04M42D=2; path=/';
    document.cookie = 'cwr_u=3; path=/';
    document.cookie = '_gcl_au=4; path=/';
    document.cookie = 'keepme=5; path=/';
    expect(clearCookies(document, ANALYTICS_COOKIE_PATTERNS).sort()).toEqual(['_ga', '_ga_D2GB04M42D', 'cwr_u']);
    expect(cookieNames(document).sort()).toEqual(['_gcl_au', 'keepme']);
    expect(clearCookies(document, ADVERTISING_COOKIE_PATTERNS)).toEqual(['_gcl_au']);
    expect(cookieNames(document)).toEqual(['keepme']);
  });

  it('does not match look-alike names', () => {
    for (const n of ['_gallery', 'x_ga', 'cwr', '_gcl']) {
      expect([...ANALYTICS_COOKIE_PATTERNS, ...ADVERTISING_COOKIE_PATTERNS].some((p) => p.test(n))).withContext(n).toBeFalse();
    }
  });
});
