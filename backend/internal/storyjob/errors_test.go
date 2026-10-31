package storyjob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

func TestStepError_TextCodeAndCause(t *testing.T) {
	cause := errors.New("underlying")
	e := &StepError{Scene: 2, Code: CodeMediaTemporary, Message: "Scene 2: try again.", Err: cause}
	if !strings.Contains(e.Error(), "MEDIA_TEMPORARY") || !strings.Contains(e.Error(), "underlying") {
		t.Errorf("Error() = %q", e.Error())
	}
	if !errors.Is(e, cause) {
		t.Error("the cause cannot be reached with errors.Is")
	}
	plain := &StepError{Code: CodeInternal, Message: "m"}
	if plain.Error() != "INTERNAL: m" || errors.Unwrap(plain) != nil {
		t.Errorf("Error() = %q", plain.Error())
	}
}

func TestMediaError_NamesTheProviderInWords(t *testing.T) {
	for provider, want := range map[string]string{"pexels": "Pexels", "pixabay": "Pixabay", "other": "the photo library"} {
		se := mediaError(1, provider, stockmedia.ErrNotFound).(*StepError)
		if !strings.Contains(se.Message, want) {
			t.Errorf("%s: %q lacks %q", provider, se.Message, want)
		}
	}
}

func TestMediaError_ACancelIsNotAFailureOfTheStep(t *testing.T) {
	for _, err := range []error{context.Canceled, fmt.Errorf("wrapped: %w", context.Canceled)} {
		if got := mediaError(1, "pixabay", err); !errors.Is(got, context.Canceled) {
			t.Errorf("got %v", got)
		}
		if got := internalError(1, err); !errors.Is(got, context.Canceled) {
			t.Errorf("internalError: got %v", got)
		}
	}
}

func TestMediaError_AnUnknownFailureIsOursAndRetried(t *testing.T) {
	se := mediaError(3, "pixabay", errors.New("something odd")).(*StepError)
	if se.Code != CodeInternal || !se.Retryable || !strings.Contains(se.Message, "Scene 3") {
		t.Errorf("got %+v", se)
	}
	whole := internalError(0, errors.New("x")).(*StepError)
	if strings.Contains(whole.Message, "Scene") {
		t.Errorf("a whole-job failure names a scene: %q", whole.Message)
	}
}

func TestIsRetryable(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"the user must change something": {&StepError{Retryable: false}, false},
		"a retry may fix it":             {&StepError{Retryable: true}, true},
		"wrapped":                        {fmt.Errorf("x: %w", &StepError{Retryable: false}), false},
		"an unexpected error is ours":    {errors.New("boom"), true},
		"no error":                       {nil, false},
	}
	for name, c := range cases {
		if got := IsRetryable(c.err); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}

func TestCopyMedia_AFormatWeCannotHandleIsAnInvalidPictureNotACrash(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(1)
	a := e.assets.assets["pixabay:photo:101"]
	a.Format = "image/tiff"
	e.assets.assets["pixabay:photo:101"] = a
	se := stepErr(t, CopyMedia(context.Background(), e.deps, job, nil))
	if se.Code != CodeMediaInvalid || se.Retryable || se.Scene != 1 {
		t.Errorf("got %+v", se)
	}
}

func TestCopyMedia_AClipThatHasNoLengthCannotBeUsed(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(1)
	e.assets.add(stockmedia.ProviderPexels, stockmedia.MediaVideo, "900")
	job.Scenes[0].Pick.Provider, job.Scenes[0].Pick.Type, job.Scenes[0].Pick.ID = "pexels", "video", "900"
	e.prober.info = func(string, string) (MediaInfo, error) {
		return MediaInfo{Width: 1080, Height: 1920, HasVideo: true}, nil // no Duration
	}
	se := stepErr(t, CopyMedia(context.Background(), e.deps, job, nil))
	if se.Code != CodeMediaInvalid {
		t.Errorf("got %+v", se)
	}
}

func TestDeps_LogfDefaultsToTheStandardLogger(t *testing.T) {
	d := Deps{}
	d.logf("no panic with no logger set: %d", 1)
	var got string
	d.Logf = func(f string, a ...any) { got = fmt.Sprintf(f, a...) }
	d.logf("hello %s", "world")
	if got != "hello world" {
		t.Errorf("got %q", got)
	}
}
