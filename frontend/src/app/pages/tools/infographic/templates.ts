import { InfographicDoc, LIMITS, Section } from './infographic.model';

/**
 * The templates of the infographic maker (#217). A template is data: how many
 * sections it takes, what it calls the small caption, and the starting text.
 * The artboard draws each `layout` (infographic-canvas.component.ts).
 * Categories follow the issue; the rest of the 20 arrive in later PRs.
 */
export type Layout = 'points' | 'concept' | 'steps' | 'compare' | 'timeline';

export interface Template {
  id: string;
  name: string;
  category: string;
  description: string;
  layout: Layout;
  minSections: number;
  maxSections: number;
  /** What the optional caption above each heading is called, or '' when the template does not show one. */
  labelName: string;
  /** True when each line of a section's body is a bullet. */
  bullets: boolean;
  /** The text a new document starts with. */
  title: string;
  subtitle: string;
  footer: string;
  sections: { label: string; heading: string; body: string }[];
}

export const TEMPLATES: Template[] = [
  {
    id: 'key-points',
    name: 'Key points',
    category: 'Educational',
    description: 'A title and a numbered list of points.',
    layout: 'points',
    minSections: 2,
    maxSections: 8,
    labelName: '',
    bullets: false,
    title: 'Five habits that make learning stick',
    subtitle: 'Small changes you can start today.',
    footer: 'Save this for later',
    sections: [
      { label: '', heading: 'Study in short bursts', body: 'Twenty-five focused minutes, then a five minute break.' },
      { label: '', heading: 'Test yourself', body: 'Close the book and write down what you remember.' },
      { label: '', heading: 'Space it out', body: 'Review a topic after a day, a week and a month.' },
      { label: '', heading: 'Teach someone', body: 'Explaining an idea shows you what you have not understood.' },
      { label: '', heading: 'Sleep well', body: 'Memory is stored while you sleep, so protect those hours.' },
    ],
  },
  {
    id: 'concept',
    name: 'Concept explainer',
    category: 'Educational',
    description: 'One big idea, explained in short cards.',
    layout: 'concept',
    minSections: 3,
    maxSections: 6,
    labelName: '',
    bullets: false,
    title: 'What is photosynthesis?',
    subtitle: 'How plants turn light into food.',
    footer: 'Biology basics',
    sections: [
      { label: '', heading: 'Light in', body: 'Leaves catch sunlight with a green pigment called chlorophyll.' },
      { label: '', heading: 'Water and air', body: 'Roots bring up water, and leaves take in carbon dioxide.' },
      { label: '', heading: 'Sugar made', body: 'The plant builds glucose, its food, from those three things.' },
      { label: '', heading: 'Oxygen out', body: 'Oxygen is released into the air as a by-product.' },
    ],
  },
  {
    id: 'steps',
    name: 'Numbered steps',
    category: 'Step by step',
    description: 'A how-to, one numbered step at a time.',
    layout: 'steps',
    minSections: 2,
    maxSections: 8,
    labelName: '',
    bullets: false,
    title: 'How to make masala chai',
    subtitle: 'Four steps, ten minutes.',
    footer: 'Serves two',
    sections: [
      { label: '', heading: 'Boil the water', body: 'Bring two cups of water to a boil with a crushed cardamom pod.' },
      { label: '', heading: 'Add the tea', body: 'Stir in two spoons of tea leaves and a pinch of ginger.' },
      { label: '', heading: 'Pour the milk', body: 'Add a cup of milk and let it rise once, then lower the heat.' },
      { label: '', heading: 'Strain and serve', body: 'Sweeten to taste and pour through a strainer into two cups.' },
    ],
  },
  {
    id: 'compare',
    name: 'A versus B',
    category: 'Comparison',
    description: 'Two options side by side, a bullet list each.',
    layout: 'compare',
    minSections: 2,
    maxSections: 2,
    labelName: '',
    bullets: true,
    title: 'Train or bus?',
    subtitle: 'Which is better for the daily commute?',
    footer: 'Your choice',
    sections: [
      { label: '', heading: 'Train', body: 'Fixed timetable\nNo traffic delays\nCrowded at peak hours\nCheaper per trip' },
      { label: '', heading: 'Bus', body: 'Goes door to door\nStuck in traffic\nSeats are easier to find\nRoutes change often' },
    ],
  },
  {
    id: 'timeline',
    name: 'Vertical timeline',
    category: 'Timeline',
    description: 'Events in order, each with a date or year.',
    layout: 'timeline',
    minSections: 2,
    maxSections: 8,
    labelName: 'Date or year',
    bullets: false,
    title: 'A short history of the web',
    subtitle: 'From one page to billions.',
    footer: 'Source: public records',
    sections: [
      { label: '1989', heading: 'The idea', body: 'Tim Berners-Lee proposes a way to share documents between computers.' },
      { label: '1993', heading: 'Open to all', body: 'The web is released into the public domain.' },
      { label: '2004', heading: 'Social web', body: 'Sites where people post and share become part of daily life.' },
      { label: '2007', heading: 'In your pocket', body: 'Smartphones put the whole web in one hand.' },
    ],
  },
];

export function templateById(id: string): Template | undefined {
  return TEMPLATES.find((t) => t.id === id);
}

let counter = 0;
/** A short id that is unique within a session, and across sessions in practice. */
export function newSectionId(): string {
  const random = typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID().slice(0, 8) : Math.random().toString(36).slice(2, 10);
  return `s${(counter++).toString(36)}${random}`;
}

export function blankSection(): Section {
  return { id: newSectionId(), label: '', heading: '', body: '' };
}

function trim(t: string, max: number): string {
  return Array.from(t).slice(0, max).join('');
}

/** A new document: the template's own text, with the design defaults. */
export function newDoc(template: Template, over: Partial<InfographicDoc> = {}): InfographicDoc {
  return {
    version: 1,
    templateId: template.id,
    format: 'portrait',
    themeId: 'paper',
    language: 'auto',
    direction: 'auto',
    title: trim(template.title, LIMITS.title),
    subtitle: trim(template.subtitle, LIMITS.subtitle),
    sections: template.sections.map((s) => ({ id: newSectionId(), label: s.label, heading: trim(s.heading, LIMITS.heading), body: trim(s.body, LIMITS.body) })),
    footer: trim(template.footer, LIMITS.footer),
    mark: false,
    ...over,
  };
}
