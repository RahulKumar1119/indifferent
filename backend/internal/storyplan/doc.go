// Package storyplan turns a user's story into a plan for a short vertical
// video: a hook, then an ordered list of scenes, each with narration, a
// duration, a visual description, English stock-search keywords and a
// preferred media type (issue #162, phase 2 / #191).
//
// It asks a Bedrock model for the plan and never trusts the answer. Whatever
// the model returns is parsed defensively, cleaned, checked against the limits
// in limits.go, repaired where that is safe, and rejected with a typed error
// where it is not. A plan that comes out of Generate satisfies every
// invariant below, whatever the model or the story said.
//
// # Language
//
// Narration and the hook stay in the story's own language and script and are
// never translated. This is stated in the prompt and checked on the way out,
// because issue #158 showed that an unstated language rule makes a model
// answer a Hindi text in English some of the time. Search keywords are always
// English: Pexels and Pixabay return no results at all for Hindi (checked live,
// see backend/internal/stockmedia/TERMS.md), so a keyword written in an Indian
// script is removed, and a scene left with no keyword is dropped. Scene
// descriptions are English too, so one can be used as alt text or a search
// fallback.
//
// # Untrusted input
//
// The story is material to summarise, not instructions. It is fenced in the
// prompt, its closing delimiter is neutralised, and the output contract is
// enforced by validation, so text in the story cannot change the schema. All
// text in a plan is plain text: markup is stripped so it cannot become SSML in
// a later text-to-speech call or HTML in the UI, and control characters are
// removed. Story text is never logged; only its length is.
//
// # Invariants of a returned Plan
//
//   - MinScenes <= len(Scenes) <= MaxScenes
//   - MinTotalSeconds <= TotalSeconds <= MaxTotalSeconds
//   - every scene has narration, a description, 1..MaxKeywords English
//     keywords and a media type of photo or video
//   - narration and hook are mostly in the story's script
//   - Scenes[0] is the scene the hook opens: its narration is expected to begin
//     with the hook; later phases decide how to present it
//
// The package does no I/O of its own. The model is behind the Completer
// interface, which *rank.Service implements.
package storyplan
