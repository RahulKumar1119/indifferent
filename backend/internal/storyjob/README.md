## storyjob: the steps between a confirmed plan and a render

Issue #193. A story video is made in three stages: **copy** the chosen pictures
into our storage, **narrate** each scene, **render** (package `storyvideo`).
This package holds the job record, the stages, and the runner that drives them
(`Runner.Run`). `cmd/storyrender` is the Fargate task that calls it. The state
machine, status and notification wiring, API routes and deploy scripts come in
the next slices.

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

## The worker (`cmd/storyrender`)

`Runner.Run(ctx, userID, jobID)` loads the job, moves it through `preparing`,
`narrating` and `rendering`, saves what each stage made (so a restart resumes),
uploads the video, and writes the end state on the job record. A thin `main`
wires the real AWS clients and the stock-media service to it. The task reads
`USER_ID`, `JOB_ID`, `S3_BUCKET`, `DYNAMODB_TABLE`, the stock keys
(`PEXELS_API_KEY` / `PIXABAY_API_KEY`) and optionally `WORK_DIR`.

### The exit code is the contract with the state machine

| Outcome | Exit | Meaning | What the state machine should do |
|---|---|---|---|
| completed | 0 | Video stored, record says `completed` | Notify, done |
| failed | 0 | The user must fix something; the reason is on the record | Notify the failure. **Do not retry** |
| cancelled | 0 | The user deleted or cancelled the job | Stop, no email |
| retry | 1 | Temporary failure, or the process was told to stop | Run the task again |
| bad start | 2 | Missing or malformed `USER_ID` / `JOB_ID` / config | A retry cannot fix it |

Three consequences the state machine must respect, because getting them wrong
turns a failure into a silent success:

- **Exit 0 does not mean success.** A failed job exits 0 so Step Functions does
  not retry something hopeless. The state machine has to read the job record's
  `status` after the task to tell completed from failed from cancelled.
- **SIGTERM exits 1.** ECS sends it on a Spot interruption. Exiting 0 there
  would tell Step Functions the job succeeded when it was cut off. A test
  pins this.
- **A retry's reason is saved on the record** (`error`, `errorCode`,
  `failedStage`), with the status left at the stage that failed. If Step
  Functions runs out of retries the user still sees why, in words, and the
  state machine can mark the job failed with that message.

### Safety properties

- **A deleted job is never brought back and an ended job is never overwritten.**
  Every write is conditional on the record existing and not being `completed` or
  `failed`. A slow worker cannot undo a cancel.
- **A cancel stops the work.** The worker re-reads the job's status every 10
  seconds and stops FFmpeg when the job is gone or has ended.
- **A failed progress write is logged, not fatal.** The one write error that
  does stop the run is learning the job was deleted.
- **The finished file is checked before the job is called completed**: a picture
  stream, a sound stream, 1080x1920, and a length within half a second of the
  plan. FFmpeg exiting 0 is not proof of a good video.
- **Ids are validated as UUIDs** before they go into any storage key.
- **No path is escaped inside an FFmpeg filter.** FFmpeg runs in the work
  directory and the captions file is named relatively. A test uses a directory
  called `it's a: odd dir [x]`.
- **Captions need fonts.** The worker checks at start-up and fails loudly; with
  no font libass draws nothing and the video would have no captions and no error.

### One line to size and cost a render

On success the worker logs a line like

    storyjob_timing job=... scenes=3 prepare=0.2s narrate=0.2s render=18.0s upload=0.0s video_seconds=10.5 output_bytes=282730

This is how the real time and cost on Fargate Spot are measured. **Nothing has
been run on Fargate yet**: the figures in `storyvideo/README.md` are from a
developer machine.

## Verified in the production image

The `Dockerfile` in `cmd/storyrender` uses Alpine's FFmpeg, which is a different
major version from the one on a developer machine (6.1.2 against 4.4.2). The
whole `storyvideo` and `storyjob` suites, including every real-render and
end-to-end test, were compiled statically and run **inside the built image**:
97 tests in `storyjob` and 41 in `storyvideo` (top level, not counting subtests), none failed and none skipped. To repeat it:

    docker build -f cmd/storyrender/Dockerfile -t story-render-local backend/
    CGO_ENABLED=0 go test -c -o /tmp/t/storyjob.test ./internal/storyjob
    docker run --rm --entrypoint sh -v /tmp/t:/t:ro -v $PWD/internal/storyjob:/src:ro -w /src \
        story-render-local -c '/t/storyjob.test -test.v'

## Not done

- The state machine, the status and notification wiring for this job type, the
  API routes, and the deploy scripts (including the lifecycle rule that expires
  `storyvideo-work/` and the ECS task definition, IAM permissions and image push).
- Running anything on Fargate, or against real DynamoDB or S3.
- The UI.
