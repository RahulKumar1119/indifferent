package storyjob

import (
	"sort"
	"strings"
	"testing"
)

// Deleting a video removes everything under JobPrefixes and nothing else. So
// every key the worker writes must be under one of them, or a delete would leave
// a file behind; and no prefix of one job may be a prefix of another's key, or a
// delete would take a neighbour's files.
func TestJobPrefixes_CoverEveryKeyTheWorkerWritesAndNothingElse(t *testing.T) {
	const user, job = "114373949285061923844", "01939f6c-8a40-7e21-b2c4-aa11bb22cc33"
	prefixes := JobPrefixes(user, job)
	if len(prefixes) != 2 {
		t.Fatalf("prefixes = %v", prefixes)
	}
	under := func(key string) bool {
		for _, p := range prefixes {
			if strings.HasPrefix(key, p) {
				return true
			}
		}
		return false
	}
	keys := []string{OutputKey(user, job)}
	for n := 1; n <= 10; n++ {
		for _, name := range []string{"media.jpg", "media.mp4", "voice.mp3", "marks.jsonl"} {
			keys = append(keys, WorkKey(user, job, n, name))
		}
	}
	for _, k := range keys {
		if !under(k) {
			t.Errorf("the worker writes %q, which a delete would not remove", k)
		}
	}
	for _, p := range prefixes {
		if !strings.HasSuffix(p, "/") {
			t.Errorf("%q does not end in '/': job \"ab\" would match job \"abc\"", p)
		}
	}
	// Another job, and another user, are out of reach.
	const otherJob = "01939f6c-8a40-7e21-b2c4-aa11bb22cc34"
	for _, k := range []string{OutputKey(user, otherJob), WorkKey(user, otherJob, 1, "voice.mp3"), OutputKey("11437394928506192384", job), OutputKey(user+"1", job), OutputKey("x", job)} {
		if under(k) {
			t.Errorf("a delete of %s/%s would reach %q", user, job, k)
		}
	}
}

// The two prefixes must not overlap, because a lifecycle rule expires one and
// must never touch the other (see the README).
func TestJobPrefixes_TheOutputAndTheWorkingFilesDoNotOverlap(t *testing.T) {
	p := JobPrefixes("u", "j")
	if strings.HasPrefix(p[0], p[1]) || strings.HasPrefix(p[1], p[0]) {
		t.Errorf("%v overlap", p)
	}
	if !strings.HasPrefix(p[0], OutputPrefix) || !strings.HasPrefix(p[1], WorkPrefix) {
		t.Errorf("%v", p)
	}
}

func TestLanguages_AreSortedAndEachHasVoices(t *testing.T) {
	langs := Languages()
	if !sort.StringsAreSorted(langs) || len(langs) < 2 {
		t.Fatalf("languages = %v", langs)
	}
	for _, l := range langs {
		if len(Voices(l)) == 0 || DefaultVoice(l) != Voices(l)[0] {
			t.Errorf("%s: voices %v default %q", l, Voices(l), DefaultVoice(l))
		}
	}
}

// The notifier skips a failed job by comparing errorCode with this text. If the
// constant drifted, a cancel would email the user.
func TestCodes_TheNotifierComparesByTheirExactText(t *testing.T) {
	if CodeCancelled != "CANCELLED" {
		t.Errorf("CodeCancelled = %q", CodeCancelled)
	}
	if CodeStartFailed != "START_FAILED" || CodeTimedOut != "TIMED_OUT" {
		t.Errorf("codes = %q %q", CodeStartFailed, CodeTimedOut)
	}
}

// Every write other than the creation of a job uses one condition. The API's
// cancel and the state machine's MarkFailed repeat it, and the argument that a
// slow worker cannot undo a cancel rests on all of them agreeing.
func TestNotEndedCondition_NamesTheRecordAndBothEndStates(t *testing.T) {
	for _, want := range []string{"attribute_exists(PK)", "#status <> :completed", "#status <> :failed"} {
		if !strings.Contains(NotEndedCondition, want) {
			t.Errorf("%q lacks %q", NotEndedCondition, want)
		}
	}
}
