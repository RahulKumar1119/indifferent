// Package stockmedia searches the Pexels and Pixabay stock libraries for the
// photos and video clips Story2Shorts puts under narration (issue #162,
// phase 1 / #190).
//
// It has three layers:
//
//   - Pexels and Pixabay, one per API. Each turns its provider's response into
//     the shared Asset shape and maps HTTP failures to ProviderError.
//   - Service, which wraps providers with parameter validation, a response
//     cache, rate-limit cooldowns, asset validation and orientation ordering,
//     and can query one provider or all of them.
//   - ValidateAsset and Service.CheckAvailable, the checks an asset has to pass
//     before it is copied or rendered.
//
// The rules this code is built around are summarised, with sources, in
// TERMS.md. The ones that shape the design:
//
//   - Pixabay requires search requests to be cached for 24 hours and forbids
//     permanent hotlinking, so callers must copy a chosen asset into our own
//     storage and never keep a provider URL.
//   - Pexels and Pixabay both ask that results shown to users credit the
//     provider; every SearchResult carries the text and link to show.
//   - Pixabay passes its API key in the URL query string. Nothing in this
//     package puts a URL in an error or a log line, and keys are wrapped in
//     Secret so they print as [REDACTED].
//   - Provider rate limits are respected, not worked around: a 429 starts a
//     cooldown during which the provider is not called.
//
// Nothing here is exposed over HTTP yet. Later phases add the endpoints that
// call Service.
package stockmedia
