import { InfographicDoc } from './infographic.model';

/**
 * Undo and redo for the infographic maker (#217). It keeps whole documents,
 * which are small and never edited in place. Typing in one field is one step,
 * not one per letter: changes with the same `key` made within `windowMs` of
 * each other are merged into the first.
 */
export class DocHistory {
  private past: InfographicDoc[] = [];
  private future: InfographicDoc[] = [];
  private lastKey = '';
  private lastAt = 0;

  constructor(
    private current: InfographicDoc,
    private readonly limit = 100,
    private readonly windowMs = 800,
  ) {}

  get present(): InfographicDoc {
    return this.current;
  }
  get canUndo(): boolean {
    return this.past.length > 0;
  }
  get canRedo(): boolean {
    return this.future.length > 0;
  }

  /** Records a change. The same document is ignored. A new change drops the redo steps. */
  push(next: InfographicDoc, key = '', now = Date.now()): boolean {
    if (next === this.current) return false;
    const merge = key !== '' && key === this.lastKey && now - this.lastAt <= this.windowMs && this.past.length > 0;
    if (!merge) {
      this.past.push(this.current);
      if (this.past.length > this.limit) this.past.shift();
    }
    this.current = next;
    this.future = [];
    this.lastKey = key;
    this.lastAt = now;
    return true;
  }

  /** Ends the current run of merged changes, so the next change starts a new undo step even for the same key. */
  endGroup(): void {
    this.lastKey = '';
  }

  undo(): InfographicDoc | null {
    const previous = this.past.pop();
    if (!previous) return null;
    this.future.push(this.current);
    this.current = previous;
    this.lastKey = '';
    return previous;
  }

  redo(): InfographicDoc | null {
    const next = this.future.pop();
    if (!next) return null;
    this.past.push(this.current);
    this.current = next;
    this.lastKey = '';
    return next;
  }

  /** Starts over from a document, with nothing to undo. Used for a draft loaded from the browser. */
  reset(doc: InfographicDoc): void {
    this.current = doc;
    this.past = [];
    this.future = [];
    this.lastKey = '';
  }
}
