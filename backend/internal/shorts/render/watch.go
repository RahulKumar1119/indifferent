package render

import (
	"context"
	"log"
	"time"
)

// jobStatusRendering is the job status while its clip tasks run. The state
// machine sets it before the clip tasks start.
const jobStatusRendering = "rendering"

// WatchJobStatus polls a shorts job's status every interval and calls cancel
// once the status is no longer "rendering". A user cancel writes "failed" to the
// job record, and cancel stops the render and the ffmpeg it runs. read returns
// the current status. A read error is logged, and the poll continues.
// WatchJobStatus returns when ctx ends.
func WatchJobStatus(ctx context.Context, every time.Duration, read func(context.Context) (string, error), cancel context.CancelFunc) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status, err := read(ctx)
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("job status check failed, will retry: %v", err)
				}
				continue
			}
			if status != jobStatusRendering {
				log.Printf("job status is %q, stopping the render", status)
				cancel()
				return
			}
		}
	}
}
