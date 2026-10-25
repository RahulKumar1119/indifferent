import type { ArticleBlock } from '../article-types';

export const blocks: ArticleBlock[] = [
  { type: 'h2', text: 'A container is not a codec' },
  {
    type: 'p',
    text: "People say 'MP4 or WebM' as if they were two kinds of video. They are two kinds of box. A video file has a container (the box, shown by the file extension) and inside it one or more streams, each encoded with a codec. The container decides how the streams are packed together. The codecs decide how big the file is, how good it looks and which devices can play it.",
  },
  {
    type: 'figure',
    src: '/blog/containers-and-codecs.svg',
    alt: 'Two boxes. The MP4 box holds H.264 video and AAC audio. The WebM box holds VP8, VP9 or AV1 video and Opus or Vorbis audio.',
    caption: 'The codecs each container usually holds.',
    width: 1200,
    height: 630,
  },
  {
    type: 'table',
    caption: 'MP4 and WebM compared',
    head: ['', 'MP4', 'WebM'],
    rows: [
      ['Video codecs', 'H.264 (the common one), H.265, AV1', 'VP8, VP9, AV1'],
      ['Audio codecs', 'AAC (the common one), MP3', 'Opus, Vorbis'],
      ['Origin', 'MPEG-4 standard family', 'Open format from the WebM Project'],
      ['Best known for', 'Uploads, phones, editors, hardware playback', 'Video embedded in web pages'],
    ],
  },
  {
    type: 'p',
    text: "You can check what a file holds with ffprobe, which ships with ffmpeg. The container is in `format_name` and the codecs are in `codec_name`:",
  },
  {
    type: 'code',
    label: 'See the container and codecs of a file',
    text: 'ffprobe -v error -show_entries stream=codec_type,codec_name,width,height -show_entries format=format_name,duration -of default=nw=1 input.mp4',
  },

  { type: 'h2', text: 'Where each one plays' },
  {
    type: 'p',
    text: "MP4 with H.264 video and AAC audio plays almost everywhere: phones, TVs, editors, browsers and every major upload form. That is why YouTube's recommended upload settings ask for an MP4 container with H.264 video and AAC-LC audio (Opus is also listed), and why the [AI Shorts generator](/features/ai-shorts), the [quiz video generator](/features/txt-to-video-quiz) and the [watermark tool](/features/watermark) all export an MP4 with H.264 video.",
  },
  {
    type: 'p',
    text: "WebM plays in current versions of Chrome, Edge, Firefox and Safari. Safari added it later than the others (macOS got it in Safari 14.1), so older Apple devices may not play a WebM file. Many editors, phone galleries and social upload forms expect MP4 and handle WebM badly or not at all. YouTube does accept WebM, as its [supported formats page](https://support.google.com/youtube/troubleshooter/2888402) shows.",
  },
  {
    type: 'p',
    text: "Platforms change their rules, so read the current ones before you export for a specific site: [YouTube's recommended upload settings](https://support.google.com/youtube/answer/1722171) are a good model for the others.",
  },

  { type: 'h2', text: 'File size is mostly a codec and bitrate question' },
  {
    type: 'p',
    text: "File size is bitrate times duration, and bitrate is what the encoder is told to spend. A newer codec such as VP9 or AV1 can reach the same visual quality with a lower bitrate than H.264, but it usually takes much longer to encode and may not decode in hardware on older devices.",
  },
  {
    type: 'p',
    text: "Quality settings do not carry over between codecs. CRF 23 in x264 and CRF 32 in VP9 are each sensible defaults for their own codec, and they do not mean the same quality. Size also depends on the footage. We encoded a 6 second, 1080p test pattern with the two commands further down. The VP9 WebM came out at 4.99 MB and the H.264 MP4 at 4.67 MB, so the newer codec was larger. A synthetic test pattern is unusually hard to compress, so treat that as a warning against generic claims, not as a benchmark. Test on your own footage.",
  },

  { type: 'h2', text: 'Converting between them with ffmpeg' },
  {
    type: 'code',
    label: 'MP4 to WebM (VP9 video, Opus audio)',
    text: 'ffmpeg -i input.mp4 -c:v libvpx-vp9 -crf 32 -b:v 0 -c:a libopus -b:a 96k output.webm',
  },
  {
    type: 'code',
    label: 'WebM to MP4 (H.264 video, AAC audio)',
    text: 'ffmpeg -i input.webm -c:v libx264 -crf 23 -preset medium -pix_fmt yuv420p -c:a aac -b:a 128k -movflags +faststart output.mp4',
  },
  {
    type: 'ul',
    items: [
      "`-crf` sets quality: lower numbers mean higher quality and bigger files.",
      "`-pix_fmt yuv420p` keeps the pixel format that the widest range of players can decode.",
      "`-movflags +faststart` moves the index to the start of the MP4, so it can begin playing before it has fully downloaded. YouTube's recommended settings ask for it.",
      "Converting always re-encodes the video, which costs a little quality each time. Convert once, from the best original you have.",
    ],
  },

  { type: 'h2', text: 'What the Indifferent tools accept and produce' },
  {
    type: 'table',
    caption: 'Formats by tool',
    head: ['Tool', 'Accepts', 'Produces'],
    rows: [
      ['AI Shorts generator', 'MP4, MOV, MP3, WAV', '1080×1920 MP4 (H.264, AAC)'],
      ['TXT to quiz video', 'TXT file', '1080p MP4'],
      ['Watermark tool', 'JPG, PNG, WebP, MP4, WebM, MOV', 'PNG or JPEG for images, MP4 for video'],
    ],
  },
  {
    type: 'p',
    text: "A WebM file therefore works in the watermark tool but not in the AI Shorts generator. Convert it to MP4 with the command above before you upload it for clipping. The [podcast guide](/blog/podcast-to-youtube-shorts) has more ffmpeg commands for files that are over the upload limits, and the [watermark guide](/blog/add-watermark-to-image-in-browser) covers the browser export in detail.",
  },

  { type: 'h2', text: 'Which should you choose' },
  {
    type: 'ul',
    items: [
      "Uploading to a social platform, sending to a client or editing in desktop software: MP4 with H.264 and AAC.",
      "Embedding a video in your own web page and wanting an open format: WebM is a fair choice, but offer an MP4 as well for older Apple devices.",
      "Shrinking a large archive for your own use: try VP9 or AV1 and measure the size on your footage before you commit.",
      "Not sure: MP4 with H.264 and AAC. It is the format with the fewest surprises.",
    ],
  },
  {
    type: 'p',
    text: "For a background on containers, the [MDN guide to media container formats](https://developer.mozilla.org/en-US/docs/Web/Media/Guides/Formats/Containers) goes further than this article does.",
  },
];
