import { greetingFor, lastDays } from './activity';

describe('lastDays', () => {
  const now = new Date(2026, 10, 8, 15, 0, 0);
  const at = (y: number, m: number, d: number, h = 10) => new Date(y, m, d, h).toISOString();

  it('returns fourteen local days, oldest first, ending today', () => {
    const b = lastDays([], now);
    expect(b.length).toBe(14);
    expect(b[13].key).toBe('2026-11-08');
    expect(b[0].key).toBe('2026-10-26');
    expect(b.every((x) => x.count === 0)).toBeTrue();
  });

  it('counts items by their local day and skips the rest', () => {
    const b = lastDays(
      [
        { date: at(2026, 10, 8) },
        { date: at(2026, 10, 8, 23) },
        { date: at(2026, 10, 1) },
        { date: at(2026, 9, 1) },
        { date: 'not a date' },
      ],
      now,
    );
    expect(b[13].count).toBe(2);
    expect(b.find((x) => x.key === '2026-11-01')?.count).toBe(1);
    expect(b.reduce((s, x) => s + x.count, 0)).toBe(3);
  });
});

describe('greetingFor', () => {
  it('uses the time of day and the first name', () => {
    expect(greetingFor(new Date(2026, 0, 1, 9), 'Asha Rao')).toBe('Good morning, Asha');
    expect(greetingFor(new Date(2026, 0, 1, 13), 'Asha')).toBe('Good afternoon, Asha');
    expect(greetingFor(new Date(2026, 0, 1, 20), 'Asha')).toBe('Good evening, Asha');
  });

  it('leaves the name out when there is none', () => {
    expect(greetingFor(new Date(2026, 0, 1, 9), '')).toBe('Good morning');
    expect(greetingFor(new Date(2026, 0, 1, 9), null)).toBe('Good morning');
  });
});
