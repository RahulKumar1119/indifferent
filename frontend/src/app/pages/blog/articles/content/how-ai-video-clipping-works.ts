import type { ArticleBlock } from '../article-types';

export const blocks: ArticleBlock[] = [
  { type: 'h2', text: 'The short version' },
  {
    type: 'p',
    text: "The [AI Shorts generator](/features/ai-shorts) turns one long recording into up to three vertical clips. It does this in six stages. A transcript tells the system what was said and when. A language model reads that transcript and picks the moments. Everything after that is mechanical: framing, captions and rendering.",
  },
  {
    type: 'figure',
    src: '/blog/shorts-pipeline.svg',
    alt: 'Six boxes in order: upload, transcribe, rank, frame, caption and render.',
    caption: 'The six stages between upload and download.',
    width: 1200,
    height: 630,
  },
  {
    type: 'p',
    text: "This article describes what each stage does and, just as important, what it cannot do. Knowing that tells you how to prepare a file and why a result sometimes disappoints.",
  },

  { type: 'h2', text: 'Stage 1: upload' },
  {
    type: 'p',
    text: "The page accepts MP4, MOV, MP3 and WAV files up to 1 hour and 2 GB and checks both limits before sending anything. Files over 16 MB are cut into 16 MB parts that upload several at a time. Each part is retried on its own, and the browser remembers which parts finished so a reload can pick up where it stopped. The file lands in private storage and is tied to your account. Only you can start a job on it.",
  },

  { type: 'h2', text: 'Stage 2: transcription' },
  {
    type: 'p',
    text: "Amazon Transcribe listens to the file and returns every word with a start and end time. You choose the language: English (US), English (India), Hindi, or auto-detect, which picks among those three. The whole file is transcribed in one language.",
  },
  {
    type: 'p',
    text: "If the transcript has no words, the job stops with a message that no speech was detected. Retrying cannot fix that, because the cause is the file or the language setting.",
  },

  { type: 'h2', text: 'Stage 3: ranking' },
  {
    type: 'p',
    text: "This is the step that decides what you get. The transcript is grouped into blocks of 10 seconds, so an hour of speech becomes about 360 lines of text. Those lines go to a language model hosted on Amazon Bedrock, with a fixed set of instructions:",
  },
  {
    type: 'ul',
    items: [
      "Find the most engaging, self-contained moments, at most three.",
      "Each moment must last between 30 and 90 seconds.",
      "Prefer a strong hook, an emotional peak or a complete thought.",
      "Return a start, an end, a score and a short hook text for each moment, as JSON.",
    ],
  },
  {
    type: 'p',
    text: "The model's answer is never trusted as it comes. Every segment is checked for valid times and for a length inside the 30 to 90 second window. An answer that is not valid JSON counts as no segments. Because the model works from 10 second blocks, its times land between words, so each boundary is then moved to the nearest word edge. A clip never starts or ends in the middle of a word.",
  },
  {
    type: 'note',
    title: 'The model reads text only',
    text: "It never sees the picture and never hears tone of voice. A great visual moment with little speech will not rank, and a flat delivery of an interesting sentence can rank high. The score you see in the gallery is the model's own number, not a measure of how people reacted to similar clips.",
  },

  { type: 'h2', text: 'Stage 4: framing' },
  {
    type: 'p',
    text: "A 16:9 recording has to become 9:16. The renderer takes up to eight frames spread across the clip and sends them to Amazon Rekognition to find faces. From each frame it keeps the largest face and notes where its center sits. The 9:16 window is then centered on the median of those positions. That gives one window position for the whole clip.",
  },
  {
    type: 'ul',
    items: [
      "A face was found: the window is placed on it.",
      "No face was found, for example in a slide deck, a screen recording or a cartoon: the whole frame is scaled to fit over a blurred copy of itself, so nothing is cut off.",
      "The source is already vertical: the window covers the full width and nothing is cropped.",
      "The source is audio only: the clip uses a black 1080×1920 canvas.",
    ],
  },
  {
    type: 'p',
    text: "The geometry and the commands are covered in [how to convert landscape footage into vertical video](/blog/landscape-to-vertical-video).",
  },

  { type: 'h2', text: 'Stage 5: captions' },
  {
    type: 'p',
    text: "The word timings from Stage 2 become caption cues of up to four words. The cues are burned into the picture with libass, a subtitle renderer, so they are part of the video file. Styles and the word-by-word highlight are described in [how to add animated captions to Shorts](/blog/animated-captions-for-shorts).",
  },

  { type: 'h2', text: 'Stage 6: rendering' },
  {
    type: 'p',
    text: "Each clip is rendered by FFmpeg in its own container on AWS Fargate. The video is cropped or fitted, scaled to 1080×1920 at 30 frames per second, and encoded as H.264 with AAC audio at 128 kbit/s and 48 kHz. The finished MP4 goes into your gallery.",
  },

  { type: 'h2', text: 'Where it goes wrong' },
  {
    type: 'ul',
    items: [
      "Moments with little speech are invisible to the ranking. Interviews and lectures work better than music videos or silent demos.",
      "The window is fixed for the whole clip. A speaker who walks across a wide frame can leave the window, and nothing follows them.",
      "With two people in frame, the larger face wins. The other person can be cropped out.",
      "A job is transcribed in one language. Mixed-language conversations will contain errors.",
      "A second run on the same file can choose different moments, because the ranking comes from a language model.",
      "At most three clips come back, even if the recording holds ten good moments.",
    ],
  },

  { type: 'h2', text: 'How to get better results' },
  {
    type: 'ul',
    items: [
      "Trim dead air and ads before you upload. Every second of filler is text the model has to read past.",
      "Choose the transcript language yourself when you know it.",
      "Use clean audio. A noisy recording produces a worse transcript, and the ranking is only as good as the transcript.",
      "For wide shots with several people, expect the largest face to win and check the preview.",
      "Split a long recording by topic and upload each part. Each upload returns its own three clips.",
      "If a clip is close but not right, download it and trim the start or end in a video editor, or use one of the other two clips.",
    ],
  },
  {
    type: 'p',
    text: "Your files are stored and processed by the AWS services named above. The [privacy policy](/privacy) lists each service, what it receives and how deletion works.",
  },
];
