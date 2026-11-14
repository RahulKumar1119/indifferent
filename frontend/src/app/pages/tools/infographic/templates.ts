import { InfographicDoc, LIMITS, Section } from './infographic.model';

/**
 * The templates of the infographic maker (#217). A template is data: how many
 * sections it takes, what it calls the small caption, and the starting text.
 * The artboard draws each `layout` (infographic-canvas.component.ts).
 * Categories follow the issue; the rest of the 20 arrive in later PRs.
 */
export type Layout = 'points' | 'concept' | 'steps' | 'compare' | 'timeline' | 'bars' | 'line' | 'donut' | 'numbers'
  | 'qa' | 'stairs' | 'checklist' | 'proscons' | 'versus' | 'zigzag' | 'milestones' | 'flow' | 'cycle' | 'mindmap' | 'tree';

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
  /** True for the statistics templates: each section is a data point and its heading is a number. */
  chart?: boolean;
  /** What the heading box is called when it is not "Heading", such as "Value". */
  headingName?: string;
  /** True when a section has no text box below the heading. */
  noBody?: boolean;
  /** A line of help under the sections. */
  hint?: string;
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
  {
    id: 'bar-chart',
    name: 'Bar chart',
    category: 'Statistics',
    description: 'Compare amounts with horizontal bars.',
    layout: 'bars',
    minSections: 2,
    maxSections: 8,
    labelName: 'Name',
    bullets: false,
    chart: true,
    headingName: 'Value',
    noBody: true,
    hint: 'Type a number in each value box, like 12, 3.5 or 40%. Bars are drawn to scale from zero.',
    title: 'Where our study time goes',
    subtitle: 'Hours per week, sample data',
    footer: 'Replace the sample numbers with your own',
    sections: [
      { label: 'Maths', heading: '12', body: '' },
      { label: 'Science', heading: '9', body: '' },
      { label: 'Languages', heading: '7', body: '' },
      { label: 'History', heading: '5', body: '' },
      { label: 'Art', heading: '3', body: '' },
    ],
  },
  {
    id: 'line-chart',
    name: 'Line chart',
    category: 'Statistics',
    description: 'Show how a number changes over time.',
    layout: 'line',
    minSections: 3,
    maxSections: 10,
    labelName: 'Point name',
    bullets: false,
    chart: true,
    headingName: 'Value',
    noBody: true,
    hint: 'Points are drawn left to right in the order of the sections. Type a number in each value box.',
    title: 'Steps walked this week',
    subtitle: 'Sample data',
    footer: 'Replace the sample numbers with your own',
    sections: [
      { label: 'Mon', heading: '4200', body: '' },
      { label: 'Tue', heading: '6100', body: '' },
      { label: 'Wed', heading: '5300', body: '' },
      { label: 'Thu', heading: '7800', body: '' },
      { label: 'Fri', heading: '6900', body: '' },
    ],
  },
  {
    id: 'donut-chart',
    name: 'Donut chart',
    category: 'Statistics',
    description: 'Show how a whole is split into parts.',
    layout: 'donut',
    minSections: 2,
    maxSections: 6,
    labelName: 'Name',
    bullets: false,
    chart: true,
    headingName: 'Value',
    noBody: true,
    hint: 'Each slice is its value as a share of the total. Type a number in each value box.',
    title: 'How a school day is spent',
    subtitle: 'Share of the day, sample data',
    footer: 'Replace the sample numbers with your own',
    sections: [
      { label: 'Lessons', heading: '40', body: '' },
      { label: 'Homework', heading: '25', body: '' },
      { label: 'Breaks', heading: '15', body: '' },
      { label: 'Sport', heading: '10', body: '' },
      { label: 'Free time', heading: '10', body: '' },
    ],
  },
  {
    id: 'big-numbers',
    name: 'Key numbers',
    category: 'Statistics',
    description: 'Large figures with a short caption each.',
    layout: 'numbers',
    minSections: 2,
    maxSections: 6,
    labelName: 'Name',
    bullets: false,
    chart: true,
    headingName: 'Number',
    hint: 'The number is shown as you type it, so you can add a unit, like 96% or 1,200+.',
    title: 'Our year in numbers',
    subtitle: 'Sample data',
    footer: 'Replace the sample numbers with your own',
    sections: [
      { label: 'Students', heading: '1,200', body: 'Across all classes' },
      { label: 'Teachers', heading: '48', body: 'Full time and visiting' },
      { label: 'Pass rate', heading: '96%', body: 'Up from 91% last year' },
      { label: 'Awards', heading: '12', body: 'State and national' },
    ],
  },
  {
    id: 'quiz-cards',
    name: 'Question cards',
    category: 'Educational',
    description: 'Questions with their answers underneath.',
    layout: 'qa',
    minSections: 2,
    maxSections: 8,
    labelName: '',
    bullets: false,
    title: 'Quick quiz: the water cycle',
    subtitle: 'Cover the answers and test yourself.',
    footer: 'Share it with your class',
    sections: [
      { label: '', heading: 'What makes water rise into the air?', body: 'The sun heats water in oceans and lakes, and it evaporates.' },
      { label: '', heading: 'What are clouds made of?', body: 'Tiny drops of water that cooled and gathered around dust.' },
      { label: '', heading: 'What is rain, in one line?', body: 'Cloud droplets that grew too heavy to stay in the air.' },
      { label: '', heading: 'Where does the water go next?', body: 'Into rivers and the ground, then back to the sea.' },
    ],
  },
  {
    id: 'staircase',
    name: 'Staircase',
    category: 'Step by step',
    description: 'Steps that climb, one level at a time.',
    layout: 'stairs',
    minSections: 3,
    maxSections: 6,
    labelName: '',
    bullets: false,
    title: 'Learn a new skill in four levels',
    subtitle: 'Each step builds on the last.',
    footer: 'Start with step one today',
    sections: [
      { label: '', heading: 'Try it', body: 'Pick one small thing and do it badly.' },
      { label: '', heading: 'Practise', body: 'Repeat it for ten minutes a day.' },
      { label: '', heading: 'Get feedback', body: 'Show someone and ask what to fix.' },
      { label: '', heading: 'Teach it', body: 'Explaining it to a friend makes it stick.' },
    ],
  },
  {
    id: 'checklist',
    name: 'Checklist',
    category: 'Step by step',
    description: 'A list with a tick box for each item.',
    layout: 'checklist',
    minSections: 2,
    maxSections: 10,
    labelName: '',
    bullets: false,
    hint: 'Each section is one tick box. Leave the text empty to show just the heading.',
    title: 'Before you press publish',
    subtitle: 'Check each one first.',
    footer: 'Print it or save it',
    sections: [
      { label: '', heading: 'Read it out loud', body: 'Awkward sentences show up when you hear them.' },
      { label: '', heading: 'Check the numbers', body: 'Every figure should have a source.' },
      { label: '', heading: 'Add a clear title', body: 'Say what the reader gets in the first line.' },
      { label: '', heading: 'Test on a phone', body: 'Most people will read it on a small screen.' },
      { label: '', heading: 'Ask one person', body: 'A fresh pair of eyes finds what you missed.' },
    ],
  },
  {
    id: 'pros-cons',
    name: 'Pros and cons',
    category: 'Comparison',
    description: 'Two lists: what is good and what is not.',
    layout: 'proscons',
    minSections: 2,
    maxSections: 2,
    labelName: '',
    bullets: true,
    hint: 'This template has two lists: the first is drawn as the good side, the second as the other side. Put one point on each line.',
    title: 'Studying with music on',
    subtitle: 'Is it worth it?',
    footer: 'Your result may differ',
    sections: [
      { label: '', heading: 'Pros', body: 'Helps you start\nBlocks out noise\nMakes long sessions feel shorter' },
      { label: '', heading: 'Cons', body: 'Words can distract you\nVolume tends to creep up\nHard to notice when you are tired' },
    ],
  },
  {
    id: 'versus',
    name: 'Side by side',
    category: 'Comparison',
    description: 'Two options as two cards, with a divider.',
    layout: 'versus',
    minSections: 2,
    maxSections: 2,
    labelName: '',
    bullets: false,
    hint: 'This template compares exactly two things.',
    title: 'Paper notes or a notes app?',
    subtitle: 'Two good ways to remember.',
    footer: 'Pick what you will actually use',
    sections: [
      { label: '', heading: 'Paper', body: 'Slower, so you think about each line. Nothing to charge, nothing to notify you.' },
      { label: '', heading: 'App', body: 'Searchable, easy to fix and always with you. Can be tempting to open other apps.' },
    ],
  },
  {
    id: 'zigzag',
    name: 'Zigzag timeline',
    category: 'Timeline',
    description: 'Events on both sides of a centre line.',
    layout: 'zigzag',
    minSections: 3,
    maxSections: 8,
    labelName: 'Date',
    bullets: false,
    title: 'How the web grew up',
    subtitle: 'From a few pages to everyone.',
    footer: 'Dates are approximate',
    sections: [
      { label: '1991', heading: 'The first website', body: 'A page that explained what the web was.' },
      { label: '1998', heading: 'Search for everyone', body: 'Finding things became as easy as typing.' },
      { label: '2005', heading: 'Video for all', body: 'Anyone could share a clip with the world.' },
      { label: '2010', heading: 'Phones as screens', body: 'The web moved into every pocket.' },
    ],
  },
  {
    id: 'milestones',
    name: 'Milestones',
    category: 'Timeline',
    description: 'Dated cards joined by a line.',
    layout: 'milestones',
    minSections: 3,
    maxSections: 6,
    labelName: 'Date',
    bullets: false,
    title: 'Our project, so far',
    subtitle: 'Four dates that mattered.',
    footer: 'Updated this month',
    sections: [
      { label: 'Jan', heading: 'Idea', body: 'A sketch on the back of an envelope.' },
      { label: 'Mar', heading: 'First test', body: 'Ten people tried it and told us the truth.' },
      { label: 'Jun', heading: 'Launch', body: 'Open to everyone, free to use.' },
      { label: 'Oct', heading: '10,000 users', body: 'A big thank you to every one of you.' },
    ],
  },
  {
    id: 'flow-chart',
    name: 'Flow chart',
    category: 'Process',
    description: 'Boxes in order, joined by arrows.',
    layout: 'flow',
    minSections: 3,
    maxSections: 6,
    labelName: '',
    bullets: false,
    title: 'From idea to published video',
    subtitle: 'The path every video follows.',
    footer: 'Takes about ten minutes',
    sections: [
      { label: '', heading: 'Write', body: 'Type or paste your story.' },
      { label: '', heading: 'Choose pictures', body: 'Pick a free photo for every scene.' },
      { label: '', heading: 'Add a voice', body: 'Select a voice and a language.' },
      { label: '', heading: 'Download', body: 'Save the finished video.' },
    ],
  },
  {
    id: 'cycle',
    name: 'Cycle',
    category: 'Process',
    description: 'Steps in a loop that goes round and round.',
    layout: 'cycle',
    minSections: 3,
    maxSections: 6,
    labelName: '',
    bullets: false,
    noBody: true,
    hint: 'Keep each step short, a word or a few words. The steps go clockwise (or the other way for right-to-left text).',
    title: 'The study cycle',
    subtitle: 'Round and round, getting better.',
    footer: 'Repeat as often as you like',
    sections: [
      { label: '', heading: 'Learn', body: '' },
      { label: '', heading: 'Practise', body: '' },
      { label: '', heading: 'Test', body: '' },
      { label: '', heading: 'Review', body: '' },
    ],
  },
  {
    id: 'mind-map',
    name: 'Mind map',
    category: 'Mind map',
    description: 'One big idea with branches around it.',
    layout: 'mindmap',
    minSections: 3,
    maxSections: 8,
    labelName: '',
    bullets: false,
    headingName: 'Branch',
    hint: 'The title is the centre of the map. Each section is a branch around it.',
    title: 'Healthy habits',
    subtitle: 'Everything starts in the middle.',
    footer: '',
    sections: [
      { label: '', heading: 'Sleep', body: 'Eight hours, same time each night' },
      { label: '', heading: 'Food', body: 'Colourful plates, plenty of water' },
      { label: '', heading: 'Movement', body: 'Thirty minutes of anything active' },
      { label: '', heading: 'Mind', body: 'Breaks, hobbies and talking to friends' },
    ],
  },
  {
    id: 'tree',
    name: 'Tree diagram',
    category: 'Mind map',
    description: 'A top idea that splits into groups.',
    layout: 'tree',
    minSections: 2,
    maxSections: 4,
    labelName: '',
    bullets: true,
    headingName: 'Group',
    hint: 'The title is the top of the tree. Each section is a group below it, with one point per line.',
    title: 'Types of rock',
    subtitle: 'Three families, many examples.',
    footer: 'Earth science basics',
    sections: [
      { label: '', heading: 'Igneous', body: 'Formed from cooled lava\nGranite\nBasalt' },
      { label: '', heading: 'Sedimentary', body: 'Layers pressed together\nSandstone\nLimestone' },
      { label: '', heading: 'Metamorphic', body: 'Changed by heat and pressure\nMarble\nSlate' },
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
    items: [],
    ...over,
  };
}
