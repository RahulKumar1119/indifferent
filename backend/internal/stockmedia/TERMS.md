# Pexels and Pixabay: what the terms require

Written for issue #190 (Story2Shorts phase 1) and checked on 2026-10-29. Terms
change, so #194 repeats this review before launch. This is an engineering
summary, not legal advice.

**How each source was read.** The two API documentation pages were fetched and
read in full. The Pexels License and Pixabay Content License pages refused
automated fetches (HTTP 403), so those sections come from search-result
excerpts of the official pages and Pexels' help center. Re-read the originals
before relying on a point marked *(excerpt)*.

## API rules

### Pexels ([API documentation](https://www.pexels.com/api/documentation/))

| Rule | Where the code handles it |
|---|---|
| Send the key as the `Authorization` header, with no scheme. | `pexels.go`: header set from `Secret.reveal()`; test asserts the key is not in the URL. |
| Default limits are 200 requests per hour and 20,000 per month. Higher limits are free if the API terms are met, shown by a demo with attribution. Working around the limit ends API access. | `service.go`: 429s and an empty quota start a cooldown. `retry.go`: a 429 is retried only when the provider said how long to wait and the wait is short (2 s in the API, 30 s in the video worker), and the wait is honoured in full; a 429 with no wait (Pexels sends none) or a long one is not retried. Nothing retries during a cooldown. |
| `X-Ratelimit-Remaining` and `X-Ratelimit-Reset` (UNIX time) come back on successful responses only, never on a 429. | `pexels.go` `pexelsQuota`; `service.go` `noteQuota` waits before the next call. |
| **As of 2026-10-29, Pexels is not issuing new API keys.** The API page says issuance is paused and cannot be requested, with no reopening date (a third-party post dates it to about 30 September). Existing keys keep working. Rate-limit increases are paused too, so the default limits are the only ones available. | The service runs on Pixabay alone when there is no Pexels key (`NewServiceFromEnv` omits a provider without one). Do not assume a Pexels key can be replaced if it is lost or revoked. The Pexels code is tested against documentation fixtures only; the live checks have not been run for lack of a key. |
| Show a prominent link to Pexels whenever you make API requests, for example "Photos provided by Pexels". | Every `SearchResult` carries `Attribution` (text and URL). The API passes it on as `attributions[]` (`/stock-media/search`, `/stock-media/assets`, `/story-plans/:id`); the UI shows it (#192). |
| Credit photographers when possible, for example "Photo by John Doe on Pexels", linked to the photo page. | `Asset.Credit`, `Asset.Author`, `Asset.AuthorURL`, `Asset.PageURL`. |
| Do not copy or replicate Pexels' core functionality. | Product rule, not code: Story2Shorts uses stock as scene media, not as a browsable library. Keep it that way in #192. |
| Photo and video search take `orientation` (landscape, portrait, square). `per_page` is at most 80. Video endpoints live under `/v1/videos/`. | `pexels.go`; the older `/videos/` path is deprecated and unused. |
| Videos come as several `video_files`, each with its own size. | `pickRendition`: the largest file whose longer side is at most 1920px. |
| Video file links are tokenised CDN URLs (Vimeo). The documentation does not say how long they last. | Treated as not durable: copy the file at confirmation (#193). Vimeo hosts are on the URL allowlist for this reason. Plans store only the id, and the API looks the asset up again on every read (#192). |
| Single photos and videos can be fetched by id: `/v1/photos/:id` and `/v1/videos/videos/:id`. A missing id is a 404. | `Pexels.Get`. Tested against documentation fixtures only; not run live (no key). |

### Pixabay ([API documentation](https://pixabay.com/api/docs/))

| Rule | Where the code handles it |
|---|---|
| The key is a `key` query parameter. | `httpclient.go`: URLs never reach errors or logs, and keys are scrubbed from error text, in raw and escaped form. Tests cover refused connections, timeouts and error bodies that echo the URL. |
| 100 requests per 60 seconds per key. Headers `X-RateLimit-Limit`, `-Remaining`, `-Reset` (seconds left in the window). A 429 body reads "API rate limit exceeded". | `pixabay.go` `quota`; `service.go` cooldown, default 60s. |
| **Requests must be cached for 24 hours.** No bulk automated queries or systematic downloads. | `service.go`: Pixabay results are cached 24h (`pixabayCacheTTL`), and a test pins the boundary. The default cache is per process, which is weaker than "every request, service-wide". A shared cache is tracked in #194. |
| **Returned image URLs are for temporary display only. Permanent hotlinking is not allowed.** Download images to your own server first. Videos may be embedded, but storing them is recommended. `webformatURL` is valid for 24 hours. | Callers must copy `Asset.MediaURL` into our storage when a scene is confirmed (#193) and never store the URL. Documented on `Asset`. |
| Show users where results come from whenever search results are displayed. | `SearchResult.Attribution`. |
| Video search has **no orientation parameter**; it has `min_width` and `min_height`. Image search has `orientation` (`horizontal`, `vertical`). | `pixabay.go` sends the filter only for images; `sortByOrientation` orders video results by shape. **Observed 2026-10-29 against the live API, 16 varied queries, top 50 results each (800 videos):** 7.1% were portrait (57 of 800). Four queries returned none ("people working", "fitness workout", "business meeting", "money"); the best returned 10 of 50 ("mountain"). `min_height` did not help (it matches the top, usually 4K, rendition, which landscape clips satisfy too). Plan on Pixabay video mostly needing cropping or blur-fill (#193). Pexels was the portrait source, and it is closed to new keys. Image search with `orientation=vertical` does work (10 of 10 portrait). |
| An invalid or missing key is answered with **HTTP 400** and a plain-text body, not 401. (Observed live.) | `pixabay.go` `isPixabayKeyError` maps it to `ErrAuth`, so a dead key is not confused with a malformed request. |
| Page size is 3 to 200; at most 500 results are reachable per query. `q` is at most 100 characters. | `MinPerPage`, `MaxQueryRunes`; `Total` reports the reachable count. |
| Without full API access an image is at most 1280px (`largeImageURL`). `fullHDURL` (1920px), `imageURL` and `vectorURL` need an account approved for full API access, requested at <https://pixabay.com/api/request_full_access/>. **Our current key does not have it:** `fullHDURL` was on 0 of 20 live hits (2026-10-29). A 1280px photo scaled to a 1920px-tall frame is enlarged 1.5 times, which looks soft. | `decodePixabayPhoto` uses `fullHDURL` when present, else `largeImageURL`, so approval needs no code change. `MinLongSide` is 1280 so default-access photos are kept. Requesting access is an account-owner action, tracked in #194. |
| `large` video may be empty (no 4K version). | `pickRendition` ignores renditions with no URL. |
| A single item is fetched with `id=`. Observed live 2026-10-29: an id that no longer exists answers HTTP 404 for videos but **HTTP 200 with an empty `hits` list for images**; a malformed id answers 400. | `Pixabay.Get` maps both "missing" shapes to `ErrNotFound`, and `validateAssetID` rejects malformed ids before any request. The result is cached 24h like a search (`TestLive_GetByIDRoundTrip`). |
| `lang` accepts a fixed list; `safesearch` filters to all-ages content. | `safesearch=true` is always sent, since results end up in public videos. |

## Content licenses

### Pexels License *(excerpt)* ([license](https://www.pexels.com/license/), [help center](https://help.pexels.com/hc/en-us/articles/360042332714-What-are-the-rules-for-using-Pexels-photos-or-videos/))

- Free for personal and commercial use, with no attribution required.
- Not allowed: selling or distributing a photo or video **standalone**, meaning
  without meaningful creative change (a filter, resize or crop is not enough).
  Not allowed on other stock or wallpaper platforms either.
- Identifiable people must not be shown in a bad light or offensively, for
  example linked to crime, illness or sexual content.
- Using the API is also subject to Pexels' Terms of Service.

### Pixabay Content License *(excerpt)* ([license](https://pixabay.com/service/license-summary/))

- Free for commercial use, with no attribution required.
- Not allowed: selling or distributing content standalone.
- Content showing recognisable trademarks, logos or brands cannot be used
  commercially in relation to goods and services.
- Private homes, designed products and some buildings may need a property
  release for commercial use; being on Pixabay does not clear that.
- Not allowed in misleading or deceptive ways (reported by secondary sources;
  confirm on the license page).

## What this means for Story2Shorts

1. **Attribution is optional in the licenses but required by the API terms**
   (both providers ask that results shown to users credit the provider). Show
   `Attribution` wherever search results are displayed. In a finished video,
   credit is not required by either license; decide the product wording in #194.
2. **Copy, never link.** A confirmed asset is copied into our bucket (#193) and
   rendered from there. The provider URL is discarded.
3. **No standalone downloads.** Users must not be able to download a stock asset
   by itself. Only the rendered video is offered. Add a test for this in #193.
4. **Recognisable people, brands and homes are the user's responsibility to
   avoid, and ours to warn about.** User-facing guidance belongs in #194.
5. **Hindi cannot be searched; keywords must be English.** Pexels' `locale`
   list and Pixabay's `lang` list have no Hindi, and this was checked live on
   Pixabay (2026-10-29): Hindi-only queries returned **0 results** for 5 of 5
   words, on both images and videos, where the English equivalents returned up
   to 500; `lang=hi` is accepted and changes nothing. A query mixing the two
   ("mountain पहाड़") returns the English results, because the Hindi word is
   ignored. The failure is silent (HTTP 200, empty list), so the package rejects
   Hindi-only and other Indian-script-only queries with `ErrUnsupportedLanguage`
   instead of letting them look like "no matching media". #191 must generate
   English keywords for Hindi stories (now a stated requirement there).
   Not searched live: Pexels (no key), and scripts other than Devanagari.
6. **A third-party aggregator is not used.** An outside comment on #162
   suggested one; this package talks to the official APIs, as the issue asks.

## Not yet done (tracked)

- Shared 24-hour cache across Lambda containers (#194).
- UI attribution and "do not use recognisable people or brands" guidance
  (#192, #194).
- Asset copy into S3 on confirmation, and a test that stock files are never
  offered standalone (#193).
- Live check against the real APIs: `live_test.go`, behind the `live` build
  tag, needs real keys and is not run in CI. Pixabay passed on 2026-10-29.
  **Pexels is unverified against the live API**, because Pexels is not issuing
  new keys (see above). Until someone with an existing key runs
  `go test -tags live ./internal/stockmedia/`, treat the Pexels decoding as
  matching its documentation but not confirmed.
- Decide whether to add a third source for portrait video, since Pixabay's
  video results are mostly landscape and Pexels may stay closed. `Provider` is
  an interface, so a new source is one file; the issue names only Pexels and
  Pixabay, so this needs a product decision first.
