/** Building blocks of a data-driven journal article. Text fields accept a tiny
 * inline syntax, rendered without innerHTML: **bold**, `code`, and
 * [label](/internal/path) or [label](https://external.example) links. */
export type ArticleBlock =
  | { type: 'h2'; text: string }
  | { type: 'h3'; text: string }
  | { type: 'p'; text: string }
  | { type: 'ul'; items: string[] }
  | { type: 'ol'; items: string[] }
  | { type: 'code'; text: string; label?: string }
  | { type: 'note'; title?: string; text: string }
  | { type: 'table'; head: string[]; rows: string[][]; caption?: string }
  | { type: 'figure'; src: string; alt: string; caption: string; width: number; height: number };

/** Every article content module exports its blocks under this name. */
export interface ArticleContentModule {
  blocks: ArticleBlock[];
}
