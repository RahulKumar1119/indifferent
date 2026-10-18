#!/usr/bin/env bash
# Regenerates the template preview images the project form shows.
#
# Each image is a question slide drawn by the same renderer that makes real
# videos (backend/cmd/templatepreview), scaled down to a small WebP. Run it
# whenever a theme palette changes, then commit the files it writes:
#
#   frontend/public/templates/<name>.webp
#
# Needs Go and an ffmpeg built with libwebp.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out="$root/frontend/public/templates"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

command -v ffmpeg >/dev/null || { echo "ffmpeg not found" >&2; exit 1; }

mkdir -p "$out"
(cd "$root/backend" && go run ./cmd/templatepreview -out "$tmp" >/dev/null)

for png in "$tmp"/*-question.png; do
  name="$(basename "$png" -question.png)"
  ffmpeg -v error -y -i "$png" -vf scale=480:270:flags=lanczos \
    -c:v libwebp -quality 82 "$out/$name.webp"
  echo "wrote frontend/public/templates/$name.webp ($(wc -c <"$out/$name.webp") bytes)"
done
