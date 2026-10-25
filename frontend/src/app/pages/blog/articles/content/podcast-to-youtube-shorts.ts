import type { ArticleBlock } from '../article-types';

export const blocks: ArticleBlock[] = [
  { type: 'h2', text: 'What you end up with' },
  {
    type: 'p',
    text: "Upload one episode to the [AI Shorts generator](/features/ai-shorts) and you get back up to three vertical clips. Each clip runs between 30 and 90 seconds and has captions burned into the picture. The files are 1080×1920 MP4s (H.264 video, AAC audio, 30 fps), which is the shape YouTube Shorts, Instagram Reels and TikTok expect.",
  },
  {
    type: 'p',
    text: "This guide follows one podcast episode from file to published clip: preparing the file, uploading it, choosing a language, reviewing the clips and fixing the usual problems. Every number below is a limit the tool enforces today.",
  },

  { type: 'h2', text: 'Check the limits first' },
  {
    type: 'table',
    caption: 'AI Shorts limits',
    head: ['Limit', 'Value'],
    rows: [
      ['Length per upload', '1 hour'],
      ['File size per upload', '2 GB'],
      ['File types', 'MP4, MOV, MP3 or WAV'],
      ['Clips per upload', 'Up to 3'],
      ['Clip length', '30 to 90 seconds'],
      ['Output', '1080×1920 MP4, H.264 and AAC, 30 fps'],
      ['Transcript languages', 'Auto-detect, English (US), English (India), Hindi'],
    ],
  },
  {
    type: 'p',
    text: "The upload page reads the length and size limits from the server and checks your file against them before anything is sent. A file that is too long or too large is rejected on the spot, with the length it found.",
  },

  { type: 'h2', text: 'Prepare the file' },
  { type: 'h3', text: 'Estimate the size before you upload' },
  {
    type: 'p',
    text: "File size is bitrate times duration. Your recorder or editor shows the bitrate in its export settings. The arithmetic for a one-hour file is `megabits per second × 3600 ÷ 8 = megabytes`.",
  },
  {
    type: 'table',
    caption: 'Typical sizes for one hour of audio or video',
    head: ['One hour of', 'Approximate size', 'Fits in 2 GB?'],
    rows: [
      ['MP3 at 128 kbit/s', '58 MB', 'Yes'],
      ['WAV, 44.1 kHz, 16-bit stereo', '635 MB', 'Yes'],
      ['720p video at 3 Mbit/s', '1.35 GB', 'Yes'],
      ['1080p video at 8 Mbit/s', '3.6 GB', 'No, shrink it first'],
    ],
  },
  {
    type: 'p',
    text: "Budget time for the upload too. On a test connection that moved about 1.2 MB per second, a 200 MB file took three minutes.",
  },
  { type: 'h3', text: 'Episodes longer than one hour' },
  {
    type: 'p',
    text: "Cut the episode at a natural break, such as the gap between two segments, and upload the parts separately. ffmpeg can cut audio without re-encoding it:",
  },
  {
    type: 'code',
    label: 'Split an MP3 at the one-hour mark',
    text: `# the first hour
ffmpeg -i episode.mp3 -t 3600 -c copy part1.mp3

# everything after the first hour
ffmpeg -ss 3600 -i episode.mp3 -c copy part2.mp3`,
  },
  {
    type: 'p',
    text: "A stream copy cuts at the nearest audio frame, so the cut can land a few milliseconds from the time you asked for. That does not matter here, because the tool chooses its own clip edges.",
  },
  { type: 'h3', text: 'Video files over 2 GB' },
  {
    type: 'p',
    text: "Re-encode the video at a lower resolution, or pull out the audio if you do not need the picture.",
  },
  {
    type: 'code',
    label: 'Shrink a video to 720p',
    text: 'ffmpeg -i episode.mp4 -vf scale=-2:720 -c:v libx264 -crf 28 -preset medium -c:a aac -b:a 128k episode-720p.mp4',
  },
  {
    type: 'code',
    label: 'Keep only the audio, as MP3',
    text: 'ffmpeg -i episode.mp4 -vn -c:a libmp3lame -b:a 128k episode.mp3',
  },
  {
    type: 'note',
    title: 'Audio-only uploads look plain',
    text: "MP3 and WAV files work, but the clips have a black 1080×1920 background with captions on it. If you recorded video, shrink the video instead of dropping the picture.",
  },

  { type: 'h2', text: 'Upload and pick a language' },
  {
    type: 'ol',
    items: [
      "Open Create AI Shorts and drop the file on the page, or choose it from the file picker.",
      "Choose the transcript language. Auto-detect picks between English (US), English (India) and Hindi. If you know the language, choose it yourself. A job is transcribed in one language, so pick the one most of the episode uses.",
      "Start the upload. Files over 16 MB are sent in 16 MB parts, several at a time. If the connection drops, only the parts that failed are sent again. After a page reload, choose the same file and press Resume upload to continue from the last finished part.",
      "When the upload finishes, a short step lets you switch on video effects such as animated captions. Leave everything off to use the defaults, then press Start processing.",
    ],
  },
  {
    type: 'note',
    text: "Effects can be changed before processing starts, or on a failed job before you retry it. A finished job keeps the look it was rendered with. To try another look, upload the file again.",
  },

  { type: 'h2', text: 'What happens while you wait' },
  {
    type: 'p',
    text: "The progress page shows the current stage: upload, transcribing, ranking and rendering. You can close the tab. If email notifications are on in Settings, you get a message when the job finishes or fails.",
  },
  {
    type: 'p',
    text: "Each stage is explained in [how AI video clipping works](/blog/how-ai-video-clipping-works).",
  },

  { type: 'h2', text: 'Review the three clips' },
  {
    type: 'p',
    text: "The Shorts gallery lists the clips in ranked order, with an engagement score on each. The score is the ranking model's own estimate, so use it as a hint. The second clip is sometimes the better one for your audience.",
  },
  {
    type: 'ul',
    items: [
      "Play each clip with the sound on. The first two seconds should make sense to someone who has not heard the rest of the episode.",
      "Read the captions as the clip plays. Names, numbers and product terms are the usual transcript misses.",
      "Check that the captions are easy to read on a phone. They are burned in, so a look you dislike means uploading the file again with different effects.",
      "Download clips one at a time, or select several and download a zip.",
    ],
  },
  {
    type: 'note',
    title: 'The Edit panel does not re-render a finished job',
    text: "The gallery has an Edit panel for a clip's start, end and caption words. It stores your changes on the job, and they are used the next time the job renders, which today means a retry after a failure. A finished job is not rendered again. For a clip that is close but not right, download it and trim it in a video editor, or use one of the other two.",
  },
  {
    type: 'p',
    text: "Caption choices get their own guide: [how to add animated captions to Shorts](/blog/animated-captions-for-shorts).",
  },

  { type: 'h2', text: 'Pick what to publish' },
  {
    type: 'p',
    text: "A clip that works on its own usually opens with a claim, a question or a story beat, and finishes the thought before it ends. Skip clips that lean on something said earlier ('like I said before') and clips where the host reads an ad.",
  },
  {
    type: 'p',
    text: "The tool does not publish for you. Download the MP4 and upload it to each platform yourself. Clips are at most 90 seconds, and each platform sets and revises its own length limits, so check the current rules before you post. Write a title and description on the platform as well. The captions are part of the picture, not text the platform can read.",
  },

  { type: 'h2', text: 'When something goes wrong' },
  {
    type: 'table',
    caption: 'Common problems and fixes',
    head: ['What you see', 'Likely cause', 'What to do'],
    rows: [
      ['The page says the file is too long or too large', 'The episode is over 1 hour or 2 GB', 'Split or shrink the file with the commands above.'],
      ['The job fails with no speech detected', 'The file is silent or music only, or the language is wrong', 'Play the file to check the audio, then upload again with the language chosen by hand.'],
      ['Hindi speech comes out as English words', 'Auto-detect chose another language', 'Upload again with Hindi selected.'],
      ['The upload stops partway', 'The network dropped', 'Reload, choose the same file and press Resume upload.'],
      ['The job failed', 'A server step failed', 'Press Retry on the progress page. A problem with your file, such as no speech or a file that is too long, will fail again until you fix the file.'],
      ['A clip starts or ends mid-sentence', 'Moments are picked from 10 second blocks of text and snapped to the nearest word, not the nearest sentence', 'Use another of the three clips, or download this one and trim it in a video editor.'],
    ],
  },
  {
    type: 'p',
    text: "If your problem is not in this table, [contact us](/contact) with the file type, its length and what the progress page showed.",
  },
];
