import {
  ROBOTS_INDEX,
  ROBOTS_NOINDEX,
  clearJsonLd,
  robotsForUrl,
  setCanonical,
  setJsonLd,
  setMetaTag,
  setPageSeo,
  setRobots,
} from './seo';

describe('seo helpers', () => {
  let head: HTMLElement;

  beforeEach(() => {
    // Isolated head so tests never touch the real document.
    head = document.createElement('head');
    document.documentElement.appendChild(head);
    spyOnProperty(document, 'head', 'get').and.returnValue(head);
  });

  afterEach(() => {
    head.remove();
  });

  it('setCanonical updates the existing canonical link instead of duplicating', () => {
    const link = document.createElement('link');
    link.rel = 'canonical';
    link.href = 'https://indifferent.fun/';
    head.appendChild(link);

    setCanonical(document, 'https://indifferent.fun/tools/add-watermark');

    expect(head.querySelectorAll('link[rel="canonical"]').length).toBe(1);
    expect(link.getAttribute('href')).toBe('https://indifferent.fun/tools/add-watermark');
  });

  it('setCanonical creates the link when absent', () => {
    setCanonical(document, 'https://indifferent.fun/about');

    const links = head.querySelectorAll('link[rel="canonical"]');
    expect(links.length).toBe(1);
    expect(links[0].getAttribute('href')).toBe('https://indifferent.fun/about');
  });

  it('setPageSeo writes consistent title, description, OG, twitter and canonical', () => {
    setPageSeo(document, {
      title: 'Watermark Images and Videos Free | Indifferent',
      description: 'Add text or logo watermarks to JPG, PNG, WebP, and MP4 files.',
      canonical: 'https://indifferent.fun/tools/add-watermark',
    });

    expect(document.title).toContain('Watermark Images and Videos Free');
    const desc = head.querySelector('meta[name="description"]');
    expect(desc?.getAttribute('content')).toContain('JPG, PNG, WebP, and MP4');
    expect(head.querySelector('meta[property="og:title"]')?.getAttribute('content')).toContain(
      'Watermark Images and Videos Free',
    );
    expect(head.querySelector('meta[property="og:description"]')?.getAttribute('content')).toContain(
      'JPG, PNG, WebP, and MP4',
    );
    expect(head.querySelector('meta[name="twitter:title"]')?.getAttribute('content')).toContain(
      'Watermark Images and Videos Free',
    );
    expect(head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
      'https://indifferent.fun/tools/add-watermark',
    );
  });

  it('setMetaTag never duplicates an existing tag', () => {
    setMetaTag(document, 'name', 'description', 'first');
    setMetaTag(document, 'name', 'description', 'second');

    const tags = head.querySelectorAll('meta[name="description"]');
    expect(tags.length).toBe(1);
    expect(tags[0].getAttribute('content')).toBe('second');
  });

  describe('robots', () => {
    it('setRobots writes one robots tag and replaces it on the next call', () => {
      setRobots(document, ROBOTS_NOINDEX);
      setRobots(document, ROBOTS_INDEX);

      const tags = head.querySelectorAll('meta[name="robots"]');
      expect(tags.length).toBe(1);
      expect(tags[0].getAttribute('content')).toBe('index, follow');
    });

    it('keeps signed-in areas out of the index', () => {
      for (const url of [
        '/dashboard',
        '/projects',
        '/projects/abc/progress',
        '/new',
        '/shorts',
        '/shorts?x=1',
        '/shorts/history',
        '/shorts/abc/clips',
        '/account',
        '/profile',
        '/profile/settings',
        '/settings',
        '/auth/callback?code=1',
      ]) {
        expect(robotsForUrl(url)).withContext(url).toBe('noindex, nofollow');
      }
    });

    it('leaves public pages indexable, including /shorts-maker which starts with /shorts', () => {
      for (const url of [
        '/',
        '/shorts-maker',
        '/features/ai-shorts',
        '/tools/add-watermark',
        '/blog',
        '/blog/podcast-to-youtube-shorts',
        '/help',
        '/privacy#cookies',
        '/newsletter',
      ]) {
        expect(robotsForUrl(url)).withContext(url).toBe(ROBOTS_INDEX);
      }
    });
  });

  describe('json-ld', () => {
    it('setJsonLd keeps one block per id and updates it in place', () => {
      setJsonLd(document, 'article', { '@type': 'Article', headline: 'First' });
      setJsonLd(document, 'article', { '@type': 'Article', headline: 'Second' });
      setJsonLd(document, 'breadcrumbs', { '@type': 'BreadcrumbList' });

      const blocks = head.querySelectorAll('script[type="application/ld+json"]');
      expect(blocks.length).toBe(2);
      expect(JSON.parse(head.querySelector('script[data-seo-id="article"]')!.textContent!).headline).toBe('Second');
    });

    it('clearJsonLd removes the blocks the helper created and nothing else', () => {
      setJsonLd(document, 'article', { '@type': 'Article' });
      const other = document.createElement('script');
      other.type = 'application/ld+json';
      other.text = '{}';
      head.appendChild(other);

      clearJsonLd(document);

      expect(head.querySelectorAll('script[data-seo-id]').length).toBe(0);
      expect(head.querySelectorAll('script[type="application/ld+json"]').length).toBe(1);
    });
  });
});
