/**
 * Colour themes for the infographic maker (#217). Every pair that carries text
 * or a shape is checked for contrast in themes.spec.ts: body text 4.5:1 or
 * better, and the accent against the page 3:1 or better.
 */
export interface Theme {
  id: string;
  name: string;
  bg: string;
  surface: string;
  ink: string;
  /** Secondary text. */
  ink2: string;
  accent: string;
  /** Text and numbers drawn on the accent colour. */
  accentInk: string;
  rule: string;
}

export const THEMES: Theme[] = [
  { id: 'paper', name: 'Paper', bg: '#FBF6EE', surface: '#FFFFFF', ink: '#231F1A', ink2: '#4A4238', accent: '#BC5227', accentInk: '#FFFBF6', rule: '#E4D9C8' },
  { id: 'ink', name: 'Ink', bg: '#14110F', surface: '#211D1A', ink: '#F7F1E8', ink2: '#D9CFC0', accent: '#E0794F', accentInk: '#14110F', rule: '#3A332C' },
  { id: 'ocean', name: 'Ocean', bg: '#EEF5F8', surface: '#FFFFFF', ink: '#0F2A3A', ink2: '#34505F', accent: '#0B6E99', accentInk: '#FFFFFF', rule: '#CBDCE5' },
  { id: 'forest', name: 'Forest', bg: '#F0F5EE', surface: '#FFFFFF', ink: '#16281A', ink2: '#3B5340', accent: '#2F7A3D', accentInk: '#FFFFFF', rule: '#CFDDCC' },
  { id: 'berry', name: 'Berry', bg: '#F9F0F4', surface: '#FFFFFF', ink: '#2B1220', ink2: '#5A3A4C', accent: '#A8205A', accentInk: '#FFFFFF', rule: '#E8CFDB' },
  { id: 'sunrise', name: 'Sunrise', bg: '#FFF4E0', surface: '#FFFFFF', ink: '#2E2108', ink2: '#5B4515', accent: '#B45309', accentInk: '#FFFFFF', rule: '#EBD7AE' },
  { id: 'night', name: 'Night blue', bg: '#0E1726', surface: '#17233A', ink: '#F1F5FB', ink2: '#C3CEE0', accent: '#5FA8FF', accentInk: '#0E1726', rule: '#2C3B57' },
  { id: 'mono', name: 'Mono', bg: '#F2F2F2', surface: '#FFFFFF', ink: '#111111', ink2: '#444444', accent: '#111111', accentInk: '#FFFFFF', rule: '#D0D0D0' },
];

export function themeById(id: string): Theme {
  return THEMES.find((t) => t.id === id) ?? THEMES[0];
}
