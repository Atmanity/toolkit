#!/usr/bin/env bash
# Measure a portrait before judging it by eye (see references/portrait-checks.md).
set -euo pipefail

die() { printf 'portrait-info.sh: %s\n' "$1" >&2; exit 1; }

[ $# -eq 1 ] || die "usage: portrait-info.sh <image>"
file="$1"
case "$(basename "$file")" in
  .*) die "refusing to read a hidden file: $file" ;;
esac
[ ! -L "$file" ] || die "refusing to read a symlink; use the file itself: $file"
[ -f "$file" ] || die "no such file: $file"

width="" height="" format=""
if command -v sips >/dev/null 2>&1; then
  info="$(sips -g pixelWidth -g pixelHeight -g format "$file" 2>/dev/null || true)"
  width="$(printf '%s\n' "$info" | sed -n 's/.*pixelWidth: *//p')"
  height="$(printf '%s\n' "$info" | sed -n 's/.*pixelHeight: *//p')"
  format="$(printf '%s\n' "$info" | sed -n 's/.*format: *//p')"
elif command -v ffprobe >/dev/null 2>&1; then
  info="$(ffprobe -v error -select_streams v:0 -show_entries stream=width,height,codec_name -of default=noprint_wrappers=1 "$file" 2>/dev/null || true)"
  width="$(printf '%s\n' "$info" | sed -n 's/^width=//p')"
  height="$(printf '%s\n' "$info" | sed -n 's/^height=//p')"
  format="$(printf '%s\n' "$info" | sed -n 's/^codec_name=//p')"
else
  echo "portrait-info.sh: needs sips or ffprobe. Measure with Python instead:" >&2
  echo "  python3 -c \"import sys,os;from PIL import Image;i=Image.open(sys.argv[1]);w,h=i.size;b=os.path.getsize(sys.argv[1]);print(f'width={w}\\nheight={h}\\naspect={max(w,h)/min(w,h):.2f}\\nformat={i.format.lower()}\\nbytes={b}\\nbytes_per_pixel={b/(w*h):.3f}')\" \"$file\"" >&2
  exit 2
fi

case "$width$height" in
  '' | *[!0-9]*) die "not an image this tool can read: $file" ;;
esac
case "$format" in
  mjpeg | jpeg) format=jpeg ;;
esac

bytes="$(wc -c < "$file" | tr -d ' ')"
awk -v w="$width" -v h="$height" -v f="$format" -v b="$bytes" 'BEGIN {
  long = (w > h) ? w : h; short = (w > h) ? h : w
  printf "width=%d\nheight=%d\naspect=%.2f\nformat=%s\nbytes=%d\nbytes_per_pixel=%.3f\n", w, h, long / short, f, b, b / (w * h)
}'
