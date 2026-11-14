import {
  ArrowRight, BookOpen, Brain, Calendar, Check, CircleCheck, Clock, Flag, GraduationCap, Globe, Heart, Leaf, Lightbulb, Megaphone,
  MessageCircle, Pencil, Rocket, ShieldCheck, Smile, Sparkles, Star, Sun, Target, ThumbsUp, Trophy, Users, Zap,
} from 'lucide';

/**
 * The icons and shapes of the infographic maker (#217). The icons are from
 * Lucide (ISC licence), drawn as outlines. The shapes are our own, drawn filled.
 * Both are lists of simple SVG elements on a 24 by 24 grid, which the glyph
 * component builds with the DOM, so no markup text is ever parsed.
 */
export type GlyphNode = [tag: string, attrs: Record<string, string | number>];

export interface Glyph {
  id: string;
  /** What a person hears and sees as the name, in English. */
  label: string;
  kind: 'icon' | 'shape';
  nodes: GlyphNode[];
}

/** The only elements and attributes the glyph component will create. Anything else in a definition is ignored. */
export const GLYPH_TAGS: ReadonlySet<string> = new Set(['path', 'circle', 'rect', 'line', 'polyline', 'polygon', 'ellipse']);
export const GLYPH_ATTRS: ReadonlySet<string> = new Set([
  'd', 'cx', 'cy', 'r', 'x', 'y', 'width', 'height', 'rx', 'ry', 'x1', 'y1', 'x2', 'y2', 'points', 'fill-rule', 'clip-rule',
]);

const icon = (id: string, label: string, nodes: unknown): Glyph => ({ id, label, kind: 'icon', nodes: nodes as GlyphNode[] });
const shape = (id: string, label: string, nodes: GlyphNode[]): Glyph => ({ id, label, kind: 'shape', nodes });

/** The points of a five-pointed star on the 24 grid. */
function starPoints(): string {
  const pts: string[] = [];
  for (let i = 0; i < 10; i++) {
    const r = i % 2 === 0 ? 10.5 : 4.4;
    const a = (Math.PI / 5) * i - Math.PI / 2;
    pts.push(`${(12 + r * Math.cos(a)).toFixed(2)} ${(12.6 + r * Math.sin(a)).toFixed(2)}`);
  }
  return pts.join(' ');
}

export const SHAPES: Glyph[] = [
  shape('circle', 'Circle', [['circle', { cx: 12, cy: 12, r: 10 }]]),
  shape('rounded-square', 'Rounded square', [['rect', { x: 2.5, y: 2.5, width: 19, height: 19, rx: 5 }]]),
  shape('triangle', 'Triangle', [['polygon', { points: '12 2.5 22 20.5 2 20.5' }]]),
  shape('diamond', 'Diamond', [['polygon', { points: '12 1.5 22.5 12 12 22.5 1.5 12' }]]),
  shape('star-shape', 'Star', [['polygon', { points: starPoints() }]]),
  shape('hexagon', 'Hexagon', [['polygon', { points: '12 1.5 21.1 6.75 21.1 17.25 12 22.5 2.9 17.25 2.9 6.75' }]]),
  shape('ring', 'Ring', [['path', { d: 'M12 2a10 10 0 1 0 0 20a10 10 0 1 0 0-20zm0 4a6 6 0 1 1 0 12a6 6 0 1 1 0-12z', 'fill-rule': 'evenodd' }]]),
  shape('bar', 'Bar', [['rect', { x: 1, y: 9.5, width: 22, height: 5, rx: 2.5 }]]),
  shape('arrow-shape', 'Arrow', [['polygon', { points: '1.5 8.5 13.5 8.5 13.5 3.5 22.5 12 13.5 20.5 13.5 15.5 1.5 15.5' }]]),
];

export const ICONS: Glyph[] = [
  icon('lightbulb', 'Light bulb', Lightbulb),
  icon('book-open', 'Open book', BookOpen),
  icon('graduation-cap', 'Graduation cap', GraduationCap),
  icon('brain', 'Brain', Brain),
  icon('pencil', 'Pencil', Pencil),
  icon('target', 'Target', Target),
  icon('check', 'Tick', Check),
  icon('circle-check', 'Tick in a circle', CircleCheck),
  icon('star', 'Star outline', Star),
  icon('heart', 'Heart', Heart),
  icon('thumbs-up', 'Thumbs up', ThumbsUp),
  icon('smile', 'Smile', Smile),
  icon('trophy', 'Trophy', Trophy),
  icon('flag', 'Flag', Flag),
  icon('rocket', 'Rocket', Rocket),
  icon('zap', 'Lightning', Zap),
  icon('sparkles', 'Sparkles', Sparkles),
  icon('clock', 'Clock', Clock),
  icon('calendar', 'Calendar', Calendar),
  icon('globe', 'Globe', Globe),
  icon('users', 'People', Users),
  icon('message-circle', 'Speech bubble', MessageCircle),
  icon('megaphone', 'Megaphone', Megaphone),
  icon('shield-check', 'Shield with a tick', ShieldCheck),
  icon('arrow-right', 'Arrow', ArrowRight),
  icon('leaf', 'Leaf', Leaf),
  icon('sun', 'Sun', Sun),
];

export const GLYPHS: Glyph[] = [...SHAPES, ...ICONS];

const BY_ID = new Map(GLYPHS.map((g) => [g.id, g]));

export function glyphById(id: string): Glyph | undefined {
  return BY_ID.get(id);
}
