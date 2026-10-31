package storyjob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// fakeAssets stands in for the stock layer.
type fakeAssets struct {
	mu        sync.Mutex
	assets    map[string]stockmedia.Asset // by "provider:type:id"
	getErr    map[string]error
	dlErr     map[string]error
	dlBytes   map[string]string
	gets      []string
	downloads []string
}

func newAssets() *fakeAssets {
	return &fakeAssets{assets: map[string]stockmedia.Asset{}, getErr: map[string]error{}, dlErr: map[string]error{}, dlBytes: map[string]string{}}
}

func ref(provider, typ, id string) string { return provider + ":" + typ + ":" + id }

func (f *fakeAssets) add(provider stockmedia.ProviderName, typ stockmedia.MediaType, id string) stockmedia.Asset {
	format := "image/jpeg"
	if typ == stockmedia.MediaVideo {
		format = "video/mp4"
	}
	a := stockmedia.Asset{Provider: provider, Type: typ, ID: id, Format: format, Width: 1280, Height: 1920,
		Credit: fmt.Sprintf("%s %s by Jo on %s", typ, id, provider), PageURL: "https://example/" + id,
		MediaURL: "https://example.invalid/should-never-be-stored/" + id}
	f.assets[ref(string(provider), string(typ), id)] = a
	return a
}

func (f *fakeAssets) Get(_ context.Context, p stockmedia.ProviderName, t stockmedia.MediaType, id string) (stockmedia.Asset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := ref(string(p), string(t), id)
	f.gets = append(f.gets, k)
	if err := f.getErr[k]; err != nil {
		return stockmedia.Asset{}, err
	}
	a, ok := f.assets[k]
	if !ok {
		return stockmedia.Asset{}, stockmedia.ErrNotFound
	}
	return a, nil
}

func (f *fakeAssets) Download(_ context.Context, a stockmedia.Asset, w io.Writer) (int64, error) {
	f.mu.Lock()
	k := ref(string(a.Provider), string(a.Type), a.ID)
	f.downloads = append(f.downloads, k)
	err := f.dlErr[k]
	content, ok := f.dlBytes[k]
	f.mu.Unlock()
	if err != nil {
		return 0, err
	}
	if !ok {
		content = "bytes-of-" + k
	}
	n, werr := io.WriteString(w, content)
	return int64(n), werr
}

// fakeFiles is an in-memory bucket.
type fakeFiles struct {
	mu     sync.Mutex
	files  map[string][]byte
	puts   []string
	putErr map[string]error
}

func newFiles() *fakeFiles { return &fakeFiles{files: map[string][]byte{}, putErr: map[string]error{}} }

func (f *fakeFiles) PutObject(_ context.Context, bucket, key string, data []byte, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.putErr[key]; err != nil {
		return err
	}
	f.files[bucket+"/"+key] = append([]byte(nil), data...)
	f.puts = append(f.puts, key)
	return nil
}

func (f *fakeFiles) PutFile(_ context.Context, bucket, key, path, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.putErr[key]; err != nil {
		return err
	}
	data, err := readFile(path)
	if err != nil {
		return err
	}
	f.files[bucket+"/"+key] = data
	f.puts = append(f.puts, key)
	return nil
}

func (f *fakeFiles) HeadObject(_ context.Context, bucket, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.files[bucket+"/"+key]
	return ok, nil
}

func (f *fakeFiles) has(bucket, key string) bool { _, ok := f.files[bucket+"/"+key]; return ok }

// fakeNarrator stands in for Polly.
type fakeNarrator struct {
	mu       sync.Mutex
	audioErr error
	marksErr error
	marks    func(text string) string
	audio    []byte
	calls    []string
}

func (f *fakeNarrator) SynthesizeText(_ context.Context, text, voice string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "audio:"+voice+":"+text)
	if f.audioErr != nil {
		return nil, f.audioErr
	}
	return append([]byte("MP3:"), []byte(text)...), nil
}

func (f *fakeNarrator) SynthesizeMarks(_ context.Context, text, voice string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "marks:"+voice+":"+text)
	if f.marksErr != nil {
		return nil, f.marksErr
	}
	if f.marks != nil {
		return []byte(f.marks(text)), nil
	}
	return []byte(marksFor(text)), nil
}

// marksFor makes plausible Polly marks for text: one word every 0.3 s.
func marksFor(text string) string {
	var lines []string
	pos := 0
	for i, w := range strings.Fields(text) {
		at := strings.Index(text[pos:], w) + pos
		lines = append(lines, fmt.Sprintf(`{"time":%d,"type":"word","start":%d,"end":%d,"value":%q}`, i*300, at, at+len(w), w))
		pos = at + len(w)
	}
	return strings.Join(lines, "\n")
}

// fakeProber reports sizes and lengths by what the file contains, so a test
// can make a file "unreadable" or "long" by its bytes.
type fakeProber struct {
	mu    sync.Mutex
	info  func(path string, content string) (MediaInfo, error)
	paths []string
}

func (f *fakeProber) Probe(_ context.Context, path string) (MediaInfo, error) {
	f.mu.Lock()
	f.paths = append(f.paths, path)
	f.mu.Unlock()
	data, err := readFile(path)
	if err != nil {
		return MediaInfo{}, err
	}
	if f.info != nil {
		return f.info(path, string(data))
	}
	if strings.HasPrefix(string(data), "MP3:") {
		// 3 seconds of speech
		return MediaInfo{Duration: 3}, nil
	}
	if strings.Contains(string(data), ":video:") {
		return MediaInfo{Width: 1080, Height: 1920, Duration: 12, HasVideo: true}, nil
	}
	return MediaInfo{Width: 1280, Height: 1920, HasVideo: true}, nil
}

type env struct {
	assets   *fakeAssets
	files    *fakeFiles
	narrator *fakeNarrator
	prober   *fakeProber
	deps     Deps
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{assets: newAssets(), files: newFiles(), narrator: &fakeNarrator{}, prober: &fakeProber{}}
	e.deps = Deps{Assets: e.assets, Files: e.files, Narrator: e.narrator, Prober: e.prober,
		Bucket: "bucket", WorkDir: t.TempDir(), Logf: func(string, ...any) {}}
	return e
}

// jobWith makes a queued job of n scenes, each with a Pixabay photo picked.
func (e *env) jobWith(n int) *Job {
	j := &Job{UserID: "u1", JobID: "j1", Voice: "Joanna", Language: "en", Status: StatusQueued}
	for i := 1; i <= n; i++ {
		id := fmt.Sprint(100 + i)
		e.assets.add(stockmedia.ProviderPixabay, stockmedia.MediaPhoto, id)
		j.Scenes = append(j.Scenes, Scene{
			Narration: fmt.Sprintf("Narration of scene %d, said clearly.", i),
			Pick:      storyplan.MediaRef{Provider: "pixabay", Type: "photo", ID: id},
		})
	}
	return j
}

func stepErr(t *testing.T, err error) *StepError {
	t.Helper()
	var se *StepError
	if !errors.As(err, &se) {
		t.Fatalf("err = %v (%T), want a *StepError", err, err)
	}
	return se
}
