package storyjob

import (
	"context"
	"io"
	"log"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

// Assets is the stock-media provider layer. *stockmedia.Service satisfies it.
type Assets interface {
	Get(ctx context.Context, provider stockmedia.ProviderName, typ stockmedia.MediaType, id string) (stockmedia.Asset, error)
	Download(ctx context.Context, a stockmedia.Asset, w io.Writer) (int64, error)
}

// Files is the object storage. *storage.S3Client satisfies it.
type Files interface {
	PutFile(ctx context.Context, bucket, key, path, contentType string) error
	PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error
	HeadObject(ctx context.Context, bucket, key string) (bool, error)
}

// Narrator makes the voice. *narrator.PollyNarrationService satisfies it.
type Narrator interface {
	SynthesizeText(ctx context.Context, text string, voiceID string) ([]byte, error)
	SynthesizeMarks(ctx context.Context, text string, voiceID string) ([]byte, error)
}

// MediaInfo is what the prober learns about a file.
type MediaInfo struct {
	Width, Height int
	// Duration is in seconds; 0 for a still picture.
	Duration float64
	// HasVideo is true when the file has a picture stream. A still image counts.
	HasVideo bool
}

// Prober reads the real properties of a file. FFprobe satisfies it. The
// properties are measured, not taken from the provider: a mislabelled file
// must not reach the renderer.
type Prober interface {
	Probe(ctx context.Context, path string) (MediaInfo, error)
}

// Deps are what the steps need from the outside world.
type Deps struct {
	Assets   Assets
	Files    Files
	Narrator Narrator
	Prober   Prober
	// Bucket is the assets bucket working files and the finished video go to.
	Bucket string
	// WorkDir is a scratch directory with room for the largest stock clip.
	WorkDir string
	// Logf receives progress notes. It defaults to log.Printf.
	Logf func(format string, args ...any)
}

func (d Deps) logf(format string, args ...any) {
	if d.Logf != nil {
		d.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// Progress is told how many scenes of a step are done. It may be nil.
type Progress func(done, total int)

func (p Progress) report(done, total int) {
	if p != nil {
		p(done, total)
	}
}
