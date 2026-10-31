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
| `TIMED_OUT` | The worker used its whole time allowance (15 minutes) | no | Making the video took too long and was stopped. Try again, or shorten the story. |

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
| retry | 1 | Temporary failure, or the process was told to stop (SIGTERM) | Run the task again |
| bad start | 2 | Missing or malformed `USER_ID` / `JOB_ID` / config | A retry cannot fix it |

Three consequences the state machine must respect, because getting them wrong
turns a failure into a silent success:

- **Exit 0 does not mean success.** A failed job exits 0 so Step Functions does
  not retry something hopeless. The state machine has to read the job record's
  `status` after the task to tell completed from failed from cancelled.
- **SIGTERM exits 1.** ECS sends it on a Spot interruption. Exiting 0 there
  would tell Step Functions the job succeeded when it was cut off. A test
  pins this.
- **Running out of time is a failure, not a retry.** The worker's own deadline
  (`taskTimeout`, 15 minutes) ends a stuck render as `TIMED_OUT`, exit 0. Asking
  for a retry would run the same stuck render on every Spot attempt and then
  on-demand: about an hour of compute, and a generic message at the end. A
  SIGTERM is different (the machine went away, not the job), so it stays a retry.
  Tests cover both, and that a cancel arriving before the deadline stays a cancel.
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
- **Ids are validated before they go into any storage key.** A job id must be a
  UUID (this system makes them). A user id must be 1 to 64 of letters, digits,
  `-` and `_`, because it is **not always a UUID**: a user who signs in with
  Google has Google's own numeric id (about 21 digits) and only a password signup
  gets a UUID. The first version of the worker accepted only UUIDs and would have
  refused every Google user's job with exit 2; it was found while building the
  API, which is the first code to start a job with a real user's id.
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
105 tests in `storyjob` and 41 in `storyvideo` (top level, not counting subtests), none failed and none skipped. To repeat it:

    docker build -f cmd/storyrender/Dockerfile -t story-render-local backend/
    CGO_ENABLED=0 go test -c -o /tmp/t/storyjob.test ./internal/storyjob
    docker run --rm --entrypoint sh -v /tmp/t:/t:ro -v $PWD/internal/storyjob:/src:ro -w /src \
        story-render-local -c '/t/storyjob.test -test.v'

## The pipeline (`pipeline/statemachine.json`)

Step Functions runs the worker and tells the user. The execution input is just
`{"userId": "...", "jobId": "..."}`; everything else is read from the job record.
It uses the direct DynamoDB integration (`getItem`, `updateItem`), so no Lambda
tracks the job.

    RunOnSpot ── ok ──▶ ReadJob ─▶ IsJobThere? ─ yes ─▶ HowDidItEnd? ─┬ completed ─▶ NotifyCompleted
      │ exit 1: retried twice,          │ no                            ├ failed ────▶ NotifyFailed
      │ 20 s then 40 s apart            ▼                               └ anything else ▶ StillRunning ─▶ MarkFailed
      ▼ still failing, or timed out    Gone
    RunOnDemand ── fails ──▶ MarkFailed ── ok ──▶ NotifyFailed
                                 │ refused: the job had already ended, or is gone
                                 ▼
                           ReadJobAgain ─▶ IsJobStillThere? ─▶ EndedHow? ─▶ NotifyCompleted / NotifyFailed / Gone

- **The outcome is read from the record, never inferred from the exit code.**
  Exit 0 means completed, failed and cancelled alike (see above).
- **A missing record goes to `Gone`, through a `Choice` with `IsPresent`.**
  `getItem` on a missing key answers `{}`. Reading `$.Item.status.S` from that
  raises `States.Runtime`, which no `Catch` can handle (not even `States.ALL`),
  so a job deleted while it ran would have failed the whole execution. The real
  engine showed this (see below); a structural test now requires every reader of
  the record to be entered only from a guard.
- **A timeout is not retried on Spot.** A rule with `MaxAttempts: 0` comes before
  the retry rule, so a stuck task goes straight to on-demand. It does not rely on
  how `States.TaskFailed` treats timeouts.
- **The two time limits are tied.** The state machine allows 1200 s; the worker
  stops itself at 15 minutes (`cmd/storyrender`). A test reads the worker's value
  from its source and requires the state machine's to be at least two minutes
  longer, so the worker's deadline (which saves a reason) always fires first.
  Both numbers were chosen before any render ran on Fargate; the
  `storyjob_timing` line is how to set them properly.
- **`MarkFailed` keeps the worker's reason** (`if_not_exists`) and is conditional
  on the record existing and not having ended, so it cannot revive a deleted job
  or overwrite a completed one or a cancel. The reasons it can add are generic.
- **Email is best effort.** Both notify states catch every error and end. The
  notifier has a `story` kind: the link is the plan's page (`/story/<planId>`),
  not the job's id; a cancelled job and a job with no plan send nothing; the
  failure email has fixed wording and never repeats the stored error.
- `pipeline.Render` fills the placeholders and refuses an empty value or one that
  would break the JSON. `cmd/storypipeline` prints the result, and the deploy
  script uses it, so the tests and the deployed definition share one renderer.

## The API (`internal/api/storyvideo.go`)

| Route | What it does |
|---|---|
| `GET /story-videos/limits` | Public. `{enabled, maxActive, perDay, maxSeconds, voices: {en: [...], hi: [...]}}` |
| `POST /story-videos` | Make a video. Body `{planId, revision, voice?, caption?}`. 202 with the job's view |
| `GET /story-videos[?planId=]` | The user's videos, newest first: `{videos: [...], hasMore}`. At most 20, read from the newest 100 |
| `GET /story-videos/{id}` | One job's view (a consistent read, so progress never goes backwards) |
| `POST /story-videos/{id}/cancel` | End a video that is being made. 200 with the view |
| `GET /story-videos/{id}/download` | `{url, filename, expiresInSeconds}` for a finished video. 15 minute link |
| `DELETE /story-videos/{id}` | Remove a finished or cancelled video and its files. 204 |

Every route but the first needs sign-in. Another user's id, an id that does not
exist and an id that is not a UUID all answer 404 with the same body.

**The view** (what the page is built on). It is not the record: no storage key, no
scene text, no internal error.

    {
      "jobId", "planId", "planRevision",
      "status": "queued" | "preparing" | "narrating" | "rendering" | "completed" | "failed",
      "progress": 0-100, "language", "voice", "caption": {preset, fontSize, color, highlight, position},
      "totalSeconds"?,                                  // once known
      "error"?, "errorCode"?, "failedStage"?,           // only when status is "failed"
      "createdAt", "updatedAt", "completedAt"?
    }

`errorCode` is `CANCELLED` when the user cancelled, `START_FAILED` when the
pipeline could not be started, `TIMED_OUT`, or one of the worker's codes (see
"Failures"). `error` is a sentence for the user. While a job is still going it
shows no error even if the worker saved the reason of an attempt it will retry.

**Errors** are `{code, message}` (and `details` where noted):

| HTTP | `code` | When |
|---|---|---|
| 400 | `MISSING_PICTURES` | A scene has no picture. The message names the scenes |
| 400 | `UNSUPPORTED_LANGUAGE` | The story is not in English or Hindi |
| 400 | `VALIDATION_ERROR` | Voice cannot read this language, or the caption style is invalid |
| 404 | `NOT_FOUND` | No such plan or video, or not yours |
| 409 | `NOT_READY` | The plan is not finished, or the video is not finished (download) |
| 409 | `STALE_REVISION` | The plan changed since the page read it. Reload |
| 409 | `ALREADY_RUNNING` | A video is already being made from this plan. `details.jobId` is that video |
| 409 | `TOO_MANY_ACTIVE` | Two videos are already being made |
| 409 | `STILL_RUNNING` | Delete of a video that has not ended. Cancel it first |
| 429 | `RATE_LIMITED` | Ten started in 24 hours. `details.retryAfter` is the seconds until the oldest leaves the window |
| 503 | `UNAVAILABLE` | `STORYVIDEO_STATE_MACHINE_ARN` is not set. Only create answers this |

### How create works, and why in this order

1. The plan is read from the user's own record. The client sends only its id and
   the revision it was looking at; a different revision is refused, so the video
   is the one the user approved.
2. Everything the user can fix is checked next (a picture for every scene, the
   language, the voice, the caption style), because it costs nothing and spends
   no allowance.
3. Then the limits, then the job is written (`attribute_not_exists(PK)`), and only
   then is the execution started, named after the job id.
4. If the start fails the job is marked failed (`START_FAILED`), not left queued.
   A start that timed out may still have started; marking the job ended also
   stops that execution, because the worker finds its record ended.

### Cancel

The record is changed first, with `storyjob.NotEndedCondition`, the one condition
the worker's store and the state machine's `MarkFailed` also use. That makes
three things true at once: a video that finished is never overwritten by a late
cancel, a slow worker never undoes a cancel, and a deleted job is never
recreated. The worker notices within about 10 seconds and stops FFmpeg, and the
notifier sends nothing for `CANCELLED`. Stopping the execution comes second and
is best effort. Cancelling a job that has ended is not an error: it answers with
its current state. So does a cancel that loses the race to the worker.

### Decisions

- **Paths** follow `/story-plans`: a plural noun, the id below it, and an action
  suffix only for the two actions that are not create/read/delete.
- **Limits: two in the making per user, ten started per 24 hours, one per plan.**
  Each video is a 2 vCPU Fargate task and the Spot quota in this account was 30
  vCPU. A cancelled or failed video counts towards the ten, since it used Polly and
  a task. Other limits (stock search, plans) are per hour; this one is per day
  because a video is the expensive step.
- **A second create for a plan that already has a video in the making is a 409
  that names the video, not a 200 that returns it.** A 200 would hide that the
  voice or caption just sent was ignored. The page can offer to cancel the other.
- **The 24 hour window is a key range on the job id**, which is a UUIDv7 and
  sorts by time, so no index or scan is needed (the plans do the same).
- **The active limit is not race-free.** It is a query followed by a put, and
  DynamoDB cannot condition a put on a count of other items. Two creates sent at
  the same instant by one user can both pass. The overshoot is bounded by how many
  a user can send inside one request, and the day limit is checked on the same
  read. Making it strict needs a counter item updated in a transaction; for a
  cost limit that was not worth it.
- **A job untouched for two hours stops counting as active.** The longest honest
  run is about 80 minutes (three Spot attempts and one on-demand, each cut off at
  20), so a job older than two hours lost its execution without being marked
  failed. It would otherwise hold its owner at the limit for ever.
- **Delete of a video that has not ended is refused (409).** Cancel changes the
  record at once and the worker stops within seconds, but a delete during that
  time could leave a file that the worker was in the middle of uploading, where no
  record points and no lifecycle rule expires. Cancel-then-delete is what a page
  does anyway. The window that remains is a script that cancels and deletes in the
  same few seconds.
- **Delete removes the files first and the record last**, so a failed cleanup
  leaves the record and can be retried. The bucket is versioned, so a plain delete
  only hides a file; every version and marker under the job's two prefixes
  (`JobPrefixes`) is removed. A test requires every key the worker writes to be
  under one of those prefixes.
- **A download link is made only for the job's own output key**, recomputed from
  the ids, never for whatever the record names. A completed record whose file is
  gone answers 404 instead of a link that returns an error page.

### Known gaps

- **Nothing reconciles a job whose execution died without marking it failed.**
  The state machine catches every error and ends by reading the record, so this
  needs something outside it (an operator stopping an execution, for one). The job
  stays "rendering"; it stops counting against the limit after two hours; the user
  can cancel it and delete it. A sweeper that asks Step Functions about old
  non-terminal jobs would close it.
- **Deleting a video gives its place in the daily allowance back**, because the
  allowance counts the records that exist. Create, cancel, delete, repeat is
  limited by the active limit, not the daily one.
- **The IAM grant that `deploy/setup.sh` gave the API role does not cover
  stopping an execution.** `states:StopExecution` is authorised on the
  *execution* (`...:execution:<machine>:<name>`), and `setup.sh` grants it on
  `...:stateMachine:...`, which never matches
  ([Service Authorization Reference](https://docs.aws.amazon.com/service-authorization/latest/reference/list_stepfunctions.html)).
  The deploy script adds the grant for this state machine. **The shorts cancel,
  the shorts delete and the quiz project delete call `StopExecution` and ignore
  the result (`_, _ =`), so they very likely have never stopped an execution:**
  the record says cancelled or is gone while the render task runs to the end,
  which is cost for nothing. Not changed here (a one-line addition to the same
  kind of grant would cover them; it is the user's call). Also in the shorts
  cancel: it overwrites the record with an unconditional `PutItem`, which can
  undo a job that just finished.

## Deploying it (`deploy/setup-storyvideo.sh`)

Order:

1. `bash deploy/setup-notifications.sh --apply`, so the deployed notifier knows
   kind `story`. Without it every email is silently skipped. `--apply` of the next
   script checks this with an invalid test invocation and refuses to go on.
2. `bash deploy/setup-storyvideo.sh` and read the plan.
3. `bash deploy/setup-storyvideo.sh --apply`.
4. Deploy the API code (the script prints the three commands; it does not run
   them). Until then the new environment variable does nothing.

It works in two phases: everything that can refuse (a missing prerequisite, a
lifecycle rule that could delete the videos, a wrong IAM permission, an old
notifier, a dirty tree, a definition AWS rejects) is a read-only check that runs
first, so a refusal never leaves a half-made stack. Then it makes: the stock keys
in SSM (SecureString, from `deploy/.env`, passed through a private file and never
on a command line or in the task definition); the roles; the lifecycle rules
(merged into the bucket's existing rules, because the put replaces them all);
the ECR repository and the image (tagged with the git commit, immutable, the 10
newest kept); the log group; the task definition (a new revision only when
something changed); and the state machine (the old definition is saved first).
Running it again changes nothing. It prints the undo command for each change.

It also wires the API Lambda: `STORYVIDEO_STATE_MACHINE_ARN` is added to the
function's environment, and one statement to its role (`states:StopExecution` on
this state machine's executions; `StartExecution`, the table and the bucket were
already covered, and the simulator checks that on the role's real policy). The
environment holds secrets, and `update-function-configuration` replaces all of
it, so it is handled with care: read into a private file (mode 700 directory,
removed on exit), merged there, written back with `--revision-id` so a change
made in between makes the write fail instead of being overwritten, and the
command's own output (which echoes the environment) suppressed with `--query`.
Only counts are printed. `bash deploy/setup-storyvideo.sh --unwire-api --apply`
removes that one variable again (create then answers 503; reading, cancelling and
deleting keep working). API Gateway needs no change: `deploy/setup-apigateway.sh`
sends every path to the Lambda through a single `{proxy+}` route.

`storyvideo-work/` is expired after 7 days, noncurrent versions after 1 day
(the bucket is versioned), and the delete markers that leaves behind are removed.
The script refuses if any existing rule could expire or move anything under
`storyvideo/`, including a whole-bucket rule.

## What was and was not checked

Checked:

- **The state machine against the real Step Functions engine**, read-only:
  `go test -tags live ./internal/storyjob/pipeline/`. `ValidateStateMachineDefinition`
  accepts it, and `TestState` with mocked responses confirms each routing
  decision, the DynamoDB key and expressions as the engine resolves them (including
  `$$.State.EnteredTime` as a timestamp), that the error name
  `DynamoDB.ConditionalCheckFailedException` reaches `ReadJobAgain`, that a missing
  record, a record with no status and a status of the wrong type all reach `Gone`,
  and that Spot falls to on-demand once its retries are used. This found the missing-record
  failure above, which the structural tests had passed.
- **The API routes**, against an in-memory table that enforces what the code relies
  on (the conditional update, key ranges, limits, projections, and that no
  expression placeholder is unused). It covers ownership on every route, the
  cancel condition, a cancel that races a finishing worker, a failed start, the
  limits and their boundaries, and that no response carries a storage key, scene
  text or a saved retry reason. 33 mutations of the API code were each caught by
  a test (two gaps were closed on the way: a failed plan that still has scenes,
  and listing for another user); one more is equivalent. A numeric Google user id
  is used throughout, and the ids the pipeline is started with are fed to the
  worker's own validator.
- **The deploy script, end to end, against a stateful stand-in for the `aws`,
  `docker` and `git` commands**: `python3 -m unittest discover -s deploy/tests -v`
  (53 tests).
  The stand-in keeps a model of the account, so the tests check what is left
  behind, not just which calls were made: that a dry run changes nothing, that
  every refusal happens before the first change, that a second `--apply` makes no
  change, that the key never appears in output, a command line or the task
  definition, that the existing lifecycle rules and the size setting survive the
  merge, that a new commit gets a new image and revision while the old revision keeps its
  image, that the policy checks can fail (three mutated policies are caught), that
  the API Lambda's environment keeps every other variable, is never printed and
  travels in a private file guarded by the revision, and that `--unwire-api`
  touches nothing else. 28 mutations of the script were each caught by a test.
- **The runner's deadline and SIGTERM handling**, and that the worker and
  state-machine time limits stay ordered.

Not checked, and what that leaves open:

- **The deploy script has never been run against AWS, not even as a dry run.**
  The stand-in models the CLI as the script uses it; it does not prove AWS accepts
  the calls. The calls most likely to need a fix on the first run:
  `put-bucket-lifecycle-configuration` with
  `--transition-default-minimum-object-size`, the `--cli-input-json` form of the
  IAM simulator, immutable ECR tags together with `docker push`, and
  `register-task-definition --runtime-platform`. A mistake in any of them fails the
  call, and nothing later runs; the earlier steps are safe to repeat. Run the dry run first: it
  makes only read calls and asks AWS to validate the definition and to simulate
  the policies.
- **IAM is checked against the stand-in's own matcher**, and against the real
  simulator only when the script runs (the dry run does it).
- **`TestState` reports every mocked error on a Task as retriable**, including a
  timeout, so the engine's rule for `States.TaskFailed` and timeouts was not
  confirmed. The explicit `MaxAttempts: 0` rule makes the behaviour independent of it.
- **Nothing has run on Fargate**: the real render time, whether 2 vCPU and 4 GB
  are enough, Spot interruptions, the task's access to Pexels/Pixabay through a
  security group that allows only egress 443, and the stock keys arriving through
  SSM. The first job will show all of these.
- **No real email was sent.** The notifier's `story` kind is tested with fakes.
- **The API was not exercised against real DynamoDB, Step Functions, S3 or API
  Gateway.** The conditional update and the key-range query are checked against the
  test table's model of them, which is as close as the model is. The one thing in the
  update expression no test can show is that DynamoDB accepts `SET #failedStage =
  #status` (copying a value inside one expression) next to the same name in the
  condition; the worker's store and the state machine use the same shape.
- **The deploy additions have not run against AWS**, with the same caveats as the
  rest of the script. The ones that matter here: `update-function-configuration`
  with `--revision-id` and `--query`, and the simulator's answer for
  `states:StopExecution` on the real role.
- **Presigned download links were not fetched.** The key they name is checked; that
  the Lambda role's credentials can sign a read of it is checked only by the
  simulator.

## Not done

- The UI (slice 4): the page that makes the video, shows its progress, and offers
  the download. The response shapes above are what it is built on.
- The reconciler, and the other gaps listed under "Known gaps".
- The step update the state machine writes uses millisecond timestamps where the
  worker writes whole seconds; both parse as RFC 3339, but code that compares
  them should parse, not compare strings.
