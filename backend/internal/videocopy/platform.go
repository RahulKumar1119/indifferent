package videocopy

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Platform names a place a finished video is posted. The list a caller serves
// is data (see Platforms), so a fourth platform is a new PlatformSpec and not a
// new code path.
type Platform string

const (
	YouTube   Platform = "youtube"
	TikTok    Platform = "tiktok"
	Instagram Platform = "instagram"
)

// Unit is what a platform counts its limit in. The three differ, and the
// difference matters outside English: a Hindi letter is one character, one
// UTF-16 unit and three UTF-8 bytes.
type Unit string

const (
	// UnitChars counts Unicode characters (code points).
	UnitChars Unit = "characters"
	// UnitBytes counts UTF-8 bytes. YouTube's Data API limits a description to
	// 5,000 bytes.
	UnitBytes Unit = "bytes"
	// UnitUTF16 counts UTF-16 code units. TikTok's developer documentation
	// limits a caption to 2,200 of them; an emoji outside the basic plane is two.
	UnitUTF16 Unit = "utf16"
)

// Count returns the length of s in unit u.
func Count(s string, u Unit) int {
	switch u {
	case UnitBytes:
		return len(s)
	case UnitUTF16:
		n := 0
		for _, r := range s {
			n += len(utf16.Encode([]rune{r}))
		}
		return n
	default:
		return utf8.RuneCountInString(s)
	}
}

// PlatformSpec is everything the server needs to know about one platform.
// Every number here is a limit the server enforces, on what the model writes
// and on what a person saves.
type PlatformSpec struct {
	ID    Platform `json:"id"`
	Label string   `json:"label"`
	// HasTitle says the platform has a title separate from its text.
	HasTitle  bool `json:"hasTitle"`
	TitleMax  int  `json:"titleMax,omitempty"`
	TitleUnit Unit `json:"titleUnit,omitempty"`
	// TextLabel is what the platform calls its text: "Description" or "Caption".
	TextLabel string `json:"textLabel"`
	// TextMax limits the text with the hashtags appended (what is pasted).
	TextMax  int  `json:"textMax"`
	TextUnit Unit `json:"textUnit"`
	// Aim is how long the text should be, in characters, for the prompt. It is
	// advice to the model; only TextMax is enforced.
	Aim int `json:"aim"`
	// HashtagsMax is the most hashtags kept. YouTube, TikTok and Instagram are
	// each held to five: a product rule that stays under every platform's own
	// cap, not a limit any of them states.
	HashtagsMax int `json:"hashtagsMax"`
	// HashtagsMin is how many the prompt asks for.
	HashtagsMin int `json:"hashtagsMin"`
}

// MaxHashtagRunes bounds one hashtag, the # included.
const MaxHashtagRunes = 31

// Platforms returns the platforms served, in display order. The limits are
// values, not code: a test pins them, and a change is one line here.
func Platforms() []PlatformSpec {
	return []PlatformSpec{
		{
			ID: YouTube, Label: "YouTube", HasTitle: true,
			TitleMax: 100, TitleUnit: UnitChars,
			TextLabel: "Description", TextMax: 5000, TextUnit: UnitBytes, Aim: 700,
			HashtagsMin: 3, HashtagsMax: 5,
		},
		{
			ID: TikTok, Label: "TikTok",
			TextLabel: "Caption", TextMax: 2200, TextUnit: UnitUTF16, Aim: 220,
			HashtagsMin: 3, HashtagsMax: 5,
		},
		{
			ID: Instagram, Label: "Instagram Reels",
			TextLabel: "Caption", TextMax: 2200, TextUnit: UnitUTF16, Aim: 400,
			HashtagsMin: 3, HashtagsMax: 5,
		},
	}
}

// PlatformByID finds a spec in specs.
func PlatformByID(specs []PlatformSpec, id Platform) (PlatformSpec, bool) {
	for _, s := range specs {
		if s.ID == id {
			return s, true
		}
	}
	return PlatformSpec{}, false
}

// PlatformCopy is the words for one platform.
type PlatformCopy struct {
	Title    string   `json:"title,omitempty"`
	Text     string   `json:"text"`
	Hashtags []string `json:"hashtags"`
}

// FullText is what a person pastes: the text, a blank line, then the hashtags
// on one line. It is the text alone when there are no hashtags.
func FullText(c PlatformCopy) string {
	if len(c.Hashtags) == 0 {
		return c.Text
	}
	if c.Text == "" {
		return strings.Join(c.Hashtags, " ")
	}
	return c.Text + "\n\n" + strings.Join(c.Hashtags, " ")
}

// generic is the set of hashtags that say nothing about the video. They were
// the old fixed suffix of the clip caption button and they are never offered
// or kept: a tag has to be about this video.
var generic = map[string]bool{
	"fyp": true, "fypage": true, "foryou": true, "foryoupage": true, "fy": true, "viral": true, "viralvideo": true,
	"shorts": true, "short": true, "youtubeshorts": true, "ytshorts": true, "shortvideo": true, "shortsvideo": true,
	"reels": true, "reel": true, "instareels": true, "instagram": true, "instagood": true, "explore": true, "explorepage": true,
	"trending": true, "trend": true, "tiktok": true, "tiktokviral": true, "video": true, "videos": true, "youtube": true,
	"follow": true, "followme": true, "like": true, "likeforlike": true, "subscribe": true, "share": true, "love": true,
	"goviral": true, "viralreels": true, "trendingreels": true, "foryourpage": true, "xyzbca": true,
}

// IsGenericTag reports whether tag (with or without #) is one of the generic tags.
func IsGenericTag(tag string) bool {
	return generic[strings.ToLower(strings.TrimPrefix(strings.TrimSpace(tag), "#"))]
}

// NormalizeHashtag turns what a model or a person wrote into one hashtag: a #,
// then letters, combining marks (the vowel signs of Devanagari), digits and
// underscores. Spaces and other punctuation inside the word are removed, so
// "#Brass Lamp!" becomes "#BrassLamp". It returns "" when nothing usable is left.
func NormalizeHashtag(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "#\uFF03 ")
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '_':
			b.WriteRune(r)
		case unicode.IsMark(r) && b.Len() > 0:
			b.WriteRune(r)
		case r == '\u200c' || r == '\u200d':
			// Joiners are part of how some Indic words are spelled; keep them
			// inside a word, never at its edge.
			if b.Len() > 0 {
				b.WriteRune(r)
			}
		}
	}
	w := strings.TrimRight(b.String(), "\u200c\u200d")
	if w == "" || !strings.ContainsFunc(w, unicode.IsLetter) {
		return ""
	}
	if utf8.RuneCountInString(w)+1 > MaxHashtagRunes {
		return ""
	}
	return "#" + w
}

// ValidateHashtag says whether a person-written tag is fit to save as it stands.
func ValidateHashtag(tag string) error {
	if !strings.HasPrefix(tag, "#") || NormalizeHashtag(tag) != tag {
		return fmt.Errorf("%q is not a hashtag: use a # and then letters, digits or underscores", tag)
	}
	return nil
}
