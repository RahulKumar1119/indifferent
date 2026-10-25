package render

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// statusStep is one answer from a fake job store: a status, or a read error.
type statusStep struct {
	status string
	err    error
}

// fakeJobStore answers reads with its steps in order, then repeats the last one.
type fakeJobStore struct {
	mu    sync.Mutex
	steps []statusStep
	reads int
}

func (f *fakeJobStore) read(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := min(f.reads, len(f.steps)-1)
	f.reads++
	return f.steps[i].status, f.steps[i].err
}

func (f *fakeJobStore) readCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads
}

func TestWatchJobStatusCancelsWhenStatusLeavesRendering(t *testing.T) {
	store := &fakeJobStore{steps: []statusStep{{status: "rendering"}, {status: "rendering"}, {status: "failed"}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go WatchJobStatus(ctx, time.Millisecond, store.read, cancel)

	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the watch did not cancel after the job left rendering")
	}
	if got := store.readCount(); got != 3 {
		t.Errorf("the watch must stop at the first status change: %d reads, want 3", got)
	}
}

func TestWatchJobStatusKeepsPollingThroughReadErrors(t *testing.T) {
	store := &fakeJobStore{steps: []statusStep{
		{err: errors.New("throttled")},
		{err: errors.New("throttled")},
		{status: "rendering"},
		{status: "completed"},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go WatchJobStatus(ctx, time.Millisecond, store.read, cancel)

	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the watch stopped polling after read errors")
	}
	if got := store.readCount(); got != 4 {
		t.Errorf("read errors must not cancel the render: %d reads, want 4", got)
	}
}

func TestWatchJobStatusStopsWhenItsContextEnds(t *testing.T) {
	store := &fakeJobStore{steps: []statusStep{{status: "failed"}}}
	ctx, stop := context.WithCancel(context.Background())
	stop()
	cancelled := 0
	WatchJobStatus(ctx, time.Millisecond, store.read, func() { cancelled++ })
	if cancelled != 0 {
		t.Errorf("a finished render must not be cancelled again, got %d calls", cancelled)
	}
}

// TestWatchJobStatusStopsTheRunningEncode runs a fake long-running encode through
// the same runner the renders use. The fake store reports a user cancel once the
// encode has started. After the cancel, the encode's process must be gone.
func TestWatchJobStatusStopsTheRunningEncode(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("needs /bin/sh for the fake encode")
	}
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	fakeFFmpeg := filepath.Join(dir, "ffmpeg")
	script := "#!/bin/sh\necho $$ > " + pidFile + "\nexec sleep 60\n"
	if err := os.WriteFile(fakeFFmpeg, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	// The user cancels after the encode has written its PID.
	read := func(context.Context) (string, error) {
		if _, err := os.Stat(pidFile); err == nil {
			return "failed", nil
		}
		return "rendering", nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go WatchJobStatus(ctx, 10*time.Millisecond, read, cancel)

	done := make(chan error, 1)
	go func() {
		done <- defaultRunCommandProgress(ctx, fakeFFmpeg, []string{"-i", "source.mp4"}, 60, func(int) {})
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("the fake encode ended on its own")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the encode still runs after the job was cancelled")
	}

	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("bad pid file %q: %v", raw, err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Errorf("process %d still exists after the cancel: %v", pid, err)
	}
}
