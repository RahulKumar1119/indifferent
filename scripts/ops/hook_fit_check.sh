#!/bin/bash
# Renders hook lines (#329) with the production render image and checks them. Read-only on the
# repository and on AWS.
#
# The hook layout in backend/internal/shorts/render/hook.go measures text before it is drawn and
# places it clear of the captions. This script proves it on pictures: it writes hook and caption
# files for English and Hindi, every hook style and position, and a range of caption presets and
# animated captions, builds the Shorts render image from backend/cmd/shortsrender/Dockerfile (the
# image production runs), burns them onto a 1080x1920 frame with that image's ffmpeg and libass, in
# the order the renderer uses, and checks the pixels. See hook_fit_verify.py for what is checked.
#
# Needs: docker, go, python3 with Pillow.
#
#   scripts/ops/hook_fit_check.sh                  # all checks
#   SHEET=/path/sheet.png scripts/ops/hook_fit_check.sh   # also write a contact sheet
#   KEEP=1 scripts/ops/hook_fit_check.sh           # keep the work folder (it is printed)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE="${IMAGE:-indifferent-shorts-render-hookcheck:local}"
WORK="$(mktemp -d)"
[ -n "${KEEP:-}" ] || trap 'chmod -R u+rwX "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT
echo "work folder: $WORK"
echo "== building the render image"
docker build --platform linux/amd64 -q -f "$ROOT/backend/cmd/shortsrender/Dockerfile" -t "$IMAGE" "$ROOT/backend" >/dev/null
echo "== writing the hook and caption files"
(cd "$ROOT/backend" && HOOK_FIT_DIR="$WORK" go test -count=1 -tags hookfit -run TestWriteHookFitCases ./internal/shorts/render >/dev/null)
python3 "$ROOT/scripts/ops/hook_fit_verify.py" jobs "$WORK"
cat > "$WORK/render.sh" <<'EOS'
#!/bin/sh
set -e
cd /w
mkdir -p out
fc-list | grep -c -i -e dejavu -e devanagari > out/fonts.txt
while IFS='|' read -r name filter secs; do
  [ -n "$name" ] || continue
  ffmpeg -nostdin -loglevel error -y -f lavfi -i "color=c=0x2a2a2a:s=1080x1920:r=30:d=6" -vf "$filter" -ss "$secs" -frames:v 1 "out/$name.png"
done < jobs.txt
chmod -R a+rwX /w
EOS
echo "== rendering in the production image"
docker run --rm --platform linux/amd64 --entrypoint sh -v "$WORK:/w" "$IMAGE" /w/render.sh
echo "fonts found in the image: $(cat "$WORK/out/fonts.txt")"
echo "== checking the pictures"
python3 "$ROOT/scripts/ops/hook_fit_verify.py" check "$WORK" ${SHEET:-}
