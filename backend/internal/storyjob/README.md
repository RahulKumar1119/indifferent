## storyjob: the steps between a confirmed plan and a render

Issue #193. A story video is made in three stages: **copy** the chosen pictures
into our storage, **narrate** each scene, **render** (package `storyvideo`).
This package holds the job record and the first two stages. The worker that
runs all three, the state machine and the API routes come in a later slice.

Both stages are safe to run again. A render on Fargate Spot can be interrupted
and started again by Step Functions; a stage that finds its output already in
storage does not repeat the download or the Polly call.

## The job

`NewFromPlan` makes a queued job from a plan the user has confirmed, and
refuses what cannot be rendered **before anything is spent**:

- a scene with no picture (it names every such scene: "Scenes 2, 4 have no
  picture. Choose one for each and try again.");
- a picture reference that is not well formed;
- a story in a language we cannot narrate (only English and Hindi);
- a voice that cannot read the story's language. An English voice given
  Devanagari makes Polly answer without error and read the text as nothing, so
  it is refused.

The plan's narration and picks are **copied** into the job, so editing the plan
afterwards changes nothing in a video that is being made.

Stored in the projects table beside the user's other records: partition key
`USER#<userId>`, sort key `STORYVIDEO#<jobId>`. The scenes and the caption style
are single JSON attributes, like the Shorts job's clips and caption. A record
that cannot be read is an error, never guessed at.

## Where files go

| Key | What | Kept |
|---|---|---|
| `storyvideo-work/<user>/<job>/scene-NN-media.<ext>` | Copy of the chosen stock file | 7 days |
| `storyvideo-work/<user>/<job>/scene-NN-voice.mp3` | Polly narration | 7 days |
| `storyvideo-work/<user>/<job>/scene-NN-marks.jsonl` | Polly word timings | 7 days |
| `storyvideo/<user>/<job>/video.mp4` | The finished video | until the user deletes it |

**The two prefixes must not overlap**, because the bucket needs a lifecycle rule
that expires `storyvideo-work/` and must never touch the output. A test checks
this. That rule is not created by this slice (see "Not done").

Working files expire because the stock files are only needed to make the video,
and keeping them gains nothing. Both providers' terms bar offering their files
on their own, and Pixabay bars systematic downloading; a bucket of copies that
never expires would drift toward being a library. Pixabay's terms do tell
integrators to download an image to their own server rather than hotlink it,
which is what the copy does. The finished video is a creative work made from
them. See `stockmedia/TERMS.md`.

## Copying the media (`CopyMedia`)

For each scene, in plan order: ask the provider for the asset **as it is now**
(links are signed and expire, so the one stored at search time is not used),
download it under the stock layer's checks, **measure the file itself** with
ffprobe, and upload it.

- The measured size, not the provider's label, decides how the picture is
  framed. A file that ffprobe cannot read as a picture or clip is refused, which
  stops an HTML page or a mislabelled file reaching the renderer.
- A provider URL is never written to the job. A test checks the stored item.
- A scene that already has a copy in storage is skipped. If the record says
  there is a copy but the object is gone, it is made again.
- Scratch files are removed on every path, including a cancel.

The download (`stockmedia.Service.Download`) is stricter than a browser would
be: the asset is validated, its URL must be on the provider's own domains, every
redirect is checked too (a redirect to `169.254.169.254` is refused and tested),
the content type must match, no credentials are sent, and the body is read with
a hard cap (50 MB for a photo, 250 MB for a clip) that does not trust the
server's `Content-Length`. Each download has a 5 minute deadline.

## Failures: what the user is told, and whether to retry

Each failure is a `StepError` with a message fit to show, a stable code, and
whether trying again can help. This is the answer to "a job whose media is
unavailable at render time fails with a clear reason, or uses a defined
fallback, and does not hang".

**There is no fallback picture**: a substitute the user did not choose would
change their video without their say. The job fails and says which scene to
change.

| Code | Cause | Retry? | Message (example) |
|---|---|---|---|
| `MEDIA_UNAVAILABLE` | The provider removed the picture, or its file is gone | no | Scene 2: the picture is no longer available from Pixabay. Choose another and make the video again. |
| `MEDIA_INVALID` | The asset fails validation, or the file is not a readable picture | no | Scene 2: the chosen picture can't be used. Choose another and make the video again. |
| `MEDIA_TEMPORARY` | The provider or its CDN did not answer, rate limited us, or timed out | yes | Scene 2: Pixabay did not answer. Try again in a few minutes. |
| `NARRATION_FAILED` | Polly failed or returned no usable audio; a scene has no text | yes (no for empty text) | Scene 1: the voice could not be made. Try again in a few minutes. |
| `NARRATION_TOO_LONG` | Spoken, the video runs over 90 seconds | no | The narration runs 94 seconds, over the 90 second limit. Shorten the narration in the plan and try again. |
| `INTERNAL` | Our storage or scratch disk failed | yes | Scene 2: something went wrong on our side. Try again. |

Messages never contain a URL. A full scratch disk is reported as `INTERNAL`,
never as a problem with the picture.

Nothing waits on a provider forever: a stalled download ends at the caller's
deadline and is reported as retryable.

## Narrating (`NarrateScenes`)

Polly audio and its word timings are requested for the same text, voice and
engine (see `narrator.SynthesizeMarks`). The audio's **length is measured from
the file**; it is what the renderer will play.

If Polly's timings cannot be had or are unusable, the scene is narrated without
them and the renderer spreads the captions evenly across the audio
(`storyvideo.EvenWords`). A video with slightly looser captions is better than
no video, and the fallback is logged. Timings that are only partly wrong are
repaired, not discarded.

Once every scene is narrated the total is checked against the 90 second limit.
A story that runs over fails here, **before any rendering time is spent**.

## Measured against the real Polly (`go test -tags live ./internal/storyjob/`)

Two sentences each, standard engine, `ap-south-1`, storage in memory:

| Language | Words | Audio | What the planner estimates |
|---|---|---|---|
| English (Joanna) | 17 | 4.52 s | 6.8 s |
| English (Joanna) | 17 | 4.39 s | 6.8 s |
| Hindi (Aditi) | 16 | 6.22 s | 6.4 s |
| Hindi (Aditi) | 13 | 5.20 s | 5.2 s |

English is spoken much faster than the planner assumes; Hindi is close to it.
That is why scene length is never taken from the plan.

## Not done in this slice

- The worker, state machine, status and notification wiring, API routes, and
  the deploy scripts that would create the lifecycle rule for
  `storyvideo-work/`.
- Rendering from these inputs (package `storyvideo` has the command builder).
- The UI.
