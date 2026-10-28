import type { ArticleBlock } from '../article-types';

export const blocks: ArticleBlock[] = [
  { type: 'h2', text: 'Where the effects live' },
  {
    type: 'p',
    text: "After you upload a file to the [AI Shorts generator](/features/ai-shorts), and before processing starts, an effects step lets you add motion and color to every clip the job produces. This guide covers each effect, the exact range of every control, and the five one-click presets. The effects are applied by the render servers with ffmpeg, so they are burned into the finished file. For how the clips are chosen and framed in the first place, see [how AI video clipping works](/blog/how-ai-video-clipping-works).",
  },
  {
    type: 'note',
    title: 'Set effects before processing, not after',
    text: "The effects panel is open only between upload and the start of processing. Once a job renders, its file is fixed and is not rendered again, so there is no way to add or change an effect on a finished clip. Decide the look first, or upload the file again to try a different one.",
  },

  { type: 'h2', text: 'Zoom punch-ins' },
  {
    type: 'p',
    text: "Zoom pushes in on the frame at times you choose, then eases back out. Intensity runs from 0 to 100, where 100 is a peak zoom of 1.5x. Each push-in is a punch-in with a start time and a duration from 0.5 to 6 seconds. You can place up to 10 punch-ins on one clip, and they cannot overlap. Use a short, strong punch-in to mark a reaction, or one long, gentle one across a whole clip for a slow creep.",
  },

  { type: 'h2', text: 'Color grade' },
  {
    type: 'p',
    text: "The grade applies one of four looks over the whole clip, scaled by an intensity from 0 to 100. The looks are cinematic, warm, cool and high-contrast. Intensity is a blend: 0 is the untouched picture and 100 is the full look, so a mid value keeps skin tones natural while still setting a mood. Cinematic is the one the presets reach for most.",
  },
  {
    type: 'table',
    caption: 'The four grade looks',
    head: ['Look', 'What it does'],
    rows: [
      ['Cinematic', 'Teal shadows and warmer highlights, the usual film tone.'],
      ['Warm', 'Pushes the whole picture toward amber.'],
      ['Cool', 'Pushes the whole picture toward blue.'],
      ['High-contrast', 'Deepens blacks and lifts highlights for a punchier image.'],
    ],
  },

  { type: 'h2', text: 'Speed ramping' },
  {
    type: 'p',
    text: "Speed changes playback over windows you set: slow motion below 1.0x, fast motion above it. Each window has a start, a duration of at least 0.3 seconds, and a rate from 0.25x to 4.0x. Outside a window the clip plays at normal speed. You can set up to 5 windows, and they cannot overlap. The audio is retimed with the picture, so a slow window drops the pitch and a fast one raises it, the same as pulling a clip's speed in an editor.",
  },
  {
    type: 'note',
    title: 'Speed and freeze are one or the other',
    text: "Speed ramping and freeze frames both work by cutting the clip into pieces and retiming them, and combining the two on one clip is not supported yet. The panel will not let you save a clip with both on. Pick the one the moment needs.",
  },

  { type: 'h2', text: 'Freeze frames and impact text' },
  {
    type: 'p',
    text: "A freeze holds a single frame still while the audio goes silent, then the rest of the clip carries on, moved later by the length of the hold. Each hold is from 0.2 to 3 seconds, you can place up to 5 on a clip, and together they can add at most 10 seconds. Holds must sit at least 0.1 seconds apart.",
  },
  {
    type: 'p',
    text: "Impact text is one short line drawn over the clip, the kind of word that lands on a freeze. It is at most 16 characters on a single line, shows for 0.3 to 4 seconds, and you can add up to 5 lines that do not overlap in time. The freeze effect's intensity sets how large the impact text is drawn.",
  },

  { type: 'h2', text: 'Camera shake' },
  {
    type: 'p',
    text: "Shake moves the frame around over windows you choose, for a hit of energy on a beat or a punchline. Intensity from 0 to 100 sets how far the frame travels. The picture is scaled up slightly first so the movement never exposes a black edge, which is why strong shake is still safe. Each window lasts 0.2 to 6 seconds, you can set up to 10, and they cannot overlap.",
  },

  { type: 'h2', text: 'Finish: grain, glow and light leaks' },
  {
    type: 'p',
    text: "The finish effect layers three film looks over the whole clip, each with its own strength from 0 to 100. Set any one to 0 to turn it off, so one finish can carry any mix of the three. An enabled finish needs at least one of them above 0.",
  },
  {
    type: 'ul',
    items: [
      'Grain adds fine film noise, strongest in the shadows.',
      'Glow softens and blooms the highlights for a dreamy feel.',
      'Light leak washes a warm streak across the frame, like light catching an old lens.',
    ],
  },

  { type: 'h2', text: 'Fade in and out' },
  {
    type: 'p',
    text: "Fade eases the clip up from black at the start and down to black at the end. Each side is a duration from 0 to 2 seconds, and setting a side to 0 turns that side off; an enabled fade needs at least one side above 0. The fade covers the finished picture, captions and impact text included, and the audio fades with it. This is a per-clip fade, not a cross-clip transition: each short is its own file, so there is nothing to dissolve between.",
  },

  { type: 'h2', text: 'Audio waveform' },
  {
    type: 'p',
    text: "The waveform draws an audio visualizer onto the clip, in a waves or bars style, in a color you pick, at the top, middle or bottom. Its height is 10 to 50 percent of the frame. It reads the clip's final audio after any speed retiming, so the bars track exactly what a viewer hears. It suits a clip that is carried by the talking rather than the picture.",
  },

  { type: 'h2', text: 'Animated captions' },
  {
    type: 'p',
    text: "The effects step also has an animated captions switch that highlights each word as it is spoken, with controls the caption presets in the gallery do not have. It is covered in full in [how to add animated captions to shorts](/blog/animated-captions-for-shorts), including the size trap on long words and the contrast table for readable colors.",
  },

  { type: 'h2', text: 'The five presets' },
  {
    type: 'p',
    text: "A preset is a named combination of the effects above. Applying one fills the panel with that combination, and you can then change any control before you save. Presets are a convenience only: the server validates and renders whatever effect set it receives, so a preset can never produce anything the panel could not.",
  },
  {
    type: 'table',
    caption: 'What each preset turns on',
    head: ['Preset', 'Turns on'],
    rows: [
      ['Viral Captions', 'Animated captions with two subtle zoom punch-ins.'],
      ['Podcast Pro', 'Captions, a stronger zoom and one line of impact text.'],
      ['Cinematic Story', 'Cinematic grade, a slow five-second zoom, grain and glow, and a gentle fade.'],
      ['Motivation', 'Bold larger captions, a strong zoom and a cinematic grade.'],
      ['Meme Mode', 'A short freeze and two lines of impact text.'],
    ],
  },
  {
    type: 'note',
    title: 'What the presets do not do',
    text: "Cinematic Story uses the per-clip fade, not a cross-clip transition, because the product does not have one. Meme Mode is the reduced form: freeze frames and impact text only. Stickers and sound effects are not built, so no preset promises them.",
  },

  { type: 'h2', text: 'A few rules worth remembering' },
  {
    type: 'ul',
    items: [
      'All times are seconds measured from the start of the clip, and a clip is 30 to 90 seconds long.',
      'Windows of the same effect, punch-ins, speed windows, shake windows and impact texts, cannot overlap each other.',
      'Freeze and speed cannot both be on for one clip.',
      'Effects are burned in and cannot be changed after a clip has rendered.',
    ],
  },
  {
    type: 'p',
    text: "For the full path from a raw file to a finished clip, see [how to turn a podcast into YouTube Shorts](/blog/podcast-to-youtube-shorts).",
  },
];
