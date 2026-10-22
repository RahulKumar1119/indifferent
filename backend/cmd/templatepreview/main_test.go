package main

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/rahul/indifferent/backend/internal/slidegen"
	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

func TestCommittedPreviewsMatchRenderer(t *testing.T) {
	r := slidegen.NewNativeRenderer()
	for _, name := range slidegen.ThemeNames() {
		t.Run(name, func(t *testing.T) {
			rendered, err := r.RenderTemplate(name+"/question.html", sample())
			if err != nil {
				t.Fatal(err)
			}
			src, err := png.Decode(bytes.NewReader(rendered))
			if err != nil {
				t.Fatal(err)
			}
			want := image.NewRGBA(image.Rect(0, 0, 480, 270))
			draw.CatmullRom.Scale(want, want.Bounds(), src, src.Bounds(), draw.Src, nil)

			path := filepath.Join("..", "..", "..", "frontend", "public", "templates", name+".webp")
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			got, err := webp.Decode(f)
			if err != nil {
				t.Fatal(err)
			}
			if got.Bounds() != want.Bounds() {
				t.Fatalf("preview size = %v, want %v", got.Bounds(), want.Bounds())
			}

			var sum, over, maxDiff float64
			var n float64
			for y := 0; y < 270; y++ {
				for x := 0; x < 480; x++ {
					r1, g1, b1, _ := want.At(x, y).RGBA()
					r2, g2, b2, _ := got.At(x, y).RGBA()
					d := (math.Abs(float64(r1)-float64(r2)) + math.Abs(float64(g1)-float64(g2)) + math.Abs(float64(b1)-float64(b2))) / 3 / 257
					sum += d
					n++
					if d > 32 {
						over++
					}
					if d > maxDiff {
						maxDiff = d
					}
				}
			}
			mean := sum / n
			t.Logf("mean=%.2f over32=%.2f%% max=%.0f", mean, over/n*100, maxDiff)
			// Lossless WebP plus the same Lanczos-style downscale leaves a
			// sub-pixel difference; changing the type scale moves this well
			// past 8. The threshold catches stale previews without failing on
			// encoder rounding.
			if mean > 2 || over/n > 0.01 {
				t.Errorf("committed %s.webp is stale (mean %.2f, %.2f%% pixels differ >32); run scripts/generate-template-previews.sh", name, mean, over/n*100)
			}
		})
	}
}
