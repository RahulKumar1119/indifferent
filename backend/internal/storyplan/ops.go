package storyplan

import "strings"

// Scene operations change the structure of a plan: which scenes there are and
// in what order. They are sent with a save and applied in order to the saved
// plan, before the per-scene edits (#326).
const (
	OpMove  = "move"
	OpSplit = "split"
	OpMerge = "merge"
)

// MaxOps bounds the operations in one save. A user who reorders ten scenes
// needs about ten; the cap only stops a request from making the server loop.
const MaxOps = 100

// SceneOp is one structural change. Scene numbers are 1-based and refer to the
// list as it stands when the operation runs, so operations compose in order.
//
//   - move: take scene From and put it at position To.
//   - split: cut Scene in two. Both halves keep the picture, description and
//     keywords; the caller supplies the narration of each half in the edits.
//   - merge: join Scene with the one after it. The result keeps the first
//     scene's picture, description and keywords, and speaks both narrations.
type SceneOp struct {
	Op    string `json:"op"`
	Scene int    `json:"scene,omitempty"`
	From  int    `json:"from,omitempty"`
	To    int    `json:"to,omitempty"`
}

// ApplyPlanChange applies ops to plan, then edits to the result, and returns
// the restructured plan (ops only, before edits) and the final plan. With no
// ops it is ApplyEdits. edits[i] is for scene i of the list the operations
// produce. A scene made by a split must be given its narration in the edits,
// since splitting does not choose where the text is cut: the page knows what
// the user is looking at, which may differ from what was saved. Nothing is
// applied unless every operation and edit is valid.
func ApplyPlanChange(plan Plan, ops []SceneOp, edits []SceneEdit, story Script) (restructured, final Plan, err error) {
	if len(ops) == 0 {
		final, err = ApplyEdits(plan, edits, story)
		return plan, final, err
	}
	if len(ops) > MaxOps {
		return Plan{}, Plan{}, invalidEdit("too many changes to the scene order at once")
	}
	scenes := make([]Scene, len(plan.Scenes))
	copy(scenes, plan.Scenes)
	needs := make([]bool, len(scenes)) // scenes whose narration the edits must supply

	for _, op := range ops {
		switch op.Op {
		case OpMove:
			n := len(scenes)
			if op.From < 1 || op.From > n || op.To < 1 || op.To > n {
				return Plan{}, Plan{}, invalidEdit("that scene move is outside the plan")
			}
			if op.From == op.To {
				return Plan{}, Plan{}, invalidEdit("a scene cannot move to where it already is")
			}
			scenes, needs = moveAt(scenes, needs, op.From-1, op.To-1)
		case OpSplit:
			if op.Scene < 1 || op.Scene > len(scenes) {
				return Plan{}, Plan{}, invalidEdit("that scene to split is outside the plan")
			}
			if len(scenes) >= MaxScenes {
				return Plan{}, Plan{}, invalidEdit("a video has at most %d scenes, so no scene can be split", MaxScenes)
			}
			i := op.Scene - 1
			half := scenes[i]
			half.Keywords = append([]string(nil), half.Keywords...)
			if half.Media != nil {
				m := *half.Media
				half.Media = &m
			}
			scenes = insertScene(scenes, i+1, half)
			needs[i] = true
			needs = insertBool(needs, i+1, true)
		case OpMerge:
			if op.Scene < 1 || op.Scene >= len(scenes) {
				return Plan{}, Plan{}, invalidEdit("that scene has no scene after it to merge with")
			}
			i := op.Scene - 1
			joined := strings.TrimSpace(scenes[i].Narration + " " + scenes[i+1].Narration)
			merged := scenes[i]
			merged.Narration = joined
			if text, cerr := CleanNarration(joined, ScriptNone, i+1); cerr == nil && text == joined {
				merged.Duration, _ = sceneDuration(nil, estimateSpeech(text))
				needs[i] = needs[i] || needs[i+1]
			} else {
				// Too long for one scene: the page must send a shortened text.
				needs[i] = true
			}
			scenes[i] = merged
			scenes = append(scenes[:i+1], scenes[i+2:]...)
			needs = append(needs[:i+1], needs[i+2:]...)
		default:
			return Plan{}, Plan{}, invalidEdit("unknown change to the scene order")
		}
	}

	if len(scenes) < MinScenes {
		return Plan{}, Plan{}, invalidEdit("a video needs at least %d scenes", MinScenes)
	}
	if len(edits) != len(scenes) {
		return Plan{}, Plan{}, invalidEdit("the edit has %d scenes but the plan now has %d; reload the plan and try again", len(edits), len(scenes))
	}
	for i, need := range needs {
		if need && edits[i].Narration == nil {
			return Plan{}, Plan{}, invalidEdit("scene %d: the narration must be given for a scene that was split or merged", i+1)
		}
	}
	restructured = plan
	restructured.Scenes = scenes
	restructured.TotalSeconds = round1(sumDurations(scenes))
	final, err = ApplyEdits(restructured, edits, story)
	return restructured, final, err
}

func moveAt(scenes []Scene, flags []bool, from, to int) ([]Scene, []bool) {
	s, f := scenes[from], flags[from]
	scenes = append(scenes[:from], scenes[from+1:]...)
	flags = append(flags[:from], flags[from+1:]...)
	return insertScene(scenes, to, s), insertBool(flags, to, f)
}

func insertScene(s []Scene, at int, v Scene) []Scene {
	s = append(s, Scene{})
	copy(s[at+1:], s[at:])
	s[at] = v
	return s
}

func insertBool(s []bool, at int, v bool) []bool {
	s = append(s, false)
	copy(s[at+1:], s[at:])
	s[at] = v
	return s
}
