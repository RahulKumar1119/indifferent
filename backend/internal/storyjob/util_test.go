package storyjob

import (
	"context"
	"io"
	"os"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

func readFile(path string) ([]byte, error) { return os.ReadFile(path) }

func listDir(t interface {
	Helper()
	Fatal(...any)
}, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

// stalled is an Assets whose downloads wait until the context ends.
type stalled struct{ *fakeAssets }

func (s stalled) Download(ctx context.Context, _ stockmedia.Asset, _ io.Writer) (int64, error) {
	<-ctx.Done()
	return 0, ctx.Err()
}

func writeFile(path string, data []byte) error { return os.WriteFile(path, data, 0o600) }
