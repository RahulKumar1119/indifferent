import { parseInline, slugifyHeading, stripInline } from './article-inline';

describe('article inline text', () => {
  it('returns plain text unchanged', () => {
    expect(parseInline('Just words.')).toEqual([{ kind: 'text', text: 'Just words.' }]);
  });

  it('splits bold, code and links around the text between them', () => {
    expect(parseInline('Use **ffmpeg** with `-c copy` or read [the guide](/blog/mp4-vs-webm-formats).')).toEqual([
      { kind: 'text', text: 'Use ' },
      { kind: 'bold', text: 'ffmpeg' },
      { kind: 'text', text: ' with ' },
      { kind: 'code', text: '-c copy' },
      { kind: 'text', text: ' or read ' },
      { kind: 'link', text: 'the guide', href: '/blog/mp4-vs-webm-formats', external: false },
      { kind: 'text', text: '.' },
    ]);
  });

  it('marks absolute http and https links as external', () => {
    const [link] = parseInline('[MDN](https://developer.mozilla.org/en-US/docs/Web/Media)');
    expect(link).toEqual({ kind: 'link', text: 'MDN', href: 'https://developer.mozilla.org/en-US/docs/Web/Media', external: true });
  });

  it('never turns a script or relative URL into a link', () => {
    for (const href of ['javascript:alert(1)', 'data:text/html,x', '//evil.example/x', 'blog/page', 'mailto:a@b.c']) {
      const segments = parseInline(`[click](${href})`);
      expect(segments.every((s) => s.kind === 'text')).withContext(href).toBeTrue();
    }
  });

  it('leaves unclosed markers as text and keeps HTML inert', () => {
    expect(parseInline('a **b and `c')).toEqual([{ kind: 'text', text: 'a **b and `c' }]);
    expect(parseInline('<script>alert(1)</script>')).toEqual([{ kind: 'text', text: '<script>alert(1)</script>' }]);
  });

  it('strips markers for word counts and builds anchor ids', () => {
    expect(stripInline('Read **this** and [that](/x) with `code`')).toBe('Read this and that with code');
    expect(slugifyHeading('Stage 3: ranking')).toBe('stage-3-ranking');
    expect(slugifyHeading('  What the tool does, and what stays on your device  ')).toBe('what-the-tool-does-and-what-stays-on-your-device');
  });
});
