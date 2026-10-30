package storyplan

import "github.com/rahul/indifferent/backend/internal/shorts"

// Story input limits. They count characters, not bytes: Hindi takes three bytes
// per letter in UTF-8, so a byte limit would allow about a third of the story.
const (
	// MinStoryRunes rejects input too thin to plan from ("hi"). Ideas of a
	// sentence are fine; this is only a floor against empty-looking requests.
	MinStoryRunes = 30
	// MaxStoryRunes caps input. A video narrates at most MaxTotalSeconds of
	// speech (about 225 words), so a longer story is summarised; this bound
	// keeps the prompt, and the Bedrock bill, small.
	MaxStoryRunes = 5000
)

// Plan limits. A returned Plan always satisfies all of them.
const (
	// MinScenes and MaxScenes bound the number of scenes. Fewer than two is
	// not a sequence of visuals; more than ten cannot be shown in under
	// MaxTotalSeconds without flashing by.
	MinScenes = 2
	MaxScenes = 10

	// MinSceneSeconds and MaxSceneSeconds bound one scene. Under two seconds a
	// stock clip cannot be read; over fifteen the picture goes stale.
	MinSceneSeconds = 2.0
	MaxSceneSeconds = 15.0

	// MinTotalSeconds and MaxTotalSeconds bound the whole video. The upper
	// bound matches the longest AI Shorts clip, so every output format the
	// product already makes fits the same platforms.
	MinTotalSeconds = 10.0
	MaxTotalSeconds = shorts.MaxClipDuration

	// TargetMinSeconds and TargetMaxSeconds are what the prompt asks for. They
	// sit inside the hard limits so a model that overshoots a little still
	// produces a valid plan.
	TargetMinSeconds = 30
	TargetMaxSeconds = 60
)

// Per-field limits, in characters.
const (
	MaxHookWords        = 20
	MaxHookRunes        = 200
	MaxNarrationRunes   = 600
	MaxDescriptionRunes = 300
	MaxKeywords         = 5
	MaxKeywordRunes     = 40
	MaxLanguageRunes    = 3
)

// WordsPerSecond is the speaking rate used to judge whether a scene's duration
// fits its narration: about 150 words a minute, a normal pace for a short.
// It is an approximation. The real length is known only once the speech is
// synthesised (phase 4), so the duration here is a plan, not a measurement.
const WordsPerSecond = 2.5

// durationTolerance is how far a model-supplied scene duration may differ from
// the narration's estimated speaking time before it is replaced by the
// estimate: 40 percent either way.
const durationTolerance = 0.4

// scenePad is added to the estimated speaking time so a scene does not end
// the instant the last word does.
const scenePad = 0.5

// Media types a scene can prefer.
const (
	MediaPhoto = "photo"
	MediaVideo = "video"
)

// defaultMedia is used when the model gives no usable media type. Photos are
// the safer default: in the live check of Pixabay, 10 of 10 image results were
// portrait but only 7.1% of video results were (see stockmedia/TERMS.md).
const defaultMedia = MediaPhoto

// MaxOutputTokens is the output budget requested from the model. A full plan is
// about 1,500 tokens in English and several times that in Devanagari, where one
// word costs more tokens; Kimi's hidden reasoning tokens count against the same
// limit.
const MaxOutputTokens = 8000
