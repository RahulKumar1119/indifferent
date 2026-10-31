# storyvideo: how a Story2Shorts video is made

Issue #193 (phase 4 of #162). This package is the pure core: it decides where
each scene sits in time, when each word is shown, and which FFmpeg command
renders the result. It does no network, disk or process work, so all of it is
tested. The Fargate worker that downloads media, calls this package and runs
FFmpeg is a later slice.

## The pipeline

1. **Narration.** Amazon Polly speaks each scene's narration (MP3), and a second
   call with the same text, voice and engine returns the **word speech marks**
   (`narrator.SynthesizeMarks`).
2. **Timeline** (`BuildTimeline`). Each scene lasts as long as its audio plus a
   0.35 s pause. Pictures cross-fade for 0.4 s as the next narration begins.
3. **Captions** (`WordsFromMarks`, `PlaceWords`, then `render.BuildCaptions`).
   The speech marks become word timings on the video's timeline. The AI Shorts
   caption presets (minimal, bold, karaoke, clean, with size, colour and
   position) are reused unchanged.
4. **Render** (`BuildArgs`). One FFmpeg filter graph frames each picture to
   9:16, zooms photos slowly, chains the scenes with cross-fades, burns in the
   captions and joins the narration.

Output matches AI Shorts: 1080x1920, 30 fps, H.264 (yuv420p), AAC 48 kHz,
`+faststart`.

## Decisions, and the measurements behind them

Measured on 2026-10-31. FFmpeg numbers are from a developer machine pinned to
two cores (`taskset -c 0,1`), the size of the production render task (2 vCPU,
4 GB). They are not Fargate timings.

### Where caption timings come from: Polly speech marks

The issue asks to choose between the TTS provider's speech marks and running
the generated audio back through Transcribe.

**Chosen: speech marks.** The marks come from the same service that makes the
voice, so they describe the audio exactly, with no second recognition step that
could mishear. They need one more Polly call per scene (a JSON response, no
audio), not a Transcribe job to start and poll. They work for Hindi: recorded
from the real service for English (Joanna) and Hindi (Aditi, `hi-IN`), both
return one `word` mark per word with a start time in milliseconds
(`testdata/`).

Two properties of the marks shape the code:

- Marks hold **bare words** ("cliff"), without punctuation, and their offsets
  are **bytes** into the text sent, not characters (a Hindi letter is three
  bytes). Each caption word is cut out of the original text, from one word's
  offset to the next, so "cliff." and the Hindi danda in "था।" are kept, and
  an opening quote goes with the word it belongs to.
- A mark gives a word's start but not its end. A word ends where the next
  begins, and the last word ends with the audio. A caption therefore stays up
  through a pause inside a sentence.

If the marks are missing or unreadable, `EvenWords` spreads the text across the
audio by word length. The video still renders and the captions stay roughly in
step. The worker should log when it falls back.

### Scene length comes from the audio, not the plan

The plan estimates 2.5 words a second. Real Polly narration (English, Joanna)
spoke eight scenes of 16 to 18 words in 4.2 to 4.7 seconds each, where the plan
would have said 6.4 to 7.2. Hindi (Aditi) came close to the estimate in the two
sentences measured on 2026-10-31: 16 words took 6.2 seconds and 13 took 5.2
(about 2.5 words a second). So the speed depends on the language, and a plan is
not trusted for timing: a video can come out longer or shorter than planned.

The limit is therefore checked after narration (`TooLongError`, 90 seconds, the
longest AI Shorts clip): a video that runs over is refused with a message
saying to shorten the narration, and is never cut short silently.

### How photos are held on screen

A still is shown for its scene's span plus the cross-fade, with a slow zoom of
8% over that time. A still held for ten seconds with no movement looks frozen,
and a few percent of movement reads as a deliberate shot. Clips already move,
so they are not zoomed, and their own sound is never used.

Zooming at the output size shimmers: the picture moves in whole-pixel steps.
Zooming a copy twice the size and shrinking it back moves it in half-pixel
steps. Measured on a 5 second photo, the variation in frame-to-frame change
(coefficient of variation, lower is smoother) was 0.29 at output size, 0.13
from a 2x copy, and 0.14 for an 8x reference.

It costs render time. A 38 second, eight-scene video (photos and clips,
portrait and landscape, real narration):

| Photo motion | Render time | Memory |
|---|---|---|
| none | 40.0 s | 0.79 GB |
| zoom at output size | 43.0 s | 0.80 GB |
| zoom from a 2x copy (**used**) | 61.8 s | not measured separately |
| `zoompan` filter | 53.5 s | 0.85 GB |

A 90 second video should take roughly 2.4 times as long, about 150 seconds, far
inside the 1800 second task timeout. Spot cost follows run time, so the extra
20 seconds is the price of smooth motion. If renders prove too slow or too
costly, `ZoomPercent = 0` turns the zoom off and the supersampled path goes
with it.

### Portrait and landscape

A picture at most 3:4 (width over height 0.75 or less) is scaled to fill the
frame and cropped. Anything wider is shown whole over a blurred, enlarged copy
of itself, the AI Shorts blur-fill. A 9:16 crop of a 16:9 clip would discard
two thirds of it; a 3:4 photo loses a quarter of its width to a 9:16 crop,
which usually keeps the subject.

### Clips shorter or longer than their scene

Shorter clips loop; longer ones are cut. Both are tested with real renders.

### Failure rules

| Case | Result |
|---|---|
| Narration runs over 90 seconds | `TooLongError` with a user-facing message |
| A scene has no narration audio | Error naming the scene |
| Speech marks missing or unreadable | `EvenWords` fallback |
| Speech marks with bad offsets or times | Repaired word by word |
| A caption file path that is unsafe in an FFmpeg filter | Refused; the worker writes into its own work directory |

Media that is unavailable at render time is the worker's concern (slice 3),
which fails the job with a reason.

## What is tested

`go test ./internal/storyvideo/` runs real FFmpeg renders and reads the result:
output format, scene order by pixel colour, cross-fades between scenes, looped
and cut clips, narration present with a pause between scenes, captions
present only at their words and in the chosen place, Hindi glyphs drawn, and
landscape shown whole where portrait is cropped. Those renders take about
100 seconds; `-short` skips them. They are skipped if FFmpeg is not installed.
