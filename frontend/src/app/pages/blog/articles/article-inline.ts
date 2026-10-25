export type InlineSegment =
  | { kind: 'text'; text: string }
  | { kind: 'bold'; text: string }
  | { kind: 'code'; text: string }
  | { kind: 'link'; text: string; href: string; external: boolean };

/** Splits article text into segments for the template. Supports **bold**,
 * `code` and [label](href). Nothing is ever interpreted as HTML. Unclosed or
 * malformed markers stay as plain text. */
export function parseInline(input: string): InlineSegment[] {
  const out: InlineSegment[] = [];
  const pattern = /\*\*([^*]+)\*\*|`([^`]+)`|\[([^\]]+)\]\(([^)\s]+)\)/g;
  let last = 0;
  let match: RegExpExecArray | null;
  while ((match = pattern.exec(input)) !== null) {
    if (match.index > last) out.push({ kind: 'text', text: input.slice(last, match.index) });
    if (match[1] !== undefined) {
      out.push({ kind: 'bold', text: match[1] });
    } else if (match[2] !== undefined) {
      out.push({ kind: 'code', text: match[2] });
    } else {
      const href = match[4];
      const external = /^https?:\/\//i.test(href);
      // Only site-relative paths and absolute http(s) URLs become links. A leading
      // "//" or "/\" is a protocol-relative URL that browsers send to another host.
      const sitePath = href.startsWith('/') && !href.startsWith('//') && !href.startsWith('/\\');
      if (external || sitePath) {
        out.push({ kind: 'link', text: match[3], href, external });
      } else {
        out.push({ kind: 'text', text: match[0] });
      }
    }
    last = pattern.lastIndex;
  }
  if (last < input.length) out.push({ kind: 'text', text: input.slice(last) });
  return out;
}

/** Plain text of a block string, markers removed. Used for word counts. */
export function stripInline(input: string): string {
  return parseInline(input)
    .map((s) => s.text)
    .join('');
}

/** Anchor id for a heading. */
export function slugifyHeading(text: string): string {
  return stripInline(text)
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}
