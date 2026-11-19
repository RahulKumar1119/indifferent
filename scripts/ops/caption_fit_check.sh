#!/bin/bash
# Renders captions with the real thing and checks them (#327). Read-only on the repository.
#
# The caption fitting in backend/internal/shorts/render measures text before it is drawn. This script
# proves it on pictures: it writes caption files for every preset, font and size, burns them onto a
# 1080x1920 frame with ffmpeg and libass inside alpine:3.21 with the packages the render images
# install (ffmpeg fontconfig ttf-dejavu font-noto-devanagari), and checks the pixels. See
# caption_fit_verify.py for what is checked.
#
# Needs: docker, go, python3 with Pillow (pip install pillow).
#
#   scripts/ops/caption_fit_check.sh                 # all checks, about 3 minutes
#   scripts/ops/caption_fit_check.sh --fonts DIR     # copy the fonts out of alpine:3.21 into DIR, for
#                                                    #   python3 scripts/ops/gen_caption_metrics.py DIR
#   KEEP=1 scripts/ops/caption_fit_check.sh          # keep the work folder (it is printed)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE="alpine:3.21"
PKGS="ffmpeg fontconfig ttf-dejavu font-noto-devanagari"

if [ "${1:-}" = "--fonts" ]; then
  dest="${2:?folder}"
  mkdir -p "$dest"
  cid=$(docker create "$IMAGE" sh -c "apk add --no-cache $PKGS >/dev/null 2>&1")
  docker start -a "$cid" >/dev/null
  for f in dejavu/DejaVuSans.ttf dejavu/DejaVuSerif.ttf dejavu/DejaVuSansCondensed.ttf noto/NotoSansDevanagari-Regular.ttf; do
    docker cp "$cid:/usr/share/fonts/$f" "$dest/"
  done
  docker rm "$cid" >/dev/null
  echo "fonts are in $dest"
  exit 0
fi

WORK="$(mktemp -d)"
[ -n "${KEEP:-}" ] || trap 'chmod -R u+rwX "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT
echo "work folder: $WORK"

echo "== writing the caption files"
(cd "$ROOT/backend" && CAPTION_FIT_DIR="$WORK" go test -count=1 -tags captionfit -run TestWriteCaptionFitCases ./internal/shorts/render >/dev/null)
python3 "$ROOT/scripts/ops/caption_fit_verify.py" jobs "$WORK"

cat > "$WORK/render.sh" <<'EOS'
#!/bin/sh
set -e
apk add --no-cache ffmpeg fontconfig ttf-dejavu font-noto-devanagari >/dev/null 2>&1
cd /w
mkdir -p out
while IFS='|' read -r name filter secs; do
  [ -n "$name" ] || continue
  ffmpeg -nostdin -loglevel error -y -f lavfi -i "color=c=0x2a2a2a:s=1080x1920:d=4" -vf "$filter" -ss "$secs" -frames:v 1 "out/$name.png"
done < jobs.txt
chmod -R a+rwX /w
EOS
echo "== rendering in $IMAGE with libass"
docker run --rm -v "$WORK:/w" "$IMAGE" sh /w/render.sh
echo "== checking the pictures"
python3 "$ROOT/scripts/ops/caption_fit_verify.py" check "$WORK"
