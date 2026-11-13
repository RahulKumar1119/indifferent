# Story2Shorts launch checklist: stock media licensing and attribution

Issue #194. Written 2026-11-04. This is an engineering summary of what the Pexels and
Pixabay terms ask for and what the product does about it. **It is not legal advice and
no lawyer has read it.** [TERMS.md](TERMS.md) has the detail of each provider's API rules;
this file is the result of checking them against the product.

**Re-read both providers' terms before the first public or commercial launch, and again
whenever either announces a change.** The terms below are as read on the date above. As a
rule of thumb, at least every six months (that interval is a suggestion, not a requirement
of either provider).

Status words: **done** means the product does it and a test or a page proves it;
**owner** means a decision or an account action only the owner can take;
**not verified** means I could not check it.

## How the sources were read (2026-11-04)

| Source | How | Result |
|---|---|---|
| [Pixabay Content License summary](https://pixabay.com/service/license-summary/) | fetched in full | Read. It says only the full license is legally binding, and the full text was not fetched. |
| [Pixabay API documentation](https://pixabay.com/api/docs/) | fetched (the parts on caching, hotlinking and showing the source) | Read. |
| [Pexels API documentation](https://www.pexels.com/api/documentation/) | fetched (guidelines) | Read. |
| [Pexels: rules for using photos or videos](https://help.pexels.com/hc/en-us/articles/360042332714-What-are-the-rules-for-using-Pexels-photos-or-videos) | fetched in full | Read. |
| [Pexels License](https://www.pexels.com/license/) | fetched | Returned only the "what you can do" half. The restrictions come from the help article above. |
| [Pexels help: what is the license of the photos and videos](https://help.pexels.com/hc/en-us/articles/360042295174-What-is-the-license-of-the-photos-and-videos-on-Pexels) | search-result excerpt only (the page itself was not fetched) | Says the photos and videos can be used free, for personal and commercial use, **without attribution**. This is the only source for the Pexels half of "no credit required" below. |
| Pexels Terms of Service, the Pexels "Terms and Conditions" help article, Pixabay Terms of Service, Pexels "AI and ML FAQ", Pixabay full Content License | refused (HTTP 403) | **Not verified** beyond search-result excerpts: Pexels defines "standalone" as no meaningful creative effort (a filter, resize or crop alone is not enough); Pexels says uploading its videos to YouTube is fine when they are used to make your own video. |

## 1. Attribution

| Item | Status | What the product does, and the proof |
|---|---|---|
| Show where results come from whenever they are displayed (Pixabay API); show a prominent link to Pexels when using its API (Pexels API) | **done**, with one gap | The picker shows each answering provider's link above the results, in view without scrolling at 320, 375, 768 and 1280px, and also when a search found nothing. The editor footer shows it for the providers of the chosen pictures. Proof: `story-media-picker.component.spec.ts` ("crediting the libraries and the care note") and `e2e/story-licensing-verification.mjs`. **Gap:** nothing is shown while a search is still loading or after one has failed, because no results are on screen then. Pexels words its rule as "whenever you are doing an API request", read strictly that includes those moments. I judged a link beside displayed results to be the intent. **Owner** to confirm. |
| Credit the photographer where possible (Pexels API) | **done** | Every result and every scene shows the ready-made credit line, linked to the provider's page ("Photo 301 by Jo Doe on Pixabay"). Proof: the picker and scene card specs. |
| Credit inside the finished video | **decision: not added** | Pixabay's summary (read in full) says content can be used without attribution; the Pexels help article (an excerpt, see the table above) says the same for Pexels. So nothing is burned into the video. If the Pexels page, once it can be read in full, says otherwise, the panel sentence and Terms section 05 must change. The video panel says this in one sentence and points to the credit line under each scene. The credits shown are those of the current plan, so after the plan is edited they can differ from an older video's (the panel already says when the plan has changed since). **Owner** can reverse the decision: a credit list or a burned-in line is a small feature. |

## 2. Never offered standalone, never used outside the license

| Item | Status | What the product does, and the proof |
|---|---|---|
| The stock file is never offered as a download by itself | **done** | The search and lookup responses leave out the full-resolution URL (`mediaUrl`), and the saved plan holds only `{provider, type, id}`. Proof: `internal/api/stockmedia_test.go` (response never contains `SECRET-ORIGINAL` or `mediaUrl`), `TestUpdatePlan_NarrationAndMediaRoundTrip` (the stored plan has no provider data), and the picker spec "never shows or links the full-resolution file". The only download is the finished MP4 at the job's own key (`TestStoryVideos_DownloadNeverLinksToAFileThatIsNotTheJobsOwn`). |
| Previews | **known, inherent** | The picker shows the provider's own preview image or clip, loaded by the browser from the provider, so a person could save a preview. This is how showing search results works. The privacy page says the browser loads previews from the providers. |
| A video is a new work, not the stock file "as is" | **done, by design** | Each scene is framed to 9:16 (blur-fill for wide pictures), photos are slowly zoomed, scenes are cross-faded, captions are burned in and narration is added (`backend/internal/storyvideo/README.md`, "The pipeline"). That is creative work on top of the content, not the "standalone" use both licenses forbid. |
| No hotlinking; images copied before use (Pixabay API) | **done** | Plans keep no URL; a video job copies each chosen file into private storage (`storyjob.CopyMedia`) and renders from the copy; copies expire after 7 days (the bucket rule in `deploy/setup-storyvideo.sh`, **not yet applied**). Previews are shown only as search results, which the Pixabay docs allow. |
| No bulk or systematic downloading (Pixabay API) | **done** | One file per scene the user chose; searches are made one at a time by a person, rate limited to 120 an hour each, with a 24-hour shared cache (#211, **not yet deployed**). |
| Auto-pick searches (#302) | **owner to confirm with Pixabay; off** | With `STORY_AUTOPICK_ENABLED` on, making a plan runs one Pixabay search per scene (about 7, at most 10, at most 3 at a time) inside that person's request, and downloads nothing. The API docs say the API is for real human requests and ask for no lots of automated queries. Whether a search made for a person's own plan counts is **not verified**: the docs page answered HTTP 429 to our fetch. Ask Pixabay before turning it on. Guards in code: quota reserve of 30, no retries, the 429 cooldown, the 24-hour cache, and no background or prefetch searches. |
| Results cached for 24 hours (Pixabay API) | **done in code** | `stockmedia` caches Pixabay results for 24 hours per container. The cache shared by every container is #211 and needs `deploy/setup-stock-cache.sh`. |
| Do not copy or replicate the provider's core functionality (Pexels API) | **owner to judge** | The picker finds media for a scene; it is not a browsable library, shows 12 results a page, and offers no collections or wallpapers. This is a judgment about the product, not something a test can prove. |
| Stock files are not fed to AI services or used to train anything | **done** | No code in the story pipeline sends a picture or clip to Amazon Rekognition, Bedrock or any other model (searched `storyjob`, `storyvideo`, `stockmedia` and `cmd/storyrender`: none). Only narration text goes to Amazon Polly and the story to a Bedrock model. What Pexels' own terms say about AI use is **not verified** (the page refused the fetch). |

## 3. Trademarks, recognisable people, privacy

Both licenses leave these to the user, so the product tells the user. Neither provider clears
every use of what a picture shows. Pixabay says content may carry other rights and that it is
the user's responsibility to check; Pexels says identifiable people must not appear in a bad
light, and that people or brands must not be implied to endorse your product.

| Item | Status | Where |
|---|---|---|
| Trademarks and logos | **done in text** | Picker note ("avoid visible logos or brand names"); Terms section 05. |
| Recognisable people, bad light, implied endorsement | **done in text** | Picker note; Terms section 05. |
| Private homes and property releases | **done in text** | Picker note ("private homes"); Terms section 05 ("does not mean that permission was given"). |
| Misleading or deceptive use | **done in text** | Terms section 05. |
| Political use | **not covered** | A Pexels help article (excerpt only) says photos with identifiable people or trademarks may not be used to promote parties, candidates or political viewpoints. The Terms and the picker do not say this. **Owner** to decide whether to add it. |
| The picker's note is advice, not a gate | **by design** | There is no checkbox and nothing is blocked. The user remains responsible, as the Terms say. |

## 4. Terms and privacy pages

Changed together with the date (`LEGAL_LAST_UPDATED`, now 2026-11-04). The pages describe Story
videos as "being introduced and may not be available to every account yet", because the rest of
the site still says three tools and the feature is not switched on.

| Page | What changed |
|---|---|
| Terms | The tool is named; stories and their plans and videos are the user's, the stock pictures are not; a new section 05 "Pictures and clips from other libraries" (whose they are, the download is the video only, no credit required, logos, people, property, removal by a library, no promise of clearance); automatic output can mismatch a scene; story videos can be deleted, plans only by email. |
| Privacy | What is sent where: the story to a Bedrock model, and **not kept** (only its length), but the plan made from it **is kept** and may follow the story's wording closely; search words go from our server to Pexels and Pixabay with no name or email; **the browser loads previews directly from the providers, who can see its IP address**; only narration text goes to Polly; no picture goes to a recognition service; working files expire after 7 days; the finished video stays until deleted; **plans have no delete button**; metrics, logs and the shared cache hold no story text or personal data; story-video emails. |

Open points, all **owner**:

- **No way to delete a plan in the app.** The privacy page says so and offers removal on request.
  A plan holds narration that can follow the user's story closely. A `DELETE /story-plans/{id}`
  route and a button would close this; it was not built in this slice.
- **Previews expose the visitor's address to the providers.** Serving previews through our own API
  would avoid it, at a bandwidth and caching cost. The page discloses it instead.
- **Not changed, and for the owner to decide with a lawyer:** governing law, the minimum age (13),
  liability wording, any company name or address. None is stated in the pages today beyond what
  was already there.
- **The 7-day expiry of copies is only true once `deploy/setup-storyvideo.sh --apply` has run.**
  The privacy page states it as how the service works; do not switch Story videos on before it has.

## 5. Provider accounts

| Item | Status |
|---|---|
| Pexels is not issuing new API keys (as of 2026-10-29) | **not re-checked** on 2026-11-04. The service runs on Pixabay alone when there is no Pexels key, and a lost Pexels key may not be replaceable. |
| Pixabay full API access (1920 px `fullHDURL`) | **owner.** Requested 2026-10-29. Checked again on 2026-11-04 from a fresh process (`TestLive_PixabayFullAccess`): `fullHDURL` on **0 of 40** image hits, so it is either still pending or was refused; the live API cannot say which. Photos are capped at 1280 px until it is granted. When approved: run `deploy/flush-stockmedia-cache.sh --apply`, then the live test again. Record the answer on #194. |
| Rate limits (Pexels 200 an hour and 20,000 a month; Pixabay 100 a minute) | **done in code**: cooldowns, honoured waits, failover (#210). |
| Higher Pexels limits need a demonstration with attribution | **owner**, only if the default limits are outgrown. The attribution above is what would be shown. |

## 6. Before the first public launch

1. Re-read both providers' license and API terms (section 1 of the table above lists what could and could not be fetched).
2. Have the Terms and Privacy pages read by someone qualified.
3. Decide the open points in section 4.
4. Run `deploy/setup-storyvideo.sh`, `setup-stock-cache.sh` and `setup-story-metrics.sh` (each is a read-only dry run until `--apply`), then switch Story videos on.
5. Link this checklist from #194.
