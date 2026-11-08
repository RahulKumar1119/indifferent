/** One calendar day of the activity chart, in local time. */
export interface DayBucket {
  /** Local date as yyyy-mm-dd. */
  key: string;
  date: Date;
  count: number;
}

const pad = (n: number) => String(n).padStart(2, '0');
const localKey = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

/**
 * Counts items per local day for the last `days` days, today included, oldest
 * first. Items outside the window or with a date that does not parse are left out.
 */
export function lastDays(items: { date: string }[], now: Date = new Date(), days = 14): DayBucket[] {
  const buckets: DayBucket[] = [];
  const index = new Map<string, DayBucket>();
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() - i);
    const b = { key: localKey(d), date: d, count: 0 };
    buckets.push(b);
    index.set(b.key, b);
  }
  for (const item of items) {
    const t = Date.parse(item.date);
    if (Number.isNaN(t)) continue;
    const b = index.get(localKey(new Date(t)));
    if (b) b.count++;
  }
  return buckets;
}

/** "Good morning" before noon, "Good afternoon" until 5 pm, then "Good evening". */
export function greetingFor(now: Date, name: string | null | undefined): string {
  const h = now.getHours();
  const part = h < 12 ? 'Good morning' : h < 17 ? 'Good afternoon' : 'Good evening';
  const first = (name ?? '').trim().split(/\s+/)[0];
  return first ? `${part}, ${first}` : part;
}
