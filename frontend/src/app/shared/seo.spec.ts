import { setCanonical, setMetaTag, setPageSeo } from './seo';

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
      title: 'Add Text & Logo Watermarks to Images and Videos | Indifferent.fun',
      description: 'Add text or logo watermarks to JPG, PNG, WebP, and MP4 files.',
      canonical: 'https://indifferent.fun/tools/add-watermark',
    });

    expect(document.title).toContain('Add Text & Logo Watermarks');
    const desc = head.querySelector('meta[name="description"]');
    expect(desc?.getAttribute('content')).toContain('JPG, PNG, WebP, and MP4');
    expect(head.querySelector('meta[property="og:title"]')?.getAttribute('content')).toContain(
      'Add Text & Logo Watermarks',
    );
    expect(head.querySelector('meta[property="og:description"]')?.getAttribute('content')).toContain(
      'JPG, PNG, WebP, and MP4',
    );
    expect(head.querySelector('meta[name="twitter:title"]')?.getAttribute('content')).toContain(
      'Add Text & Logo Watermarks',
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
});
