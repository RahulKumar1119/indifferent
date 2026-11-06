package storyjob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

// peakCounter counts calls in progress together and remembers the most there
// were at once.
type peakCounter struct {
	mu        sync.Mutex
	cur, peak int
}

func (p *peakCounter) enter() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cur++
	p.peak = max(p.peak, p.cur)
	return p.cur
}

func (p *peakCounter) leave() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cur--
}

func (p *peakCounter) mostAtOnce() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.peak
}

// wait receives from ch or fails the test after a few seconds.
func wait[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		panic("unreachable")
	}
}

// ---- runParallel -----------------------------------------------------------

func TestRunParallel_OneWorkerIsAPlainLoopInOrder(t *testing.T) {
	var order []int
	errs, left := runParallel(context.Background(), 1, []int{4, 2, 7}, nil, func(_ context.Context, i int) error {
		order = append(order, i) // one goroutine, so no lock is needed
		return nil
	})
	if len(errs) != 0 || len(left) != 0 {
		t.Fatalf("errs=%v left=%v, want neither", errs, left)
	}
	if fmt.Sprint(order) != "[4 2 7]" {
		t.Errorf("order = %v, want the order given", order)
	}
}

func TestRunParallel_NeverRunsMoreThanWorkersAtOnce(t *testing.T) {
	var pc peakCounter
	idx := []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
	errs, left := runParallel(context.Background(), 3, idx, nil, func(context.Context, int) error {
		pc.enter()
		time.Sleep(15 * time.Millisecond)
		pc.leave()
		return nil
	})
	if len(errs) != 0 || len(left) != 0 {
		t.Fatalf("errs=%v left=%v", errs, left)
	}
	if got := pc.mostAtOnce(); got != 3 {
		t.Errorf("most at once = %d, want 3", got)
	}
}

func TestRunParallel_AFailureStopsNewWorkAndLetsRunningWorkFinish(t *testing.T) {
	var finished atomic.Int32
	var startedOnce sync.Once // more than one other index starting is the bug under test, not a reason to panic
	oneStarted := make(chan struct{})
	release := make(chan struct{})
	boom := errors.New("boom")
	errs, left := runParallel(context.Background(), 2, []int{0, 1, 2, 3}, nil, func(_ context.Context, i int) error {
		if i == 0 {
			<-oneStarted // 1 must be under way before 0 fails
			close(release)
			return boom
		}
		startedOnce.Do(func() { close(oneStarted) })
		<-release
		time.Sleep(30 * time.Millisecond) // the failure of 0 is recorded by now
		finished.Add(1)
		return nil
	})
	if !errors.Is(errs[0], boom) || len(errs) != 1 {
		t.Errorf("errs = %v, want only scene 0's error", errs)
	}
	if finished.Load() != 1 {
		t.Errorf("finished = %d, want the one already running to finish", finished.Load())
	}
	if fmt.Sprint(left) != "[2 3]" {
		t.Errorf("not started = %v, want [2 3]", left)
	}
}

func TestRunParallel_StopHaltsNewWorkOnly(t *testing.T) {
	var stopped atomic.Bool
	var started atomic.Int32
	_, left := runParallel(context.Background(), 1, []int{0, 1, 2, 3}, stopped.Load, func(_ context.Context, i int) error {
		started.Add(1)
		if i == 1 {
			stopped.Store(true)
		}
		return nil
	})
	if started.Load() != 2 || fmt.Sprint(left) != "[2 3]" {
		t.Errorf("started %d, left %v; want 2 started and [2 3] left", started.Load(), left)
	}
}

func TestRunParallel_ACancelledContextStartsNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	_, left := runParallel(ctx, 3, []int{0, 1}, nil, func(context.Context, int) error { called = true; return nil })
	if called || len(left) != 2 {
		t.Errorf("called=%v left=%v, want nothing started", called, left)
	}
}

func TestFirstError_IsTheLowestScene(t *testing.T) {
	a, b := errors.New("a"), errors.New("b")
	if got := firstError(map[int]error{5: b, 2: a, 9: b}); got != a {
		t.Errorf("got %v, want the error of scene 2", got)
	}
	if firstError(nil) != nil {
		t.Error("no errors must give nil")
	}
}

// ---- CopyMedia, side by side -----------------------------------------------

func TestCopyMedia_Parallel_RunsExactlyThatManyScenesAtOnce(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 3
	var saves atomic.Int32
	e.deps.SaveProgress = func(context.Context) error { saves.Add(1); return nil }
	started := make(chan string, 16)
	release := make(chan struct{})
	var pc peakCounter
	e.assets.onDownload = func(key string) {
		pc.enter()
		defer pc.leave()
		started <- key
		<-release
	}
	job := e.jobWith(6)

	result := make(chan error, 1)
	go func() { result <- CopyMedia(context.Background(), e.deps, job, nil) }()

	for range 3 {
		wait(t, started, "three downloads to start together")
	}
	select {
	case k := <-started:
		t.Fatalf("a fourth download (%s) started while three were held open", k)
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	if err := wait(t, result, "CopyMedia"); err != nil {
		t.Fatalf("CopyMedia: %v", err)
	}

	if got := pc.mostAtOnce(); got != 3 {
		t.Errorf("most downloads at once = %d, want 3", got)
	}
	if saves.Load() != 6 {
		t.Errorf("SaveProgress called %d times, want once per scene (6)", saves.Load())
	}
	for i, s := range job.Scenes {
		if s.Media == nil {
			t.Fatalf("scene %d has no media", i+1)
		}
		if want := WorkKey("u1", "j1", i+1, "media.jpg"); s.Media.Key != want {
			t.Errorf("scene %d key = %q, want %q (each copy goes under its own scene)", i+1, s.Media.Key, want)
		}
		if got, want := string(e.files.files["bucket/"+s.Media.Key]), fmt.Sprintf("bytes-of-pixabay:photo:%d", 101+i); got != want {
			t.Errorf("scene %d stored %q, want %q (copies must not be swapped between scenes)", i+1, got, want)
		}
	}
	if left := listDir(t, e.deps.WorkDir); len(left) != 0 {
		t.Errorf("scratch files left behind: %v", left)
	}
}

func TestCopyMedia_ByDefaultScenesAreCopiedOneAtATime(t *testing.T) {
	for _, parallel := range []int{0, 1} {
		t.Run(fmt.Sprint("Parallel=", parallel), func(t *testing.T) {
			e := newEnv(t)
			e.deps.Parallel = parallel
			var pc peakCounter
			e.assets.onDownload = func(string) { pc.enter(); time.Sleep(5 * time.Millisecond); pc.leave() }
			job := e.jobWith(5)
			if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
				t.Fatal(err)
			}
			if got := pc.mostAtOnce(); got != 1 {
				t.Errorf("most downloads at once = %d, want 1", got)
			}
		})
	}
}

func TestCopyMedia_Parallel_ProgressCountsFinishedScenesAndEndsAtTheTotal(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 4
	job := e.jobWith(8)
	var mu sync.Mutex
	var seen [][2]int
	progress := Progress(func(done, total int) {
		mu.Lock()
		seen = append(seen, [2]int{done, total})
		mu.Unlock()
	})
	if err := CopyMedia(context.Background(), e.deps, job, progress); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 8 {
		t.Fatalf("progress reported %d times, want once per scene (8): %v", len(seen), seen)
	}
	for i, p := range seen {
		if p != [2]int{i + 1, 8} {
			t.Errorf("report %d = %v, want %v (a running count, whatever the order of finishing)", i, p, [2]int{i + 1, 8})
		}
	}
}

func TestCopyMedia_Parallel_AThrottleMakesTheRestRunOneAtATime(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 3
	e.deps.Retry = quickRetry()
	e.deps.ThrottleWaits = []time.Duration{time.Second}
	slept := &sleepLog{}
	e.deps.Sleep = slept.sleep

	// The first line the throttled scene logs opens the gate for the two scenes
	// held beside it. The flag is set before that line is written.
	logs := &logSink{}
	gate := make(chan struct{})
	var once sync.Once
	e.deps.Logf = func(f string, a ...any) {
		logs.logf(f, a...)
		if strings.Contains(fmt.Sprintf(f, a...), "scene=1 media=download_retry") {
			once.Do(func() { close(gate) })
		}
	}
	var pc peakCounter
	var lateConcurrent atomic.Int32
	two, three := make(chan struct{}), make(chan struct{})
	e.assets.onDownload = func(key string) {
		switch key {
		case "pixabay:photo:101":
			// Scenes 2 and 3 must be under way before scene 1 is throttled, so
			// that the first three are started together whatever the timing.
			wait(t, two, "scene 2 to start")
			wait(t, three, "scene 3 to start")
		case "pixabay:photo:102":
			close(two)
			wait(t, gate, "scene 1 to meet the throttle")
		case "pixabay:photo:103":
			close(three)
			wait(t, gate, "scene 1 to meet the throttle")
		case "pixabay:photo:104", "pixabay:photo:105", "pixabay:photo:106":
			// Scenes 1 to 3 have all finished by now, so this must be the only
			// one. Stay a moment, so that a scene started beside it would be seen.
			if n := pc.enter(); n != 1 {
				lateConcurrent.Add(1)
			}
			time.Sleep(20 * time.Millisecond)
			pc.leave()
		}
	}
	job := e.jobWith(6)
	// Scene 1 is throttled once, then the retry gets through.
	e.assets.dlSeq["pixabay:photo:101"] = []error{throttled()}

	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatalf("a throttle that clears must not fail the step: %v", err)
	}
	if lateConcurrent.Load() != 0 {
		t.Errorf("%d of the last three scenes ran beside another; after a throttle they must run one at a time", lateConcurrent.Load())
	}
	if !strings.Contains(logs.text(), "media=serial_after_throttle remaining=3") {
		t.Errorf("the switch to one at a time was not logged:\n%s", logs.text())
	}
	for i, s := range job.Scenes {
		if s.Media == nil {
			t.Errorf("scene %d was not copied", i+1)
		}
	}
	if n := countOf(e.assets.downloads, "pixabay:photo:101"); n != 2 {
		t.Errorf("scene 1 downloads = %d, want 2 (the throttled one and the retry)", n)
	}
}

func TestCopyMedia_Parallel_TheThrottleAllowanceIsSharedNotMultiplied(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 3
	e.deps.Retry = quickRetry()
	e.deps.ThrottleWaits = []time.Duration{30 * time.Second, 30 * time.Second}
	e.deps.ThrottleBudget = 45 * time.Second
	slept := &sleepLog{}
	e.deps.Sleep = slept.sleep
	job := e.jobWith(3)
	// Two scenes side by side are throttled for good. Each would like 30 s; the
	// budget is 45 s for the whole job.
	e.assets.dlErr["pixabay:photo:101"] = throttled()
	e.assets.dlErr["pixabay:photo:102"] = throttled()

	se := stepErr(t, CopyMedia(context.Background(), e.deps, job, nil))
	if se.Code != CodeMediaTemporary || !se.Retryable {
		t.Errorf("got %s retryable=%v, want a retryable MEDIA_TEMPORARY", se.Code, se.Retryable)
	}
	if se.Scene != 1 {
		t.Errorf("failed scene = %d, want 1 (the lowest of the two throttled scenes)", se.Scene)
	}
	var total time.Duration
	for _, w := range slept.waits {
		total += w
	}
	if total > 45*time.Second {
		t.Errorf("waited %v in all, over the 45s allowance for the job (waits %v)", total, slept.waits)
	}
	if len(slept.waits) != 1 {
		t.Errorf("waits = %v, want exactly one 30s wait to fit in 45s", slept.waits)
	}
}

func TestCopyMedia_Parallel_AFailureStopsNewScenesButKeepsTheOnesUnderWay(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 2
	var saves atomic.Int32
	e.deps.SaveProgress = func(context.Context) error { saves.Add(1); return nil }
	twoStarted, oneFailing := make(chan struct{}), make(chan struct{})
	e.assets.onDownload = func(key string) {
		switch key {
		case "pixabay:photo:101":
			wait(t, twoStarted, "scene 2 to start") // 2 must be under way before 1 fails
			close(oneFailing)
		case "pixabay:photo:102":
			close(twoStarted)
			<-oneFailing
			time.Sleep(30 * time.Millisecond) // scene 1's failure is recorded by now
		}
	}
	job := e.jobWith(4)
	// Scene 1 is gone from the provider: not worth a retry.
	e.assets.dlErr["pixabay:photo:101"] = fmt.Errorf("%w: gone", stockmedia.ErrAssetUnavailable)

	se := stepErr(t, CopyMedia(context.Background(), e.deps, job, nil))
	if se.Scene != 1 || se.Code != CodeMediaUnavailable || se.Retryable {
		t.Errorf("got scene %d %s retryable=%v, want scene 1 MEDIA_UNAVAILABLE not retryable", se.Scene, se.Code, se.Retryable)
	}
	if job.Scenes[1].Media == nil {
		t.Error("scene 2 was under way and finished, so it must be recorded for the next attempt")
	}
	if job.Scenes[2].Media != nil || job.Scenes[3].Media != nil {
		t.Error("scenes 3 and 4 must not have been started after a failure")
	}
	for _, k := range []string{"pixabay:photo:103", "pixabay:photo:104"} {
		if countOf(e.assets.downloads, k) != 0 {
			t.Errorf("%s was downloaded after the step had failed", k)
		}
	}
	if saves.Load() != 1 {
		t.Errorf("SaveProgress called %d times, want 1 (for scene 2)", saves.Load())
	}
}

func TestCopyMedia_Parallel_TellsTheUserAboutTheLowestFailingScene(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 3
	three := make(chan struct{})
	e.assets.onDownload = func(key string) {
		switch key {
		case "pixabay:photo:103":
			close(three)
		case "pixabay:photo:102":
			<-three // scene 3 has failed first
			time.Sleep(30 * time.Millisecond)
		}
	}
	job := e.jobWith(3)
	gone := func(string) error { return fmt.Errorf("%w: gone", stockmedia.ErrAssetUnavailable) }
	e.assets.dlErr["pixabay:photo:102"] = gone("2")
	e.assets.dlErr["pixabay:photo:103"] = gone("3")

	se := stepErr(t, CopyMedia(context.Background(), e.deps, job, nil))
	if se.Scene != 2 {
		t.Errorf("reported scene %d, want 2: the same scene a one-at-a-time run would stop on", se.Scene)
	}
}

func TestCopyMedia_Parallel_ACancelStopsNewScenes(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 2
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelled := make(chan struct{})
	e.assets.onDownload = func(key string) {
		switch key {
		case "pixabay:photo:101":
			cancel()
			close(cancelled)
		case "pixabay:photo:102":
			<-cancelled // finish only after the cancel, so no later scene can slip in first
		}
	}
	job := e.jobWith(4)
	err := CopyMedia(ctx, e.deps, job, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	for _, k := range []string{"pixabay:photo:103", "pixabay:photo:104"} {
		if countOf(e.assets.downloads, k) != 0 {
			t.Errorf("%s was started after the job was cancelled", k)
		}
	}
}

func TestCopyMedia_Parallel_AJobDeletedWhileSavingStopsTheStep(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 3
	e.deps.SaveProgress = func(context.Context) error { return ErrJobGone }
	job := e.jobWith(6)
	err := CopyMedia(context.Background(), e.deps, job, nil)
	if !errors.Is(err, ErrJobGone) {
		t.Fatalf("err = %v, want ErrJobGone", err)
	}
	// Only the scenes already under way (at most 3) may have been fetched.
	if n := len(e.assets.downloads); n > 3 {
		t.Errorf("%d downloads after the job was gone, want no more than the 3 under way", n)
	}
}

func TestCopyMedia_Parallel_ARestartSkipsWhatIsCopiedAndDoesTheRestSideBySide(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 3
	job := e.jobWith(6)
	// A first attempt copied scenes 1 and 2 and then died.
	first := e.jobWith(6)
	first.Scenes = first.Scenes[:2]
	if err := CopyMedia(context.Background(), e.deps, first, nil); err != nil {
		t.Fatal(err)
	}
	job.Scenes[0].Media, job.Scenes[1].Media = first.Scenes[0].Media, first.Scenes[1].Media
	before := len(e.assets.downloads)

	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	if got := len(e.assets.downloads) - before; got != 4 {
		t.Errorf("the restart downloaded %d scenes, want only the 4 not yet copied", got)
	}
}

// ---- NarrateScenes, side by side -------------------------------------------

func TestNarrateScenes_Parallel_RunsExactlyThatManyScenesAtOnce(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 3
	started := make(chan string, 16)
	release := make(chan struct{})
	var pc peakCounter
	e.narrator.onAudio = func(text string) {
		pc.enter()
		defer pc.leave()
		started <- text
		<-release
	}
	job := e.jobWith(6)

	result := make(chan error, 1)
	go func() { result <- NarrateScenes(context.Background(), e.deps, job, nil) }()
	for range 3 {
		wait(t, started, "three scenes to be narrated together")
	}
	select {
	case txt := <-started:
		t.Fatalf("a fourth scene (%q) started while three were held open", txt)
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	if err := wait(t, result, "NarrateScenes"); err != nil {
		t.Fatalf("NarrateScenes: %v", err)
	}

	if got := pc.mostAtOnce(); got != 3 {
		t.Errorf("most scenes at once = %d, want 3", got)
	}
	for i, s := range job.Scenes {
		if s.Voice == nil {
			t.Fatalf("scene %d has no voice", i+1)
		}
		if want := WorkKey("u1", "j1", i+1, "voice.mp3"); s.Voice.AudioKey != want {
			t.Errorf("scene %d audio key = %q, want %q", i+1, s.Voice.AudioKey, want)
		}
		// Each scene's own text must be in its own file, not another scene's.
		if got, want := string(e.files.files["bucket/"+s.Voice.AudioKey]), "MP3:"+s.Narration; got != want {
			t.Errorf("scene %d stored audio %q, want %q", i+1, got, want)
		}
	}
	if want := 6*3.35 + 0.4; job.TotalSeconds < want-1e-9 || job.TotalSeconds > want+1e-9 {
		t.Errorf("TotalSeconds = %v, want %v (six scenes of 3 s with the pauses)", job.TotalSeconds, want)
	}
}

func TestNarrateScenes_Parallel_GivesTheSameResultAsOneAtATime(t *testing.T) {
	run := func(parallel int) *Job {
		e := newEnv(t)
		e.deps.Parallel = parallel
		job := e.jobWith(7)
		if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
			t.Fatalf("Parallel=%d: %v", parallel, err)
		}
		return job
	}
	serial, side := run(1), run(4)
	if serial.TotalSeconds != side.TotalSeconds {
		t.Errorf("TotalSeconds %v (serial) vs %v (parallel)", serial.TotalSeconds, side.TotalSeconds)
	}
	for i := range serial.Scenes {
		a, b := serial.Scenes[i].Voice, side.Scenes[i].Voice
		if a == nil || b == nil || *a != *b {
			t.Errorf("scene %d voice differs: %+v vs %+v", i+1, a, b)
		}
	}
}

func TestNarrateScenes_Parallel_ProgressCountsFinishedScenes(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 4
	job := e.jobWith(6)
	var mu sync.Mutex
	var seen []int
	progress := Progress(func(done, total int) {
		mu.Lock()
		seen = append(seen, done)
		mu.Unlock()
		if total != 6 {
			t.Errorf("total = %d, want 6", total)
		}
	})
	if err := NarrateScenes(context.Background(), e.deps, job, progress); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(seen) != "[1 2 3 4 5 6]" {
		t.Errorf("progress = %v, want a running count to 6", seen)
	}
}

func TestNarrateScenes_Parallel_AFailureStopsNewScenesButKeepsTheOnesUnderWay(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 2
	job := e.jobWith(4)
	failing := job.Scenes[0].Narration
	e.narrator.audioErrFor = map[string]error{failing: errors.New("polly is down")}
	twoStarted, oneFailing := make(chan struct{}), make(chan struct{})
	e.narrator.onAudio = func(text string) {
		switch text {
		case job.Scenes[0].Narration:
			wait(t, twoStarted, "scene 2 to start") // 2 must be under way before 1 fails
			close(oneFailing)
		case job.Scenes[1].Narration:
			close(twoStarted)
			<-oneFailing
			time.Sleep(30 * time.Millisecond)
		}
	}

	se := stepErr(t, NarrateScenes(context.Background(), e.deps, job, nil))
	if se.Scene != 1 || se.Code != CodeNarrationFailed || !se.Retryable {
		t.Errorf("got scene %d %s retryable=%v, want scene 1 NARRATION_FAILED retryable", se.Scene, se.Code, se.Retryable)
	}
	if job.Scenes[1].Voice == nil {
		t.Error("scene 2 was under way and finished, so its voice must be kept (Polly was paid for it)")
	}
	if job.Scenes[2].Voice != nil || job.Scenes[3].Voice != nil {
		t.Error("scenes 3 and 4 must not have been started after a failure")
	}
	for _, c := range e.narrator.calls {
		if strings.Contains(c, job.Scenes[2].Narration) || strings.Contains(c, job.Scenes[3].Narration) {
			t.Errorf("Polly was called for a scene after the step had failed: %s", c)
		}
	}
}

func TestNarrateScenes_ByDefaultScenesAreNarratedOneAtATime(t *testing.T) {
	e := newEnv(t)
	var pc peakCounter
	e.narrator.onAudio = func(string) { pc.enter(); time.Sleep(5 * time.Millisecond); pc.leave() }
	if err := NarrateScenes(context.Background(), e.deps, e.jobWith(5), nil); err != nil {
		t.Fatal(err)
	}
	if got := pc.mostAtOnce(); got != 1 {
		t.Errorf("most scenes at once = %d, want 1", got)
	}
}

// ---- the whole runner, side by side ----------------------------------------
//
// These go through Runner.Run and a store that copies the whole job on every
// save, as DynamoStore does, so the job is read by the save while the scenes
// are being recorded. Run with -race, they are what proves that the saves and
// the recording of a scene never overlap.

func TestRun_Parallel_ATenSceneJobCompletesWithEverySceneKept(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 4
	st := newStore(e.jobWith(10))
	out, err := e.runner(st, &fakeRenderer{}).Run(context.Background(), "u1", "j1")
	if err != nil || out != OutcomeCompleted {
		t.Fatalf("outcome %v, err %v", out, err)
	}
	got := st.current()
	if len(got.Scenes) != 10 {
		t.Fatalf("%d scenes saved, want 10", len(got.Scenes))
	}
	for i, s := range got.Scenes {
		n := i + 1
		if s.Media == nil || s.Media.Key != WorkKey("u1", "j1", n, "media.jpg") {
			t.Errorf("scene %d media = %+v", n, s.Media)
		}
		if s.Voice == nil || s.Voice.AudioKey != WorkKey("u1", "j1", n, "voice.mp3") {
			t.Errorf("scene %d voice = %+v", n, s.Voice)
		}
	}
	if got.Status != StatusCompleted || got.Progress != 100 {
		t.Errorf("job ended %s at %d%%, want completed at 100%%", got.Status, got.Progress)
	}
	if n := len(e.assets.downloads); n != 10 {
		t.Errorf("%d downloads, want one per scene (10)", n)
	}
}

func TestRun_Parallel_ARestartNeverFetchesASceneTwice(t *testing.T) {
	e := newEnv(t)
	e.deps.Parallel = 3
	// Scene 4 cannot be looked up on the first attempt. Which of scenes 5 and 6
	// had been started by then depends on timing, but every scene that was
	// copied must have been recorded, so across both attempts each scene is
	// fetched exactly once.
	e.assets.getErr["pixabay:photo:104"] = fmt.Errorf("provider: %w", errTemporary())
	st := newStore(e.jobWith(6))
	if out, _ := e.runner(st, &fakeRenderer{}).Run(context.Background(), "u1", "j1"); out != OutcomeRetry {
		t.Fatalf("setup: the first attempt should ask to be retried, got %v", out)
	}
	delete(e.assets.getErr, "pixabay:photo:104") // the provider is back
	out, err := e.runner(st, &fakeRenderer{}).Run(context.Background(), "u1", "j1")
	if out != OutcomeCompleted || err != nil {
		t.Fatalf("second attempt: outcome %v err %v", out, err)
	}
	for id := 101; id <= 106; id++ {
		key := fmt.Sprintf("pixabay:photo:%d", id)
		if n := countOf(e.assets.downloads, key); n != 1 {
			t.Errorf("%s was downloaded %d times across the two attempts, want once", key, n)
		}
	}
}
