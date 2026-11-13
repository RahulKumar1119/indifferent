package storyjob

import (
	"context"
	"strings"
	"testing"
	"time"
)

// #311: while a scene waits out a file-host throttle, the job says which scene,
// so the page can tell the person and let them choose another picture.

func TestCopyMedia_ASceneWaitingOutAThrottleIsNamedOnTheJobAndClearedWhenItWorks(t *testing.T) {
	e := newEnv(t)
	slept := &sleepLog{}
	job := e.jobWith(3)
	var savedWhileWaiting []int
	var saves []int
	e.deps.SaveProgress = func(context.Context) error { saves = append(saves, job.DelayedScene); return nil }
	e.deps.Sleep = func(ctx context.Context, d time.Duration) error {
		// What the page can read at this moment is what the last save wrote.
		savedWhileWaiting = append(savedWhileWaiting, saves[len(saves)-1])
		return slept.sleep(ctx, d)
	}
	e.deps.Retry = quickRetry()
	e.deps.ThrottleWaits = []time.Duration{20 * time.Second, 40 * time.Second}
	e.assets.dlSeq["pixabay:photo:102"] = []error{throttled(), throttled(), throttled(), throttled(), throttled(), throttled()}

	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatalf("a host that stops throttling must not fail the job: %v", err)
	}
	if len(savedWhileWaiting) != 2 || savedWhileWaiting[0] != 2 || savedWhileWaiting[1] != 2 {
		t.Errorf("saved delayed scene at each long wait = %v, want [2 2]", savedWhileWaiting)
	}
	if job.DelayedScene != 0 {
		t.Errorf("DelayedScene = %d after the scene was copied, want 0", job.DelayedScene)
	}
	if last := saves[len(saves)-1]; last != 0 {
		t.Errorf("the last save still names scene %d", last)
	}
}

func TestCopyMedia_ASceneThatNeverRecoversStaysNamedForTheNextAttemptToClear(t *testing.T) {
	e := newEnv(t)
	slept := &sleepLog{}
	e.deps.Sleep = slept.sleep
	e.deps.Retry = quickRetry()
	e.deps.ThrottleWaits = []time.Duration{time.Second}
	job := e.jobWith(3)
	e.assets.dlErr["pixabay:photo:102"] = throttled()
	var last int
	e.deps.SaveProgress = func(context.Context) error { last = job.DelayedScene; return nil }
	_ = stepErr(t, CopyMedia(context.Background(), e.deps, job, nil))
	if job.DelayedScene != 2 || last != 2 {
		t.Errorf("DelayedScene = %d, last saved = %d; the failed task should leave scene 2 named", job.DelayedScene, last)
	}
}

func TestCopyMedia_AJobWithNoThrottleIsNeverMarked(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(4)
	var seen []int
	e.deps.SaveProgress = func(context.Context) error { seen = append(seen, job.DelayedScene); return nil }
	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	for _, n := range seen {
		if n != 0 {
			t.Fatalf("a save named scene %d with no throttle", n)
		}
	}
}

func TestCopyState_NamesTheLowestWaitingScene(t *testing.T) {
	st := &copyState{}
	var reported []int
	st.report = func(context.Context) { reported = append(reported, st.lowestDelayed()) }
	if st.lowestDelayed() != 0 {
		t.Fatal("nothing waits yet")
	}
	st.markDelayed(context.Background(), 5)
	st.markDelayed(context.Background(), 3)
	st.markDelayed(context.Background(), 4)
	if st.lowestDelayed() != 3 {
		t.Errorf("lowest = %d, want 3", st.lowestDelayed())
	}
	st.dropDelayed(3)
	if st.lowestDelayed() != 4 {
		t.Errorf("lowest after scene 3 worked = %d, want 4", st.lowestDelayed())
	}
	st.dropDelayed(4)
	st.dropDelayed(5)
	st.dropDelayed(9) // a scene that never waited
	if st.lowestDelayed() != 0 {
		t.Errorf("lowest = %d with none waiting, want 0", st.lowestDelayed())
	}
	if want := []int{5, 3, 3}; len(reported) != 3 || reported[0] != want[0] || reported[1] != want[1] || reported[2] != want[2] {
		t.Errorf("reported = %v, want %v", reported, want)
	}
}

func TestCopyMedia_AFailedNoteWriteDoesNotFailTheJob(t *testing.T) {
	e := newEnv(t)
	logs := &logSink{}
	e.deps.Logf, e.deps.Sleep = logs.logf, (&sleepLog{}).sleep
	e.deps.Retry = quickRetry()
	e.deps.ThrottleWaits = []time.Duration{20 * time.Second}
	job := e.jobWith(3)
	e.assets.dlSeq["pixabay:photo:102"] = []error{throttled(), throttled(), throttled()}
	calls := 0
	e.deps.SaveProgress = func(context.Context) error {
		calls++
		if job.DelayedScene != 0 {
			return context.DeadlineExceeded
		}
		return nil
	}
	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatalf("a failed note write must not fail the copy: %v", err)
	}
	if !strings.Contains(logs.text(), "storyjob save progress failed") {
		t.Errorf("the failed write was not logged:\n%s", logs.text())
	}
	if calls == 0 {
		t.Error("the note was never written")
	}
}

// --- the record

func TestItem_DelayedSceneRoundTripsAndIsOmittedWhenZero(t *testing.T) {
	j := fullJob()
	j.DelayedScene = 4
	item, err := ToItem(j)
	if err != nil {
		t.Fatal(err)
	}
	got, err := FromItem(item)
	if err != nil || got.DelayedScene != 4 {
		t.Fatalf("round trip = %d, %v", got.DelayedScene, err)
	}
	j.DelayedScene = 0
	item, _ = ToItem(j)
	if _, has := item["delayedScene"]; has {
		t.Error("a job with no delayed scene has the attribute")
	}
}

func TestDynamoStore_DelayedSceneIsWrittenWhileGoingAndRemovedWhenClearedOrEnded(t *testing.T) {
	j := fullJob()
	j.Status, j.Error, j.ErrorCode, j.FailedStage, j.OutputKey, j.CompletedAt = StatusPreparing, "", "", "", "", ""
	st, db := seeded(t, j)
	key := "USER#u1|STORYVIDEO#j1"

	j.DelayedScene = 3
	if err := st.Save(context.Background(), &j); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Load(context.Background(), "u1", "j1"); got.DelayedScene != 3 {
		t.Fatalf("saved scene = %d, want 3", got.DelayedScene)
	}

	j.DelayedScene = 0
	if err := st.Save(context.Background(), &j); err != nil {
		t.Fatal(err)
	}
	if _, has := db.items[key]["delayedScene"]; has {
		t.Error("clearing the scene left the attribute on the record")
	}

	// An ended job never keeps it, even if the caller forgot to clear it.
	j.DelayedScene = 3
	if err := st.Save(context.Background(), &j); err != nil {
		t.Fatal(err)
	}
	j.Status, j.Error, j.ErrorCode, j.FailedStage = StatusFailed, "Scene 2: gone.", CodeMediaUnavailable, StatusPreparing
	if err := st.Finish(context.Background(), &j); err != nil {
		t.Fatal(err)
	}
	if _, has := db.items[key]["delayedScene"]; has {
		t.Error("a finished job still names a delayed scene")
	}
}
