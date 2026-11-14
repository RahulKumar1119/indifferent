/**
 * Styles of the infographic artboard (#217). The artboard has a fixed design
 * size (1080 wide and so on) and is shown scaled to fit the screen, so every
 * length here is in design pixels. Only logical properties (inline, block,
 * start, end) are used, so a right-to-left document mirrors by itself.
 * `--k` is the fit factor: the component lowers it until the text fits.
 */
export const CANVAS_STYLES = `
  :host { display: block; }
  .holder { position: relative; overflow: hidden; border-radius: 6px; box-shadow: 0 0 0 1px rgb(0 0 0 / 0.12), 0 10px 30px -14px rgb(0 0 0 / 0.35); }
  .stage { box-sizing: border-box; position: absolute; inset-block-start: 0; inset-inline-start: 0; transform-origin: top left; overflow: hidden;
    display: flex; flex-direction: column; gap: calc(var(--gap) * var(--k)); padding: calc(var(--pad) * var(--k)); background: var(--bg); color: var(--ink);
    font-family: 'Noto Sans', 'Noto Sans Gujarati', sans-serif; font-weight: 400; line-height: var(--lh); font-synthesis: none; }
  .stage *, .stage *::before, .stage *::after { box-sizing: border-box; letter-spacing: 0; text-transform: none; overflow-wrap: break-word; }
  /* Headings keep whole words: the fit makes the text smaller until the longest word fits, and only then lets it break. */
  .stage h1, .stage h2 { overflow-wrap: normal; }
  .stage[data-break] h1, .stage[data-break] h2 { overflow-wrap: anywhere; }
  .stage { --pad: 72px; --gap: 36px; --h1: 84px; --sub: 36px; --h2: 44px; --body: 32px; --small: 28px; --num: 76px; --cols: 1; }
  .stage.f-square { --pad: 56px; --gap: 22px; --h1: 66px; --sub: 28px; --h2: 36px; --body: 26px; --small: 22px; --num: 60px; }
  .stage.f-landscape { --pad: 56px; --gap: 26px; --h1: 70px; --sub: 30px; --h2: 38px; --body: 28px; --small: 24px; --num: 64px; --cols: 2; }
  h1, h2, p, ul, ol { margin: 0; padding: 0; }
  h1 { font-size: calc(var(--h1) * var(--k)); line-height: 1.2; font-weight: 700; text-wrap: balance; }
  .sub { margin-block-start: calc(12px * var(--k)); font-size: calc(var(--sub) * var(--k)); color: var(--ink2); }
  .list { flex: 1 0 auto; display: grid; align-content: start; gap: calc(var(--gap) * 0.7 * var(--k)); grid-template-columns: repeat(var(--cols), minmax(0, 1fr)); list-style: none; }
  .card, .item { background: var(--surface); border: 2px solid var(--rule); border-radius: calc(24px * var(--k)); padding: calc(var(--gap) * 0.8 * var(--k)); }
  .item { position: relative; display: flex; gap: calc(var(--gap) * 0.8 * var(--k)); align-items: flex-start; border-inline-start: calc(12px * var(--k)) solid var(--accent); }
  .num { flex: none; display: grid; place-items: center; width: calc(var(--num) * var(--k)); height: calc(var(--num) * var(--k)); border-radius: 50%; background: var(--accent);
    color: var(--accent-ink); font-size: calc(var(--num) * 0.5 * var(--k)); font-weight: 700; line-height: 1; }
  .l-steps .num { border-radius: calc(var(--num) * 0.28 * var(--k)); }
  .l-steps .item:not(:last-child)::after { content: ''; position: absolute; inset-inline-start: calc(var(--gap) * 0.8 * var(--k) + var(--num) * 0.5 * var(--k) - 2px); inset-block-start: 100%; width: 4px; height: calc(var(--gap) * 0.7 * var(--k)); background: var(--accent); }
  .l-steps .list, .l-timeline .list { --cols: 1; }
  .l-timeline .list { flex: none; }
  .txt { min-width: 0; flex: 1; }
  .cap { margin-block-end: calc(4px * var(--k)); font-size: calc(var(--small) * var(--k)); font-weight: 700; color: var(--ink2); }
  h2 { font-size: calc(var(--h2) * var(--k)); line-height: 1.25; font-weight: 700; text-wrap: balance; }
  .body { margin-block-start: calc(8px * var(--k)); font-size: calc(var(--body) * var(--k)); color: var(--ink2); white-space: pre-line; }
  .l-concept .head { padding: calc(var(--gap) * 0.9 * var(--k)) calc(var(--gap) * 1.1 * var(--k)); border-radius: calc(32px * var(--k)); background: var(--accent); color: var(--accent-ink); }
  .l-concept .head .sub { color: var(--accent-ink); }
  .l-concept .list { --cols: 2; }
  .l-concept.f-landscape .list { --cols: 3; }
  .l-concept .card { border-block-start: calc(10px * var(--k)) solid var(--accent); }
  .l-compare .list { --cols: 2; align-items: stretch; }
  .side { display: flex; flex-direction: column; padding: 0; overflow: hidden; }
  .band { padding: calc(var(--gap) * 0.6 * var(--k)) calc(var(--gap) * 0.8 * var(--k)); background: var(--accent); color: var(--accent-ink); }
  .side:nth-child(2) .band { background: var(--ink); color: var(--bg); }
  .bul { margin: 0; padding: calc(var(--gap) * 0.8 * var(--k)); padding-inline-start: calc(var(--gap) * 0.8 * var(--k) + 40px * var(--k)); list-style: disc; font-size: calc(var(--body) * var(--k)); color: var(--ink2); }
  .bul li + li { margin-block-start: calc(12px * var(--k)); }
  .bul li::marker { color: var(--accent); }
  .timeline { padding-inline-start: calc(60px * var(--k)); position: relative; }
  .timeline::before { content: ''; position: absolute; inset-inline-start: calc(20px * var(--k)); inset-block: calc(10px * var(--k)); width: calc(6px * var(--k)); background: var(--rule); border-radius: 3px; }
  .ev { position: relative; }
  .ev::before { content: ''; position: absolute; inset-inline-start: calc(-60px * var(--k) + 6px * var(--k)); inset-block-start: calc(14px * var(--k)); width: calc(32px * var(--k)); height: calc(32px * var(--k)); border-radius: 50%; background: var(--accent); border: calc(6px * var(--k)) solid var(--bg); }
  .when { display: inline-block; margin-block-end: calc(6px * var(--k)); padding: calc(2px * var(--k)) calc(14px * var(--k)); border-radius: 999px; background: var(--accent); color: var(--accent-ink); font-size: calc(var(--small) * var(--k)); font-weight: 700; }
  footer { margin-block-start: auto; display: flex; justify-content: space-between; gap: calc(24px * var(--k)); font-size: calc(var(--small) * var(--k)); color: var(--ink2); }
  .mark { margin-inline-start: auto; font-weight: 700; }
`;
